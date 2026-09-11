//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// scyllaDBDatacenterInformerLag is how far behind the API server the ScyllaDBDatacenter informer is kept in the
	// spec that lags it: wide enough for syncs triggered by the other kinds to run from a ScyllaDBDatacenter cache
	// that predates the spec's update.
	scyllaDBDatacenterInformerLag = 2 * time.Second

	// invariantPollingInterval samples the state often enough to catch a status that is only briefly wrong.
	invariantPollingInterval = 20 * time.Millisecond
)

// Informer caches give no read-your-writes, so a sync may decide from state that predates the controller's own
// writes. These specs hold invariants throughout a change, while the other specs only assert the end state, so that
// a decision made from a stale cache is visible.
var _ = g.Describe("ScyllaDBDatacenter controller status invariants", func() {
	const rackName = "rack-a"

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should never report a status observed generation that decreases or exceeds the generation", func(ctx g.SpecContext) {
		sdc := setupRolledOutRacks(ctx, env, false, []string{rackName}, 1)

		// Each update bumps the generation. They are issued back to back, so that syncs of consecutive generations
		// overlap with the lagging cache.
		g.By("Updating the ScyllaDB arguments several times in a row")
		for i := range 3 {
			updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
				sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments = []string{fmt.Sprintf("--logger-log-level=compaction=debug%d", i)}
			})
		}

		g.By("Verifying the observed generation never decreases nor exceeds the generation")
		consistentlyObservedGenerationIsMonotonic(ctx, env, sdc.Name, scyllaDBDatacenterControllerDefaultConsistentlyTimeout)

		g.By("Waiting for the status to catch up with the generation")
		waitForObservedGeneration(ctx, env, sdc.Name)
	})

	g.It("should never list a decommissioning node whose Service is not labelled as decommissioned", func(ctx g.SpecContext) {
		sdc, rackStatefulSetName, leavingServiceName := setupDecommissioningRack(ctx, env, false)

		// A listed node has to carry the decommissioned label, or be gone already: the list is derived from the
		// labels, so a sync from a Service cache that hasn't observed the pruning can list a pruned node for a
		// while, but never one that exists without the label.
		listedNodesAreLabelled := func(co o.Gomega, ctx context.Context) {
			for _, node := range getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName) {
				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, node.Name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					continue
				}
				co.Expect(err).NotTo(o.HaveOccurred())
				co.Expect(svc.Labels).To(o.HaveKey(naming.DecommissionedLabel), "node %q is listed as decommissioning while its Service isn't labelled", node.Name)
			}
		}

		g.By("Scaling the rack down to one node")
		scaleRackTemplate(ctx, env, sdc.Name, decommissioningInitialNodes-1)

		g.By("Verifying no unlabelled node is listed while the decommission is requested")
		o.Consistently(listedNodesAreLabelled).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(invariantPollingInterval).Should(o.Succeed())
		waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)

		g.By("Marking the node as decommissioned in place of the sidecar")
		setServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueTrue)

		g.By("Verifying no unlabelled node is listed while the node is scaled away and pruned")
		o.Consistently(listedNodesAreLabelled).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(invariantPollingInterval).Should(o.Succeed())

		g.By("Waiting for the rack StatefulSet to be scaled down and the leaving node to be pruned")
		waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, decommissioningInitialNodes-1)
		waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, leavingServiceName)
	})

	g.It("should never regress the published status to a previous generation with a lagging ScyllaDBDatacenter informer", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller with a lagging ScyllaDBDatacenter informer")
		runScyllaDBDatacenterControllerWithOptions(ctx, env, scyllaDBDatacenterControllerRunOptions{
			scyllaInformerOptions: []scyllainformers.SharedInformerOption{
				scyllainformers.WithTransform(informerLagTransform(scyllaDBDatacenterInformerLag, isScyllaDBDatacenter)),
			},
		})

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet and its status to be published")
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, rackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout+scyllaDBDatacenterInformerLag)
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)
		waitForObservedGeneration(ctx, env, sdc.Name)

		g.By("Updating the ScyllaDB arguments")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments = []string{"--logger-log-level=compaction=debug"}
		})

		// The StatefulSet informer observes the rollout marker long before the ScyllaDBDatacenter informer observes
		// the update, so the sync it triggers runs from the ScyllaDBDatacenter of the previous generation.
		g.By("Triggering a sync through the StatefulSet while the ScyllaDBDatacenter update is not observed yet")
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

		g.By("Verifying the observed generation never decreases nor exceeds the generation")
		consistentlyObservedGenerationIsMonotonic(ctx, env, sdc.Name, scyllaDBDatacenterControllerDefaultConsistentlyTimeout+scyllaDBDatacenterInformerLag)

		g.By("Waiting for the status to catch up with the generation")
		waitForObservedGeneration(ctx, env, sdc.Name)
	})
})

func isScyllaDBDatacenter(obj any) bool {
	_, ok := obj.(*scyllav1alpha1.ScyllaDBDatacenter)
	return ok
}

// consistentlyObservedGenerationIsMonotonic verifies for the given window that the observed generation of the
// ScyllaDBDatacenter status, and of every condition in it, never decreases between samples nor exceeds the generation.
func consistentlyObservedGenerationIsMonotonic(ctx context.Context, e *envtest.Environment, sdcName string, window time.Duration) {
	g.GinkgoHelper()

	var highestObservedGeneration int64
	o.Consistently(func(co o.Gomega, ctx context.Context) {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
		co.Expect(err).NotTo(o.HaveOccurred())
		co.Expect(sdc.Status.ObservedGeneration).NotTo(o.BeNil())
		co.Expect(*sdc.Status.ObservedGeneration).To(o.BeNumerically("<=", sdc.Generation), "status observed generation exceeds the generation")
		co.Expect(*sdc.Status.ObservedGeneration).To(o.BeNumerically(">=", highestObservedGeneration), "status observed generation regressed")
		highestObservedGeneration = *sdc.Status.ObservedGeneration

		for _, condition := range sdc.Status.Conditions {
			co.Expect(condition.ObservedGeneration).To(o.BeNumerically("<=", sdc.Generation), "condition %q observed generation exceeds the generation", condition.Type)
		}
	}).WithContext(ctx).WithTimeout(window).WithPolling(invariantPollingInterval).Should(o.Succeed())
}

// waitForObservedGeneration waits for the ScyllaDBDatacenter status to observe the current generation.
func waitForObservedGeneration(ctx context.Context, e *envtest.Environment, sdcName string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(sdc.Status.ObservedGeneration).To(o.HaveValue(o.Equal(sdc.Generation)))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout + scyllaDBDatacenterInformerLag).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}
