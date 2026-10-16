// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func addQuantity(lhs resource.Quantity, rhs resource.Quantity) *resource.Quantity {
	res := lhs.DeepCopy()
	res.Add(rhs)

	// Pre-cache the string so DeepEqual works.
	_ = res.String()

	return &res
}

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should rolling restart cluster when forceRedeploymentReason is changed", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		initialPodUID := pod.UID
		framework.Infof("Initial pod %q UID is %q", pod.Name, initialPodUID)

		sc, err = utils.PatchScyllaClusterForceRedeploymentReason(ctx, f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()), sc.Name, "foo")
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		pod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(pod.UID).NotTo(o.Equal(initialPodUID))
	})

	g.It("should reconcile resource changes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Changing pod resources")
		oldResources := *sc.Spec.Datacenter.Racks[0].Resources.DeepCopy()
		newResources := *oldResources.DeepCopy()
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceMemory))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceMemory))

		newResources.Requests[corev1.ResourceCPU] = *addQuantity(newResources.Requests[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Requests[corev1.ResourceMemory] = *addQuantity(newResources.Requests[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Requests).NotTo(o.BeEquivalentTo(oldResources.Requests))

		newResources.Limits[corev1.ResourceCPU] = *addQuantity(newResources.Limits[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Limits[corev1.ResourceMemory] = *addQuantity(newResources.Limits[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Limits).NotTo(o.BeEquivalentTo(oldResources.Limits))

		o.Expect(newResources).NotTo(o.BeEquivalentTo(oldResources))

		newResourcesJSON, err := json.Marshal(newResources)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources", "value": %s}]`,
				newResourcesJSON,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Resources).To(o.BeEquivalentTo(newResources))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHosts := hosts
		oldHostIDs := hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(len(oldHosts)))
		o.Expect(hostIDs).To(o.ConsistOf(oldHostIDs))

		// Reset hosts as the client won't be able to discover a single node after rollout.
		err = di.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verification.VerifyCQLData(ctx, di)

		framework.By("Scaling the ScyllaCluster up to create a new replica")
		oldMembers := sc.Spec.Datacenter.Racks[0].Members
		newMebmers := oldMembers + 1
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`,
				newMebmers,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.Equal(newMebmers))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHostIDs = hostIDs
		_, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)

		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHostIDs).To(o.HaveLen(int(oldMembers)))
		o.Expect(hostIDs).To(o.HaveLen(int(newMebmers)))
		o.Expect(hostIDs).To(o.ContainElements(oldHostIDs))
		verification.VerifyCQLData(ctx, di)
	})

	g.It("should recover from a stuck rollout when the spec is reverted", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))
		// With parallel node operations the StatefulSet controller replaces the stuck Pod on its own, so disable them
		// to have the operator do it.
		sc.Spec.EnableParallelNodeOperations = pointer.Ptr(false)

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		patchRackResources := func(resources corev1.ResourceRequirements) {
			resourcesJSON, err := json.Marshal(resources)
			o.Expect(err).NotTo(o.HaveOccurred())

			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
				ctx,
				sc.Name,
				types.JSONPatchType,
				[]byte(fmt.Sprintf(
					`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources", "value": %s}]`,
					resourcesJSON,
				)),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sc.Spec.Datacenter.Racks[0].Resources).To(o.BeEquivalentTo(resources))
		}

		framework.By("Requesting more memory than any node has")
		oldResources := *sc.Spec.Datacenter.Racks[0].Resources.DeepCopy()
		unschedulableResources := *oldResources.DeepCopy()
		unschedulableResources.Requests[corev1.ResourceMemory] = resource.MustParse("1000Ti")
		unschedulableResources.Limits[corev1.ResourceMemory] = resource.MustParse("1000Ti")
		patchRackResources(unschedulableResources)

		framework.By("Waiting for the Pod to get stuck unschedulable")
		// The old Pod has to terminate first.
		waitCtx2, waitCtx2Cancel := context.WithTimeout(ctx, utils.ScyllaDBTerminationTimeout+utils.SyncTimeout)
		defer waitCtx2Cancel()
		_, err = controllerhelpers.WaitForPodState(waitCtx2, f.KubeClient().CoreV1().Pods(f.Namespace()), utils.GetNodeName(sc, 0), controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(pod *corev1.Pod) (bool, error) {
			scheduledCondition := controllerhelpers.GetPodCondition(pod.Status.Conditions, corev1.PodScheduled)
			return scheduledCondition != nil && scheduledCondition.Status == corev1.ConditionFalse && scheduledCondition.Reason == corev1.PodReasonUnschedulable, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Reverting the resources")
		patchRackResources(oldResources)

		framework.By("Waiting for the ScyllaCluster to roll out")
		// The operator considers a rollout stuck once a Pod of the update revision has been unready for 5 minutes.
		waitCtx3, waitCtx3Cancel := context.WithTimeout(ctx, 5*time.Minute+utils.RolloutTimeoutForScyllaCluster(sc))
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHostIDs := hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hostIDs).To(o.ConsistOf(oldHostIDs))

		err = di.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verification.VerifyCQLData(ctx, di)
	})
})
