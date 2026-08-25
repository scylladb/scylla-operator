//go:build envtest

package controllers

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var _ = g.Describe("ScyllaDBDatacenter controller scale down", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should decommission the last member of a rack on scale down", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Running a fake StatefulSet rollout syncer")
		runFakeStatefulSetRolloutSyncer(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a two-node rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a"}, withEnableParallelNodeOperations(false), withRackTemplateNodes(2))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet and the last member service to be created")
		statefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, statefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		lastMemberServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1)
		waitForService(ctx, env, lastMemberServiceName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

		g.By("Scaling the rack down to one node")
		sdc = updateRackTemplateNodes(ctx, env, sdc.Name, 1)

		g.By("Waiting for the last member service to be marked with the decommission intent")
		waitForServiceDecommissionIntent(ctx, env, lastMemberServiceName)

		g.By("Marking the member as decommissioned in place of the sidecar")
		markMemberServiceAsDecommissioned(ctx, env, lastMemberServiceName)

		g.By("Waiting for the StatefulSet to scale down and the member service to be pruned")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			statefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, statefulSetName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(*statefulSet.Spec.Replicas).To(o.Equal(int32(1)))

			_, err = env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, lastMemberServiceName, metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should complete an ongoing decommission and re-bootstrap the member when the rack is scaled back up mid-decommission", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Running a fake StatefulSet rollout syncer")
		runFakeStatefulSetRolloutSyncer(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a two-node rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a"}, withEnableParallelNodeOperations(false), withRackTemplateNodes(2))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet and the last member service to be created")
		statefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, statefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		lastMemberServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1)
		waitForService(ctx, env, lastMemberServiceName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

		g.By("Scaling the rack down to one node")
		sdc = updateRackTemplateNodes(ctx, env, sdc.Name, 1)

		g.By("Waiting for the last member service to be marked with the decommission intent")
		decommissioningService := waitForServiceDecommissionIntent(ctx, env, lastMemberServiceName)

		g.By("Scaling the rack back up to two nodes while the decommission is still in progress")
		sdc = updateRackTemplateNodes(ctx, env, sdc.Name, 2)

		g.By("Waiting for the controller to observe the scale up")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			freshSDC, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(freshSDC.Status.ObservedGeneration).NotTo(o.BeNil())
			eo.Expect(*freshSDC.Status.ObservedGeneration).To(o.BeNumerically(">=", sdc.Generation))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Marking the member as decommissioned in place of the sidecar")
		markMemberServiceAsDecommissioned(ctx, env, lastMemberServiceName)

		g.By("Waiting for the decommissioned member to be removed and re-bootstrapped afresh")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, lastMemberServiceName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(svc.UID).NotTo(o.Equal(decommissioningService.UID), "the decommissioned member's service should have been pruned and recreated for a fresh bootstrap")
			eo.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))

			statefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, statefulSetName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(*statefulSet.Spec.Replicas).To(o.Equal(int32(2)))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should complete a scale down that the user reverted mid-decommission and bootstrap the regained capacity afresh", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Running a fake StatefulSet rollout syncer")
		runFakeStatefulSetRolloutSyncer(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a three-node rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a"}, withEnableParallelNodeOperations(false), withRackTemplateNodes(3))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSet and all member services to be created")
		statefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, statefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		memberServiceNames := []string{
			naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1),
			naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 2),
		}
		decommissioningServices := map[string]*corev1.Service{}
		for _, name := range memberServiceNames {
			decommissioningServices[name] = waitForService(ctx, env, name, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		}

		g.By("Scaling the rack down to one node")
		sdc = updateRackTemplateNodes(ctx, env, sdc.Name, 1)

		g.By("Waiting for both leaving nodes to be recorded in the rack status")
		waitForRackDecommissioningNodes(ctx, env, sdc.Name, sdc.Spec.Racks[0].Name, memberServiceNames)

		g.By("Waiting for the last member service to be marked with the decommission intent")
		waitForServiceDecommissionIntent(ctx, env, memberServiceNames[1])

		g.By("Scaling the rack back up to three nodes while the decommission is still in progress")
		sdc = updateRackTemplateNodes(ctx, env, sdc.Name, 3)

		g.By("Marking the last member as decommissioned in place of the sidecar")
		markMemberServiceAsDecommissioned(ctx, env, memberServiceNames[1])

		g.By("Waiting for the second leaving node to be marked with the decommission intent despite the reverted node count")
		waitForServiceDecommissionIntent(ctx, env, memberServiceNames[0])

		g.By("Marking the second member as decommissioned in place of the sidecar")
		markMemberServiceAsDecommissioned(ctx, env, memberServiceNames[0])

		g.By("Waiting for both nodes to be removed and bootstrapped afresh")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, name := range memberServiceNames {
				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, name, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(svc.UID).NotTo(o.Equal(decommissioningServices[name].UID), "the decommissioned member's service should have been pruned and recreated for a fresh bootstrap")
				eo.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))
			}

			statefulSet, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, statefulSetName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(*statefulSet.Spec.Replicas).To(o.Equal(int32(3)))

			freshSDC, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(freshSDC.Status.Racks).To(o.HaveLen(1))
			eo.Expect(freshSDC.Status.Racks[0].DecommissioningNodes).To(o.BeEmpty())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

