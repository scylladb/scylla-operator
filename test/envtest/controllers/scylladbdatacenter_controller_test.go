//go:build envtest

package controllers

import (
	"context"
	stdcrypto "crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"slices"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/util/retry"
)

const (
	// scyllaDBDatacenterControllerDisabledStatefulSetCachePropagationDelay disables the production cache-propagation
	// wait in envtests. Envtest runs the controller and API server in-process, so the default delay only slows tests down.
	// Tests that need to exercise cache lag should override this.
	scyllaDBDatacenterControllerDisabledStatefulSetCachePropagationDelay = 0 * time.Second

	scyllaDBDatacenterControllerResyncPeriod = 12 * time.Hour

	// scyllaDBDatacenterControllerDefaultEventuallyTimeout is the default timeout for async envtest assertions.
	// Pad accordingly when a test uses a non-zero cache-propagation delay, otherwise Eventually may time out before
	// the controller resumes reconciliation.
	scyllaDBDatacenterControllerDefaultEventuallyTimeout = 15 * time.Second

	// scyllaDBDatacenterControllerDefaultConsistentlyTimeout is the default window for stability assertions.
	// Pad accordingly when a test uses a non-zero cache-propagation delay, otherwise Consistently may pass while the
	// controller is delayed instead of observing real steady state.
	scyllaDBDatacenterControllerDefaultConsistentlyTimeout = 5 * time.Second

	// envtestServiceFinalizer holds a member Service in a terminating state, so that specs can freeze the window
	// between the controller deleting it and the object actually going away.
	envtestServiceFinalizer = "scylla-operator.scylladb.com/envtest"
)

