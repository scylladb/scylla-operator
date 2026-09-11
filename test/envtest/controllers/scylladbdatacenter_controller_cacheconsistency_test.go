//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/util/retry"
)

const (
	// informerLag is how far behind the API server the informer of one kind is kept in these specs, on top of the
	// default lag of all informers: wide enough for a sync deciding from a stale cache to be caught in the act.
	informerLag = 2 * time.Second

	// laggingEventuallyTimeout pads the default timeout for the syncs that wait for the lagging caches to observe
	// the controller's writes, one lag per write of the lagging kind.
	laggingEventuallyTimeout = 60 * time.Second

	// laggingConsistentlyTimeout is long enough for a sync that doesn't wait for the lagging caches to have decided
	// from the stale ones, and for the caches to have caught up afterwards.
	laggingConsistentlyTimeout = 3 * informerLag
)

// withInformerLag lags the informer of the objects selected by lags further behind the API server than the default.
func withInformerLag(lag time.Duration, lags func(obj any) bool) informers.SharedInformerOption {
	return informers.WithTransform(informerLagTransform(lag, lags))
}

// The informer caches don't give read-your-writes: a sync that runs before the informer delivers the controller's
// own write decides from the state that predates it. Every spec runs the controller with lagging informers; these
// two lag the informer of one kind far enough to catch a sync deciding from a cache that has not observed the
// controller's own write, which is the hazard behind each of them.
var _ = g.Describe("ScyllaDBDatacenter controller with lagging informers", func() {
	const rackName = "rack-a"

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should not report a rack as rolled out from a StatefulSet cache that has not observed the applied rollout", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller with a lagging StatefulSet informer")
		runScyllaDBDatacenterController(ctx, env, withInformerLag(informerLag, isStatefulSet))

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet to be created and marking it as rolled out")
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		rackStatefulSet := waitForStatefulSet(ctx, env, rackStatefulSetName, laggingEventuallyTimeout)
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)
		rolledOutGeneration := rackStatefulSet.Generation

		g.By("Waiting for the rack to be reported as rolled out")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.ObservedGeneration).To(o.HaveValue(o.Equal(sdc.Generation)))

			rackStatus := findRackStatus(sdc, rackName)
			eo.Expect(rackStatus).NotTo(o.BeNil())
			eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()))
		}).WithContext(ctx).WithTimeout(laggingEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Changing the ScyllaDB arguments to roll the rack StatefulSet out")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(env.Namespace(), sdc.Name), err)
			}

			sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments = []string{"--logger-log-level=compaction=debug"}
			_, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("can't update ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
			}

			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet to be updated")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sts.Generation).To(o.BeNumerically(">", rolledOutGeneration))
		}).WithContext(ctx).WithTimeout(laggingEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		// The StatefulSet status is never updated in envtest, so the rack stays stale from the update on. A sync that
		// recomputed the rack from the StatefulSet cache before it observed the update would report it as not stale
		// for the new ScyllaDBDatacenter generation.
		g.By("Verifying the rack is never reported as rolled out for the new generation")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())

			if sdc.Status.ObservedGeneration == nil || *sdc.Status.ObservedGeneration != sdc.Generation {
				return
			}

			rackStatus := findRackStatus(sdc, rackName)
			co.Expect(rackStatus).NotTo(o.BeNil())
			co.Expect(rackStatus.Stale).To(o.HaveValue(o.BeTrue()), "rack %q was reported as rolled out for generation %d from a StatefulSet cache that has not observed the update", rackName, sdc.Generation)
		}).WithContext(ctx).WithTimeout(laggingConsistentlyTimeout).WithPolling(50 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should not scale a rack up over a node whose requested decommission has not been observed yet", func(ctx g.SpecContext) {
		const initialNodes = int32(3)

		g.By("Running ScyllaDBDatacenter controller with a lagging Service informer")
		runScyllaDBDatacenterController(ctx, env, withInformerLag(informerLag, isService))

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a three-node rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName}, withRackTemplateNodes(initialNodes))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet and member Services to be created and marking the StatefulSet as rolled out")
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		leavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(initialNodes-1))
		waitForStatefulSet(ctx, env, rackStatefulSetName, laggingEventuallyTimeout)
		waitForService(ctx, env, leavingServiceName, laggingEventuallyTimeout)
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

		g.By("Scaling the rack down to two nodes")
		scaleRackTemplate(ctx, env, sdc.Name, initialNodes-1)

		g.By("Waiting for the decommission of the highest node to be requested")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, leavingServiceName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, naming.LabelValueFalse))
		}).WithContext(ctx).WithTimeout(laggingEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		// The decommission request is the controller's own write. A sync deciding from a Service cache that has not
		// observed it yet sees no leaving node and, with the node count above the StatefulSet replicas, scales the
		// rack up while the node is leaving the cluster.
		g.By("Raising the node count above the current one while the node is still decommissioning")
		scaleRackTemplate(ctx, env, sdc.Name, initialNodes+1)

		g.By("Verifying the rack StatefulSet is not scaled up over the leaving node")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(*sts.Spec.Replicas).To(o.Equal(initialNodes))
		}).WithContext(ctx).WithTimeout(laggingConsistentlyTimeout).WithPolling(50 * time.Millisecond).Should(o.Succeed())

		g.By("Waiting for the deferred node count change to be reported as progressing")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())

			progressingCondition := apimeta.FindStatusCondition(sdc.Status.Conditions, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.ProgressingCondition))
			eo.Expect(progressingCondition).NotTo(o.BeNil())
			eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(progressingCondition.Reason).To(o.ContainSubstring("DeferringRackNodeCountChange"))
		}).WithContext(ctx).WithTimeout(laggingEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

func findRackStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName string) *scyllav1alpha1.RackStatus {
	rackStatus, _, found := oslices.Find(sdc.Status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
		return rackStatus.Name == rackName
	})
	if !found {
		return nil
	}

	return &rackStatus
}