func withRackTemplateNodes(nodes int32) func(*scyllav1alpha1.ScyllaDBDatacenter) {
	return func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
		sdc.Spec.RackTemplate.Nodes = new(nodes)
	}
}

// updateRackTemplateNodes retries the update until it lands: the controller keeps writing the status of the object
// while it reconciles, so a conflict is expected rather than exceptional.
func updateRackTemplateNodes(ctx context.Context, e *envtest.Environment, name string, nodes int32) *scyllav1alpha1.ScyllaDBDatacenter {
	g.GinkgoHelper()

	var sdc *scyllav1alpha1.ScyllaDBDatacenter
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		freshSDC, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())

		freshSDC.Spec.RackTemplate.Nodes = new(nodes)
		sdc, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Update(ctx, freshSDC, metav1.UpdateOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return sdc
}

func waitForService(ctx context.Context, e *envtest.Environment, name string, timeout time.Duration) *corev1.Service {
	g.GinkgoHelper()

	var svc *corev1.Service
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		var err error
		svc, err = e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(timeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return svc
}

func waitForServiceDecommissionIntent(ctx context.Context, e *envtest.Environment, name string) *corev1.Service {
	g.GinkgoHelper()

	var svc *corev1.Service
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		var err error
		svc, err = e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, naming.LabelValueFalse))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return svc
}

func waitForRackDecommissioningNodes(ctx context.Context, e *envtest.Environment, name string, rackName string, decommissioningNodes []string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())

		rackStatus, _, ok := oslices.Find(sdc.Status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
			return rackStatus.Name == rackName
		})
		eo.Expect(ok).To(o.BeTrue())
		eo.Expect(rackStatus.DecommissioningNodes).To(o.Equal(decommissioningNodes))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

// markMemberServiceAsDecommissioned stands in for the sidecar, which doesn't run in envtest: it reports
// the completion of the member's decommission by flipping the decommission label to true.
func markMemberServiceAsDecommissioned(ctx context.Context, e *envtest.Environment, name string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		svc, err := e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())

		svc.Labels[naming.DecommissionedLabel] = naming.LabelValueTrue
		_, err = e.TypedKubeClient().CoreV1().Services(e.Namespace()).Update(ctx, svc, metav1.UpdateOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

// runFakeStatefulSetRolloutSyncer stands in for the StatefulSet controller and kubelet, which don't run in envtest:
// it keeps every StatefulSet's status in sync with its spec so that rollouts always converge.
func runFakeStatefulSetRolloutSyncer(ctx context.Context, e *envtest.Environment) {
	client := e.TypedKubeClient().AppsV1().StatefulSets(e.Namespace())

	go func() {
		defer g.GinkgoRecover()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			statefulSetList, err := client.List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}

			for i := range statefulSetList.Items {
				statefulSet := &statefulSetList.Items[i]
				if statefulSet.Spec.Replicas == nil || isFakeRolledOut(statefulSet) {
					continue
				}

				replicas := *statefulSet.Spec.Replicas
				statefulSet.Status.ObservedGeneration = statefulSet.Generation
				statefulSet.Status.Replicas = replicas
				statefulSet.Status.ReadyReplicas = replicas
				statefulSet.Status.AvailableReplicas = replicas
				statefulSet.Status.UpdatedReplicas = replicas
				statefulSet.Status.CurrentRevision = "envtest-revision"
				statefulSet.Status.UpdateRevision = statefulSet.Status.CurrentRevision
				// Conflicts and transient errors are retried on the next tick.
				if _, err := client.UpdateStatus(ctx, statefulSet, metav1.UpdateOptions{}); err != nil {
					klog.Warningf("can't update status of StatefulSet %q: %s", naming.ObjRef(statefulSet), err)
				}
			}
		}
	}()
}

func isFakeRolledOut(statefulSet *appsv1.StatefulSet) bool {
	replicas := *statefulSet.Spec.Replicas
	return statefulSet.Status.ObservedGeneration == statefulSet.Generation &&
		statefulSet.Status.Replicas == replicas &&
		statefulSet.Status.ReadyReplicas == replicas &&
		statefulSet.Status.AvailableReplicas == replicas &&
		statefulSet.Status.UpdatedReplicas == replicas
}