var _ = g.Describe("ScyllaDBDatacenter controller", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should create rack StatefulSets sequentially with parallel node operations disabled", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with two racks")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a", "rack-b"}, withEnableParallelNodeOperations(false))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the first rack StatefulSet to be created")
		firstRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, firstRackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

		g.By("Verifying the second rack StatefulSet is not created before the first rack rolls out")
		secondRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, secondRackStatefulSetName, metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Marking the first rack StatefulSet as rolled out")
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), firstRackStatefulSetName)

		g.By("Waiting for the second rack StatefulSet to be created")
		waitForStatefulSet(ctx, env, secondRackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
	})

	g.It("should create rack StatefulSets in parallel with parallel node operations enabled", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with three racks and parallel node operations enabled")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a", "rack-b", "rack-c"}, withEnableParallelNodeOperations(true))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for all rack StatefulSets to be created without any of them rolling out")
		for _, rack := range sdc.Spec.Racks {
			statefulSet := waitForStatefulSet(ctx, env, naming.StatefulSetNameForRack(rack, sdc), scyllaDBDatacenterControllerDefaultEventuallyTimeout)
			o.Expect(statefulSet.Spec.PodManagementPolicy).To(o.Equal(appsv1.ParallelPodManagement))
		}
	})

	// The specs shared by both modes of parallel node operations. The ones whose flow differs between the modes follow
	// in their own blocks below.
	g.DescribeTableSubtree("decommissioning", func(enableParallelNodeOperations bool) {
		g.It("should list a node whose decommission is requested and drop it once it is pruned", func(ctx g.SpecContext) {
			sdc, rackStatefulSetName, leavingServiceName := setupDecommissioningRack(ctx, env, enableParallelNodeOperations)

			g.By("Scaling the rack down to one node")
			scaleRackTemplate(ctx, env, sdc.Name, decommissioningInitialNodes-1)

			g.By("Waiting for the decommission of the leaving node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)

			g.By("Waiting for the leaving node to be listed in the rack status")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: leavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled down")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, decommissioningInitialNodes-1)

			g.By("Waiting for the leaving node's Service to be pruned and the record to drain")
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, leavingServiceName)
		})

		g.It("should finish an ongoing decommission before applying a node count raised back mid-decommission", func(ctx g.SpecContext) {
			const (
				nodes  = int32(3)
				target = int32(1)
			)

			sdc := setupDecommissioningRacks(ctx, env, enableParallelNodeOperations, []string{decommissioningRackName}, nodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

			var leavingServiceNames, stayingServiceNames []string
			for ordinal := int32(0); ordinal < nodes; ordinal++ {
				svcName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(ordinal))
				if slices.Contains(leavingOrdinals(enableParallelNodeOperations, nodes, target), ordinal) {
					leavingServiceNames = append(leavingServiceNames, svcName)
				} else {
					stayingServiceNames = append(stayingServiceNames, svcName)
				}
			}

			g.By(fmt.Sprintf("Scaling the rack down to %d node(s)", target))
			scaleRackTemplate(ctx, env, sdc.Name, target)

			g.By(fmt.Sprintf("Waiting for the decommission of the leaving node(s) %q to be requested", leavingServiceNames))
			for _, svcName := range leavingServiceNames {
				waitForServiceDecommissionedLabel(ctx, env, svcName, naming.LabelValueFalse)
			}

			serviceUIDs := map[string]types.UID{}
			for _, svcName := range slices.Concat(leavingServiceNames, stayingServiceNames) {
				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				serviceUIDs[svcName] = svc.UID
			}

			// A leaving node is not ready from the moment its decommission starts, so the StatefulSet is not rolled out
			// for as long as the node is a part of it.
			g.By("Marking the leaving node(s) as not ready in place of the StatefulSet controller")
			markStatefulSetNodesAsNotReady(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName, int32(len(leavingServiceNames)))

			g.By("Raising the node count back while the node(s) are still decommissioning")
			scaleRackTemplate(ctx, env, sdc.Name, nodes)

			g.By("Waiting for the deferred node count change to be reported as progressing")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
				eo.Expect(progressingCondition).NotTo(o.BeNil())
				eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
				eo.Expect(progressingCondition.Reason).To(o.ContainSubstring("DeferringRackNodeCountChange"))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			expectedDecommissioningNodes := make([]scyllav1alpha1.DecommissioningNodeStatus, 0, len(leavingServiceNames))
			for _, svcName := range slices.Sorted(slices.Values(leavingServiceNames)) {
				expectedDecommissioningNodes = append(expectedDecommissioningNodes, scyllav1alpha1.DecommissioningNodeStatus{Name: svcName})
			}

			g.By("Verifying the raised node count is not applied while the node(s) are still decommissioning")
			o.Consistently(func(co o.Gomega, ctx context.Context) {
				sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
				co.Expect(err).NotTo(o.HaveOccurred())
				co.Expect(*sts.Spec.Replicas).To(o.Equal(nodes))

				for _, svcName := range leavingServiceNames {
					svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
					co.Expect(err).NotTo(o.HaveOccurred())
					co.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, naming.LabelValueFalse))
				}
				for _, svcName := range stayingServiceNames {
					svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
					co.Expect(err).NotTo(o.HaveOccurred())
					co.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))
				}

				co.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal(expectedDecommissioningNodes))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the leaving node(s) as decommissioned in place of the sidecar")
			for _, svcName := range leavingServiceNames {
				setServiceDecommissionedLabel(ctx, env, svcName, naming.LabelValueTrue)
			}

			// The scale-down, the pruning and the scale-up back can all happen within milliseconds, so instead of
			// sampling the intermediate states, verify the end state: a node can only have been pruned once the
			// StatefulSet was scaled below it, and a pruned node comes back as a fresh Service, while the staying
			// nodes keep theirs.
			g.By("Waiting for the leaving node(s) to be removed and the rack to grow back to the raised node count with fresh nodes")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(*sts.Spec.Replicas).To(o.Equal(nodes))

				for _, svcName := range leavingServiceNames {
					svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
					eo.Expect(err).NotTo(o.HaveOccurred())
					eo.Expect(svc.UID).NotTo(o.Equal(serviceUIDs[svcName]))
					eo.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))
				}
				for _, svcName := range stayingServiceNames {
					svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, svcName, metav1.GetOptions{})
					eo.Expect(err).NotTo(o.HaveOccurred())
					eo.Expect(svc.UID).To(o.Equal(serviceUIDs[svcName]))
					eo.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))
				}

				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.BeEmpty())
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
		})

		// The rack node count in the status reaches zero as soon as the StatefulSet is scaled down, while the member
		// Service of the last leaving node lives until it is pruned. The Service is held by a finalizer to freeze that
		// window, in which the rack removal used to be admitted, leaving its member Service and PVC unprunable.
		g.It("should reject removing a rack whose last leaving node is not pruned yet", func(ctx g.SpecContext) {
			sdc, rackStatefulSetName, leavingServiceName := setupDecommissioningRack(ctx, env, enableParallelNodeOperations)

			lastServiceName := fmt.Sprintf("%s-0", rackStatefulSetName)

			g.By("Adding a finalizer to the member Service of the last node")
			addServiceFinalizer(ctx, env, lastServiceName)

			g.By("Scaling the rack down to zero nodes")
			scaleRackTemplate(ctx, env, sdc.Name, 0)

			g.By("Marking both nodes as decommissioned in place of the sidecar")
			for _, serviceName := range []string{leavingServiceName, lastServiceName} {
				waitForServiceDecommissionedLabel(ctx, env, serviceName, naming.LabelValueFalse)
				setServiceDecommissionedLabel(ctx, env, serviceName, naming.LabelValueTrue)
			}

			g.By("Waiting for the rack StatefulSet to be scaled down to zero replicas")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, 0)

			g.By("Marking the rack StatefulSet as rolled out so that the rack status is not stale")
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

			g.By("Waiting for the rack status to report no nodes with the finalized node still listed as leaving")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				rackStatus := getRackStatus(ctx, env, sdc.Name, decommissioningRackName)
				eo.Expect(rackStatus).NotTo(o.BeNil())
				eo.Expect(rackStatus.Nodes).To(o.HaveValue(o.BeEquivalentTo(0)))
				eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()))
				eo.Expect(rackStatus.DecommissioningNodes).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: lastServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Verifying the rack can't be removed")
			err := removeRacks(ctx, env, sdc.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring(fmt.Sprintf("rack %q can't be removed because it still has nodes leaving the cluster: %s", decommissioningRackName, lastServiceName)))

			g.By("Removing the finalizer from the member Service of the last node")
			removeServiceFinalizer(ctx, env, lastServiceName)

			g.By("Waiting for the last node's Service to be pruned and the record to drain")
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, lastServiceName)

			g.By("Verifying the rack can be removed")
			err = removeRacks(ctx, env, sdc.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should rebuild the list from the decommissioned labels when it is wiped from the status", func(ctx g.SpecContext) {
			sdc, _, leavingServiceName := setupDecommissioningRack(ctx, env, enableParallelNodeOperations)

			g.By("Scaling the rack down to one node")
			scaleRackTemplate(ctx, env, sdc.Name, decommissioningInitialNodes-1)

			g.By("Waiting for the decommission of the leaving node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)

			g.By("Wiping the list from the rack status")
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(env.Namespace(), sdc.Name), err)
				}

				for i := range sdc.Status.Racks {
					sdc.Status.Racks[i].DecommissioningNodes = nil
				}
				_, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).UpdateStatus(ctx, sdc, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("can't update status of ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
				}

				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for the list to be rebuilt from the label")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: leavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
		})
	},
		g.Entry("with parallel node operations disabled", false),
		g.Entry("with parallel node operations enabled", true),
	)

	g.Describe("decommissioning with parallel node operations disabled", func() {
		g.It("should decommission a multi-node scale-down one node at a time from the highest ordinal", func(ctx g.SpecContext) {
			const nodes = int32(3)

			sdc := setupDecommissioningRacks(ctx, env, false, []string{decommissioningRackName}, nodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			firstLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 2)
			secondLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1)

			g.By("Scaling the rack down to one node")
			scaleRackTemplate(ctx, env, sdc.Name, 1)

			g.By("Waiting for the decommission of the highest node to be requested and listed")
			waitForServiceDecommissionedLabel(ctx, env, firstLeavingServiceName, naming.LabelValueFalse)
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: firstLeavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Verifying the decommission of the lower node is not requested until the highest node is removed, and the scale-down is not reported as a deferred node count change")
			o.Consistently(func(co o.Gomega, ctx context.Context) {
				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, secondLeavingServiceName, metav1.GetOptions{})
				co.Expect(err).NotTo(o.HaveOccurred())
				co.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))

				progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
				co.Expect(progressingCondition).NotTo(o.BeNil())
				co.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
				co.Expect(progressingCondition.Reason).NotTo(o.ContainSubstring("DeferringRackNodeCountChange"))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the highest node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, firstLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled down by one and the highest node to be pruned")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, nodes-1)
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				_, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, firstLeavingServiceName, metav1.GetOptions{})
				eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Waiting for the decommission of the remaining leaving node to be requested and listed")
			waitForServiceDecommissionedLabel(ctx, env, secondLeavingServiceName, naming.LabelValueFalse)
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: secondLeavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the remaining node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, secondLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled down to one node and the list to drain")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, 1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, secondLeavingServiceName)
		})

		g.It("should decommission nodes of several racks one rack at a time", func(ctx g.SpecContext) {
			const otherRackName = "rack-b"

			sdc := setupDecommissioningRacks(ctx, env, false, []string{decommissioningRackName, otherRackName}, decommissioningInitialNodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			leavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(decommissioningInitialNodes-1))
			otherRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)
			otherLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[1], sdc, int(decommissioningInitialNodes-1))

			g.By("Scaling both racks down to one node")
			scaleRackTemplate(ctx, env, sdc.Name, decommissioningInitialNodes-1)

			g.By(fmt.Sprintf("Waiting for the decommission of the %q rack's leaving node to be requested", decommissioningRackName))
			waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)

			g.By(fmt.Sprintf("Verifying the decommission of the %q rack's leaving node is not requested until the %q rack is done", otherRackName, decommissioningRackName))
			o.Consistently(func(co o.Gomega, ctx context.Context) {
				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, otherLeavingServiceName, metav1.GetOptions{})
				co.Expect(err).NotTo(o.HaveOccurred())
				co.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))

				co.Expect(getDecommissioningNodes(ctx, env, sdc.Name, otherRackName)).To(o.BeEmpty())
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By(fmt.Sprintf("Marking the %q rack's node as decommissioned in place of the sidecar", decommissioningRackName))
			setServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueTrue)

			g.By(fmt.Sprintf("Waiting for the %q rack's leaving node to be removed and marking the StatefulSet as rolled out", decommissioningRackName))
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, decommissioningInitialNodes-1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, leavingServiceName)
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

			g.By(fmt.Sprintf("Waiting for the decommission of the %q rack's leaving node to be requested only now", otherRackName))
			waitForServiceDecommissionedLabel(ctx, env, otherLeavingServiceName, naming.LabelValueFalse)

			g.By(fmt.Sprintf("Marking the %q rack's node as decommissioned in place of the sidecar", otherRackName))
			setServiceDecommissionedLabel(ctx, env, otherLeavingServiceName, naming.LabelValueTrue)

			g.By(fmt.Sprintf("Waiting for the %q rack's leaving node to be removed", otherRackName))
			waitForStatefulSetReplicas(ctx, env, otherRackStatefulSetName, decommissioningInitialNodes-1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, otherRackName, otherLeavingServiceName)
		})

		g.It("should wait for a rack's decommissioning node before scaling another rack", func(ctx g.SpecContext) {
			const otherRackName = "rack-b"

			sdc := setupDecommissioningRacks(ctx, env, false, []string{decommissioningRackName, otherRackName}, decommissioningInitialNodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			leavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(decommissioningInitialNodes-1))
			otherRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)

			g.By(fmt.Sprintf("Scaling the %q rack down to one node", decommissioningRackName))
			scaleRack(ctx, env, sdc.Name, decommissioningRackName, decommissioningInitialNodes-1)

			g.By("Waiting for the decommission of the leaving node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)

			g.By(fmt.Sprintf("Scaling the %q rack up while the %q rack is still decommissioning", otherRackName, decommissioningRackName))
			scaleRack(ctx, env, sdc.Name, otherRackName, decommissioningInitialNodes+1)

			g.By(fmt.Sprintf("Verifying the %q rack StatefulSet is not scaled up while the %q rack is decommissioning", otherRackName, decommissioningRackName))
			o.Consistently(func(co o.Gomega, ctx context.Context) {
				sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, otherRackStatefulSetName, metav1.GetOptions{})
				co.Expect(err).NotTo(o.HaveOccurred())
				co.Expect(*sts.Spec.Replicas).To(o.Equal(decommissioningInitialNodes))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueTrue)

			g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet to be scaled down and marking it as rolled out", decommissioningRackName))
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, decommissioningInitialNodes-1)
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

			g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet to be scaled up once the decommission is finished", otherRackName))
			waitForStatefulSetReplicas(ctx, env, otherRackStatefulSetName, decommissioningInitialNodes+1)
		})

		g.It("should extend an ongoing scale-down when the node count is lowered further", func(ctx g.SpecContext) {
			const nodes = int32(3)

			sdc := setupDecommissioningRacks(ctx, env, false, []string{decommissioningRackName}, nodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			firstLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 2)
			secondLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1)

			g.By("Scaling the rack down to two nodes")
			scaleRackTemplate(ctx, env, sdc.Name, nodes-1)

			g.By("Waiting for the decommission of the highest node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, firstLeavingServiceName, naming.LabelValueFalse)

			g.By("Lowering the node count to one while the highest node is still decommissioning")
			scaleRackTemplate(ctx, env, sdc.Name, 1)

			g.By("Verifying the decommission of the uncovered node is not requested while the highest node is still decommissioning")
			o.Consistently(func(co o.Gomega, ctx context.Context) {
				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, secondLeavingServiceName, metav1.GetOptions{})
				co.Expect(err).NotTo(o.HaveOccurred())
				co.Expect(svc.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the highest node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, firstLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the decommission of the uncovered node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, secondLeavingServiceName, naming.LabelValueFalse)

			g.By("Verifying the lowered node count is not reported as a deferred node count change")
			progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
			o.Expect(progressingCondition).NotTo(o.BeNil())
			o.Expect(progressingCondition.Reason).NotTo(o.ContainSubstring("DeferringRackNodeCountChange"))

			g.By("Marking the uncovered node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, secondLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled down to one node and both leaving nodes to be pruned")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, 1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, firstLeavingServiceName)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, secondLeavingServiceName)
		})
	})

	g.Describe("decommissioning with parallel node operations enabled", func() {
		g.It("should decommission a multi-node scale-down at once", func(ctx g.SpecContext) {
			const nodes = int32(3)

			sdc := setupDecommissioningRacks(ctx, env, true, []string{decommissioningRackName}, nodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			lowerLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1)
			higherLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 2)

			g.By("Scaling the rack down to one node")
			scaleRackTemplate(ctx, env, sdc.Name, 1)

			g.By("Waiting for the decommission of both leaving nodes to be requested")
			waitForServiceDecommissionedLabel(ctx, env, lowerLeavingServiceName, naming.LabelValueFalse)
			waitForServiceDecommissionedLabel(ctx, env, higherLeavingServiceName, naming.LabelValueFalse)

			g.By("Waiting for both leaving nodes to be listed in the rack status")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: lowerLeavingServiceName},
					{Name: higherLeavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking only the higher node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, higherLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled below the higher node while the lower node is still decommissioning")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, nodes-1)
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, lowerLeavingServiceName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, naming.LabelValueFalse))

			g.By("Waiting for the higher node's Service to be pruned with the lower node still listed")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				_, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, higherLeavingServiceName, metav1.GetOptions{})
				eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: lowerLeavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the lower node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, lowerLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled down to one node, the lower node's Service to be pruned and the record to drain")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, 1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, lowerLeavingServiceName)
		})

		g.It("should decommission nodes of several racks at once", func(ctx g.SpecContext) {
			const otherRackName = "rack-b"

			sdc := setupDecommissioningRacks(ctx, env, true, []string{decommissioningRackName, otherRackName}, decommissioningInitialNodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			leavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(decommissioningInitialNodes-1))
			otherRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)
			otherLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[1], sdc, int(decommissioningInitialNodes-1))

			g.By("Scaling both racks down to one node")
			scaleRackTemplate(ctx, env, sdc.Name, decommissioningInitialNodes-1)

			// Neither node is marked as decommissioned, so the decommission of the other rack's node can only have
			// been requested if the racks aren't serialized against each other.
			g.By("Waiting for the decommission of the leaving node of both racks to be requested at once")
			waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)
			waitForServiceDecommissionedLabel(ctx, env, otherLeavingServiceName, naming.LabelValueFalse)

			g.By("Waiting for the leaving node of each rack to be listed in its rack status")
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, decommissioningRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: leavingServiceName},
				}))
				eo.Expect(getDecommissioningNodes(ctx, env, sdc.Name, otherRackName)).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
					{Name: otherLeavingServiceName},
				}))
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			g.By("Marking the leaving node of both racks as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueTrue)
			setServiceDecommissionedLabel(ctx, env, otherLeavingServiceName, naming.LabelValueTrue)

			// A rack whose nodes are all removed has to roll out before the other racks are scaled again, so mark
			// each rack as rolled out in place of the StatefulSet controller as it is scaled down.
			g.By("Waiting for both rack StatefulSets to be scaled down")
			for _, stsName := range []string{rackStatefulSetName, otherRackStatefulSetName} {
				waitForStatefulSetReplicas(ctx, env, stsName, decommissioningInitialNodes-1)
				markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), stsName)
			}

			g.By("Waiting for the leaving node of both racks to be pruned and the records to drain")
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, leavingServiceName)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, otherRackName, otherLeavingServiceName)
		})

		g.It("should keep scaling another rack while a rack has a node decommissioning", func(ctx g.SpecContext) {
			const otherRackName = "rack-b"

			sdc := setupDecommissioningRacks(ctx, env, true, []string{decommissioningRackName, otherRackName}, decommissioningInitialNodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			leavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(decommissioningInitialNodes-1))
			otherRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)

			g.By(fmt.Sprintf("Scaling the %q rack down to one node", decommissioningRackName))
			scaleRack(ctx, env, sdc.Name, decommissioningRackName, decommissioningInitialNodes-1)

			g.By("Waiting for the decommission of the leaving node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueFalse)

			g.By(fmt.Sprintf("Scaling the %q rack up while the %q rack is still decommissioning", otherRackName, decommissioningRackName))
			scaleRack(ctx, env, sdc.Name, otherRackName, decommissioningInitialNodes+1)

			g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet to be scaled up while the %q rack is still decommissioning", otherRackName, decommissioningRackName))
			waitForStatefulSetReplicas(ctx, env, otherRackStatefulSetName, decommissioningInitialNodes+1)
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, leavingServiceName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, naming.LabelValueFalse))

			g.By(fmt.Sprintf("Marking the %q rack StatefulSet as rolled out", otherRackName))
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), otherRackStatefulSetName)

			g.By("Marking the node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, leavingServiceName, naming.LabelValueTrue)

			g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet to be scaled down and its leaving node to be pruned", decommissioningRackName))
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, decommissioningInitialNodes-1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, leavingServiceName)
		})

		g.It("should extend an ongoing scale-down when the node count is lowered further", func(ctx g.SpecContext) {
			const nodes = int32(3)

			sdc := setupDecommissioningRacks(ctx, env, true, []string{decommissioningRackName}, nodes)
			rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
			firstLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 2)
			secondLeavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 1)

			g.By("Scaling the rack down to two nodes")
			scaleRackTemplate(ctx, env, sdc.Name, nodes-1)

			g.By("Waiting for the decommission of the highest node to be requested")
			waitForServiceDecommissionedLabel(ctx, env, firstLeavingServiceName, naming.LabelValueFalse)

			g.By("Lowering the node count to one while the highest node is still decommissioning")
			scaleRackTemplate(ctx, env, sdc.Name, 1)

			g.By("Waiting for the decommission of the uncovered node to be requested while the highest node is still decommissioning")
			waitForServiceDecommissionedLabel(ctx, env, secondLeavingServiceName, naming.LabelValueFalse)
			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, firstLeavingServiceName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, naming.LabelValueFalse))

			g.By("Marking the highest node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, firstLeavingServiceName, naming.LabelValueTrue)

			g.By("Verifying the lowered node count is not reported as a deferred node count change")
			progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
			o.Expect(progressingCondition).NotTo(o.BeNil())
			o.Expect(progressingCondition.Reason).NotTo(o.ContainSubstring("DeferringRackNodeCountChange"))

			g.By("Marking the uncovered node as decommissioned in place of the sidecar")
			setServiceDecommissionedLabel(ctx, env, secondLeavingServiceName, naming.LabelValueTrue)

			g.By("Waiting for the rack StatefulSet to be scaled down to one node and both leaving nodes to be pruned")
			waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, 1)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, firstLeavingServiceName)
			waitForServiceToBePrunedAndRecordToDrain(ctx, env, sdc.Name, decommissioningRackName, secondLeavingServiceName)
		})
	})

	g.DescribeTableSubtree("with parallel node operations",
		func(enableParallelNodeOperations bool) {
			g.DescribeTable("should not create a StatefulSet for a new rack while an existing rack is not rolled out",
				func(ctx g.SpecContext, initialRacks, updatedRacks []string, existingRack, newRack string) {
					g.By("Running ScyllaDBDatacenter controller")
					runScyllaDBDatacenterController(ctx, env)

					g.By("Creating ScyllaOperatorConfig singleton")
					createScyllaOperatorConfig(ctx, env)

					g.By("Creating a ScyllaDBDatacenter")
					sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), initialRacks, withEnableParallelNodeOperations(enableParallelNodeOperations))
					sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Waiting for the first StatefulSet to be created")
					existingRackStatefulSetName := naming.StatefulSetNameForRack(makeRackSpec(existingRack), sdc)
					waitForStatefulSet(ctx, env, existingRackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

					g.By("Marking the first rack StatefulSet as not rolled out")
					markStatefulSetAsNotRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), existingRackStatefulSetName)

					g.By("Adding a new rack")
					err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
						if err != nil {
							return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(env.Namespace(), sdc.Name), err)
						}

						sdc.Spec.Racks = makeRackSpecs(updatedRacks...)
						_, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})
						if err != nil {
							return fmt.Errorf("can't update ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
						}

						return nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Verifying the new rack StatefulSet is not created")
					newRackStatefulSetName := naming.StatefulSetNameForRack(makeRackSpec(newRack), sdc)
					o.Consistently(func(co o.Gomega, ctx context.Context) {
						_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, newRackStatefulSetName, metav1.GetOptions{})
						co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
					}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
				},
				g.Entry("when new rack is prepended", []string{"rack-b"}, []string{"rack-a", "rack-b"}, "rack-b", "rack-a"),
				g.Entry("when new rack is appended", []string{"rack-a"}, []string{"rack-a", "rack-b"}, "rack-a", "rack-b"),
			)
		},
		g.Entry("disabled", false),
		// Parallel node operations only relax the initial creation of missing StatefulSets. Adding a rack to an existing
		// datacenter still waits for the existing racks to roll out, so that racks aren't added while another one is
		// scaling or updating, regardless of whether parallel node operations are enabled.
		g.Entry("enabled", true),
	)
})

const (
	// decommissioningRackName is the rack the decommissioning specs scale.
	decommissioningRackName = "rack-a"
	// decommissioningInitialNodes is the node count the decommissioning specs start a rack with, unless they say otherwise.
	decommissioningInitialNodes = int32(2)
)

// leavingOrdinals returns the ordinals of the nodes that leave a rack of the given node count in the first step of a
// scale-down to the target node count: all the nodes above the target with parallel node operations enabled, only the
// highest one otherwise.
func leavingOrdinals(enableParallelNodeOperations bool, nodes, target int32) []int32 {
	if !enableParallelNodeOperations {
		return []int32{nodes - 1}
	}

	var ordinals []int32
	for ordinal := target; ordinal < nodes; ordinal++ {
		ordinals = append(ordinals, ordinal)
	}
	return ordinals
}

// setupDecommissioningRacks runs the controller and brings up rolled-out racks with the given number of nodes each.
func setupDecommissioningRacks(ctx g.SpecContext, env *envtest.Environment, enableParallelNodeOperations bool, rackNames []string, nodes int32) *scyllav1alpha1.ScyllaDBDatacenter {
	g.GinkgoHelper()

	g.By("Running ScyllaDBDatacenter controller")
	runScyllaDBDatacenterController(ctx, env)

	g.By("Creating ScyllaOperatorConfig singleton")
	createScyllaOperatorConfig(ctx, env)

	g.By(fmt.Sprintf("Creating a ScyllaDBDatacenter with %d rack(s) of %d node(s)", len(rackNames), nodes))
	sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), rackNames, withRackTemplateNodes(nodes), withEnableParallelNodeOperations(enableParallelNodeOperations))
	sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// With parallel node operations disabled racks are created one by one, so every rack has to roll out before the
	// next one shows up.
	for _, rack := range sdc.Spec.Racks {
		g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet and member Services to be created", rack.Name))
		rackStatefulSetName := naming.StatefulSetNameForRack(rack, sdc)
		waitForStatefulSet(ctx, env, rackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
		waitForService(ctx, env, naming.MemberServiceName(rack, sdc, int(nodes-1)), scyllaDBDatacenterControllerDefaultEventuallyTimeout)

		g.By(fmt.Sprintf("Marking the %q rack StatefulSet as rolled out", rack.Name))
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)
	}

	g.By("Verifying no node is recorded as decommissioning")
	o.Consistently(func(co o.Gomega, ctx context.Context) {
		for _, rack := range sdc.Spec.Racks {
			co.Expect(getDecommissioningNodes(ctx, env, sdc.Name, rack.Name)).To(o.BeEmpty())
		}
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return sdc
}

// setupDecommissioningRack brings up a rolled-out two-node rack. It returns the ScyllaDBDatacenter, the name of the
// rack StatefulSet and the name of the member Service of the highest ordinal, which is the node that a scale-down by
// one removes.
func setupDecommissioningRack(ctx g.SpecContext, env *envtest.Environment, enableParallelNodeOperations bool) (*scyllav1alpha1.ScyllaDBDatacenter, string, string) {
	g.GinkgoHelper()

	sdc := setupDecommissioningRacks(ctx, env, enableParallelNodeOperations, []string{decommissioningRackName}, decommissioningInitialNodes)
	rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
	leavingServiceName := naming.MemberServiceName(sdc.Spec.Racks[0], sdc, int(decommissioningInitialNodes-1))

	return sdc, rackStatefulSetName, leavingServiceName
}

func waitForStatefulSet(ctx context.Context, e *envtest.Environment, name string, timeout time.Duration) *appsv1.StatefulSet {
	g.GinkgoHelper()

	var statefulSet *appsv1.StatefulSet
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		var err error
		statefulSet, err = e.TypedKubeClient().AppsV1().StatefulSets(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(timeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return statefulSet
}

func waitForService(ctx context.Context, e *envtest.Environment, name string, timeout time.Duration) *corev1.Service {
	g.GinkgoHelper()

	var service *corev1.Service
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		var err error
		service, err = e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(timeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return service
}

// getStatefulSetControllerProgressingCondition returns the progressing condition of the StatefulSet controller from
// the ScyllaDBDatacenter status, or nil if there is none.
func getStatefulSetControllerProgressingCondition(ctx context.Context, e *envtest.Environment, sdcName string) *metav1.Condition {
	g.GinkgoHelper()

	sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return apimeta.FindStatusCondition(sdc.Status.Conditions, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.ProgressingCondition))
}

// getRackStatus returns the status of the named rack, or nil if the rack has none.
func getRackStatus(ctx context.Context, e *envtest.Environment, sdcName, rackName string) *scyllav1alpha1.RackStatus {
	g.GinkgoHelper()

	sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, rackStatus := range sdc.Status.Racks {
		if rackStatus.Name == rackName {
			return &rackStatus
		}
	}

	return nil
}

// getDecommissioningNodes returns the decommissioning nodes recorded in the status of the named rack.
func getDecommissioningNodes(ctx context.Context, e *envtest.Environment, sdcName, rackName string) []scyllav1alpha1.DecommissioningNodeStatus {
	g.GinkgoHelper()

	rackStatus := getRackStatus(ctx, e, sdcName, rackName)
	if rackStatus == nil {
		return nil
	}

	return rackStatus.DecommissioningNodes
}

// removeRacks removes all racks from the spec, returning the error of the update so that admission can be asserted.
// The controller keeps writing the status, so the update is retried on conflict to make sure the error that's returned
// comes from admission and not from a resource version race.
func removeRacks(ctx context.Context, e *envtest.Environment, sdcName string) error {
	g.GinkgoHelper()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(e.Namespace(), sdcName), err)
		}

		sdc.Spec.Racks = nil
		_, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})

		return err
	})
}

func addServiceFinalizer(ctx context.Context, e *envtest.Environment, name string) {
	g.GinkgoHelper()

	updateServiceFinalizers(ctx, e, name, func(finalizers []string) []string {
		return append(finalizers, envtestServiceFinalizer)
	})
}

func removeServiceFinalizer(ctx context.Context, e *envtest.Environment, name string) {
	g.GinkgoHelper()

	updateServiceFinalizers(ctx, e, name, func(finalizers []string) []string {
		return oslices.Filter(finalizers, func(finalizer string) bool {
			return finalizer != envtestServiceFinalizer
		})
	})
}

func updateServiceFinalizers(ctx context.Context, e *envtest.Environment, name string, mutateFunc func([]string) []string) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svc, err := e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get Service %q: %w", naming.ManualRef(e.Namespace(), name), err)
		}

		svc.Finalizers = mutateFunc(svc.Finalizers)
		_, err = e.TypedKubeClient().CoreV1().Services(e.Namespace()).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update Service %q: %w", naming.ObjRef(svc), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func scaleRackTemplate(ctx context.Context, e *envtest.Environment, sdcName string, nodes int32) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(e.Namespace(), sdcName), err)
		}

		sdc.Spec.RackTemplate.Nodes = new(nodes)
		_, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// scaleRack sets the node count of the named rack, overriding the rack template.
func scaleRack(ctx context.Context, e *envtest.Environment, sdcName, rackName string, nodes int32) {
	g.GinkgoHelper()

	sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sdcName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	rackIdx := slices.IndexFunc(sdc.Spec.Racks, func(rack scyllav1alpha1.RackSpec) bool {
		return rack.Name == rackName
	})
	o.Expect(rackIdx).NotTo(o.Equal(-1), "rack %q not found in ScyllaDBDatacenter %q", rackName, naming.ObjRef(sdc))

	// Patch instead of update: the controller keeps writing the status of the same object, so an optimistic update
	// issued while it is reconciling is prone to exhausting the conflict retries.
	_, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Patch(
		ctx,
		sdcName,
		types.JSONPatchType,
		[]byte(fmt.Sprintf(`[{"op": "add", "path": "/spec/racks/%d/nodes", "value": %d}]`, rackIdx, nodes)),
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func setServiceDecommissionedLabel(ctx context.Context, e *envtest.Environment, name, value string) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svc, err := e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get Service %q: %w", naming.ManualRef(e.Namespace(), name), err)
		}

		svc.Labels[naming.DecommissionedLabel] = value
		_, err = e.TypedKubeClient().CoreV1().Services(e.Namespace()).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update Service %q: %w", naming.ObjRef(svc), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForServiceDecommissionedLabel(ctx context.Context, e *envtest.Environment, name, value string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		svc, err := e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(svc.Labels).To(o.HaveKeyWithValue(naming.DecommissionedLabel, value))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

func waitForStatefulSetReplicas(ctx context.Context, e *envtest.Environment, name string, replicas int32) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		sts, err := e.TypedKubeClient().AppsV1().StatefulSets(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(*sts.Spec.Replicas).To(o.Equal(replicas))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

func waitForServiceToBePrunedAndRecordToDrain(ctx context.Context, e *envtest.Environment, sdcName, rackName, serviceName string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		_, err := e.TypedKubeClient().CoreV1().Services(e.Namespace()).Get(ctx, serviceName, metav1.GetOptions{})
		eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())

		eo.Expect(getDecommissioningNodes(ctx, e, sdcName, rackName)).To(o.BeEmpty())
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

func makeEnvtestScyllaDBDatacenter(namespace string, racks []string, mutators ...func(*scyllav1alpha1.ScyllaDBDatacenter)) *scyllav1alpha1.ScyllaDBDatacenter {
	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envtest-sdc",
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ClusterName: "envtest-cluster",
			DNSDomains:  []string{"envtest.scylladb.local"},
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image:               unit.ScyllaDBImageRepository + ":" + configassets.Project.Operator.ScyllaDBVersion,
				EnableDeveloperMode: new(true),
			},
			ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
				Image: new(envtestScyllaDBManagerAgentImage),
			},
			RackTemplate: &scyllav1alpha1.RackTemplate{
				Nodes: new(int32(1)),
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
					},
				},
			},
		},
	}

	sdc.Spec.Racks = makeRackSpecs(racks...)

	for _, mutator := range mutators {
		mutator(sdc)
	}

	return sdc
}

func withEnableParallelNodeOperations(enableParallelNodeOperations bool) func(*scyllav1alpha1.ScyllaDBDatacenter) {
	return func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
		sdc.Spec.EnableParallelNodeOperations = new(enableParallelNodeOperations)
	}
}

func withRackTemplateNodes(nodes int32) func(*scyllav1alpha1.ScyllaDBDatacenter) {
	return func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
		sdc.Spec.RackTemplate.Nodes = new(nodes)
	}
}

func makeRackSpecs(names ...string) []scyllav1alpha1.RackSpec {
	racks := make([]scyllav1alpha1.RackSpec, 0, len(names))
	for _, name := range names {
		racks = append(racks, makeRackSpec(name))
	}
	return racks
}

func makeRackSpec(name string) scyllav1alpha1.RackSpec {
	return scyllav1alpha1.RackSpec{Name: name}
}

func markStatefulSetAsNotRolledOut(ctx context.Context, statefulSets appsv1client.StatefulSetInterface, name string) {
	g.GinkgoHelper()

	statefulSet, err := statefulSets.Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	statefulSet.Status.ObservedGeneration = statefulSet.Generation - 1
	statefulSet.Status.Replicas = 1
	statefulSet.Status.ReadyReplicas = 0
	statefulSet.Status.UpdatedReplicas = 0
	_, err = statefulSets.UpdateStatus(ctx, statefulSet, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// markStatefulSetNodesAsNotReady marks the given number of the StatefulSet's nodes as not ready, as the StatefulSet
// controller would report leaving nodes, while keeping the StatefulSet generation observed.
func markStatefulSetNodesAsNotReady(ctx context.Context, statefulSets appsv1client.StatefulSetInterface, name string, notReadyNodes int32) {
	g.GinkgoHelper()

	statefulSet, err := statefulSets.Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	replicas := *statefulSet.Spec.Replicas
	o.Expect(notReadyNodes).To(o.BeNumerically("<=", replicas))
	statefulSet.Status.ObservedGeneration = statefulSet.Generation
	statefulSet.Status.Replicas = replicas
	statefulSet.Status.ReadyReplicas = replicas - notReadyNodes
	statefulSet.Status.AvailableReplicas = replicas - notReadyNodes
	statefulSet.Status.UpdatedReplicas = replicas
	statefulSet.Status.CurrentRevision = "envtest-revision"
	statefulSet.Status.UpdateRevision = statefulSet.Status.CurrentRevision
	_, err = statefulSets.UpdateStatus(ctx, statefulSet, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func markStatefulSetAsRolledOut(ctx context.Context, statefulSets appsv1client.StatefulSetInterface, name string) {
	g.GinkgoHelper()

	statefulSet, err := statefulSets.Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	replicas := *statefulSet.Spec.Replicas
	statefulSet.Status.ObservedGeneration = statefulSet.Generation
	statefulSet.Status.Replicas = replicas
	statefulSet.Status.ReadyReplicas = replicas
	statefulSet.Status.AvailableReplicas = replicas
	statefulSet.Status.UpdatedReplicas = replicas
	statefulSet.Status.CurrentRevision = "envtest-revision"
	statefulSet.Status.UpdateRevision = statefulSet.Status.CurrentRevision
	_, err = statefulSets.UpdateStatus(ctx, statefulSet, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

type staticKeyGenerator struct {
	key stdcrypto.Signer
}

func newStaticKeyGenerator() *staticKeyGenerator {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	o.Expect(err).NotTo(o.HaveOccurred())

	return &staticKeyGenerator{key: key}
}

func (g *staticKeyGenerator) GetNewKey(ctx context.Context) (stdcrypto.Signer, error) {
	return g.key, nil
}

func (g *staticKeyGenerator) GetKeyType() crypto.KeyType {
	return crypto.ECDSAKeyType
}

func runScyllaDBDatacenterController(ctx context.Context, e *envtest.Environment) {
	g.GinkgoHelper()

	kubeClient := e.TypedKubeClient()
	scyllaClient := e.ScyllaClient()
	kubeInformers := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		scyllaDBDatacenterControllerResyncPeriod,
		informers.WithNamespace(e.Namespace()),
	)
	scyllaInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		scyllaClient,
		scyllaDBDatacenterControllerResyncPeriod,
		scyllainformers.WithNamespace(e.Namespace()),
	)
	scyllaGlobalInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		scyllaClient,
		scyllaDBDatacenterControllerResyncPeriod,
		scyllainformers.WithNamespace(corev1.NamespaceAll),
	)
	keyGenerator := newStaticKeyGenerator()

	options := []scylladbdatacenter.ControllerOption{
		// The default delay only slows tests down; tests that need to exercise cache lag should override this.
		scylladbdatacenter.WithStatefulSetCachePropagationDelay(scyllaDBDatacenterControllerDisabledStatefulSetCachePropagationDelay),
	}

	sdcc, err := scylladbdatacenter.NewController(
		kubeClient,
		scyllaClient.ScyllaV1alpha1(),
		kubeInformers.Core().V1().Pods(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Apps().V1().StatefulSets(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Networking().V1().Ingresses(),
		kubeInformers.Batch().V1().Jobs(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenterNodesStatusReports(),
		scyllaGlobalInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		"scylla/operator:envtest",
		scylla.DefaultNativeTransportPort,
		keyGenerator,
		options...,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	kubeInformers.Start(ctx.Done())
	scyllaInformers.Start(ctx.Done())
	scyllaGlobalInformers.Start(ctx.Done())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sdcc.Run(ctx, 1)
	}()

	g.DeferCleanup(func() {
		kubeInformers.Shutdown()
		scyllaInformers.Shutdown()
		scyllaGlobalInformers.Shutdown()
		wg.Wait()
	})
}
