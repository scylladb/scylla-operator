// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, framework.SuiteKindClusterTopology, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	type scalingStep struct {
		rackLayout
		// beforeFunc, when set, runs against the cluster in the state preceding the scaling operation.
		beforeFunc func(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster)
		// verifyFunc asserts how the cluster's members changed over the scaling operation.
		// leftHostIDs holds the host IDs of every node which has left the cluster.
		verifyFunc func(previousHostIDs, hostIDs, leftHostIDs []string)
	}

	type horizontalScalingEntry struct {
		initialRackLayout rackLayout
		steps             []scalingStep
	}

	g.DescribeTable("should support horizontal scaling", func(ctx g.SpecContext, e *horizontalScalingEntry) {
		sc := createClusterAndWaitForRollout(ctx, f, e.initialRackLayout)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(int(utils.GetMemberCount(sc))))
		o.Expect(hostIDs).To(o.HaveLen(int(utils.GetMemberCount(sc))))

		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		// Host IDs of nodes that have left the cluster. None of them may be resurrected - a node scaled back up has to
		// bootstrap from scratch, instead of reusing storage left over from a decommissioned node.
		var leftHostIDs []string

		for _, step := range e.steps {
			if step.beforeFunc != nil {
				step.beforeFunc(ctx, f, sc)
			}

			previousHostIDs := hostIDs

			sc = scaleClusterAndWaitForRollout(ctx, f, sc, step.rackLayout)
			scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

			hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(hosts).To(o.HaveLen(int(utils.GetMemberCount(sc))))
			o.Expect(hostIDs).To(o.HaveLen(int(utils.GetMemberCount(sc))))

			for _, previousHostID := range previousHostIDs {
				if !slices.Contains(hostIDs, previousHostID) {
					leftHostIDs = append(leftHostIDs, previousHostID)
				}
			}

			step.verifyFunc(previousHostIDs, hostIDs, leftHostIDs)

			verification.VerifyCQLData(ctx, di)
		}
	},
		// Scaling by more than a single node at a time is what distinguishes parallel node operations from sequential
		// ones - a step moving a single node is indistinguishable between the two.
		g.Entry("out", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 1},
			steps: []scalingStep{
				{rackCount: 1, membersPerRack: 3, verifyFunc: verifyScaledOut},
			},
		}),
		g.Entry("out, into new racks", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 1},
			steps: []scalingStep{
				{rackCount: 3, membersPerRack: 1, verifyFunc: verifyScaledOut},
			},
		}),
		g.Entry("out, across multiple racks", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 3, membersPerRack: 1},
			steps: []scalingStep{
				{rackCount: 3, membersPerRack: 2, verifyFunc: verifyScaledOut},
			},
		}),
		g.Entry("in", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 3},
			steps: []scalingStep{
				{rackCount: 1, membersPerRack: 1, verifyFunc: verifyScaledIn},
			},
		}),
		g.Entry("in, across multiple racks", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 3, membersPerRack: 2},
			steps: []scalingStep{
				{rackCount: 3, membersPerRack: 1, verifyFunc: verifyScaledIn},
			},
		}),
		// Draining a node leaves it in ScyllaDB's DRAINED operational mode, which no longer accepts writes and cannot be
		// undrained - the sidecar has to restart ScyllaDB to make the node decommissionable at all. Maintenance mode is
		// how that state is reachable in practice: it makes the readiness probe report the node as not ready, so the
		// operator does not restart it while nodetool is driven against it by hand.
		//
		// Scaling in from here therefore exercises a different code path than an ordinary decommission. The drain targets
		// the highest ordinal node, so this entry deliberately scales in by a single node.
		g.Entry("in, when the node has been drained in maintenance mode", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 3},
			steps: []scalingStep{
				{rackCount: 1, membersPerRack: 2, beforeFunc: markHighestOrdinalForMaintenanceAndDrain, verifyFunc: verifyScaledIn},
			},
		}),
		// Scaling back out verifies that a decommissioned node's storage isn't left in place and reused.
		g.Entry("out, with new storage after decommissioning", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 3},
			steps: []scalingStep{
				{rackCount: 1, membersPerRack: 1, verifyFunc: verifyScaledIn},
				{rackCount: 1, membersPerRack: 3, verifyFunc: verifyScaledOutWithNewNodes},
			},
		}),
	)
})

// verifyScaledOut asserts that scaling out preserved every node the cluster already had.
func verifyScaledOut(previousHostIDs, hostIDs, _ []string) {
	g.GinkgoHelper()

	o.Expect(hostIDs).To(o.ContainElements(previousHostIDs))
}

// verifyScaledIn asserts that scaling in only removed nodes, without replacing any of the remaining ones.
func verifyScaledIn(previousHostIDs, hostIDs, _ []string) {
	g.GinkgoHelper()

	o.Expect(previousHostIDs).To(o.ContainElements(hostIDs))
}

// verifyScaledOutWithNewNodes asserts that scaling out preserved the existing nodes and bootstrapped genuinely new
// ones, instead of resurrecting a node which has left the cluster from storage left behind after it.
func verifyScaledOutWithNewNodes(previousHostIDs, hostIDs, leftHostIDs []string) {
	g.GinkgoHelper()

	verifyScaledOut(previousHostIDs, hostIDs, leftHostIDs)

	// Guard against the assertion below passing vacuously before any node has left the cluster.
	o.Expect(leftHostIDs).NotTo(o.BeEmpty())
	for _, leftHostID := range leftHostIDs {
		o.Expect(hostIDs).NotTo(
			o.ContainElement(leftHostID),
			"host ID %q of a node which has left the cluster must not be resurrected", leftHostID,
		)
	}
}

// markHighestOrdinalForMaintenanceAndDrain puts the highest ordinal node of the first rack into maintenance mode and drains it.
func markHighestOrdinalForMaintenanceAndDrain(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	g.GinkgoHelper()

	o.Expect(sc.Spec.Datacenter.Racks).NotTo(o.BeEmpty())
	rack := sc.Spec.Datacenter.Racks[0]
	o.Expect(rack.Members).NotTo(o.BeZero())
	ordinal := int(rack.Members) - 1
	podName := naming.PodNameForScyllaCluster(rack, sc, ordinal)
	svcName := naming.MemberServiceNameForScyllaCluster(rack, sc, ordinal)

	framework.By("Marking ScyllaCluster node #%d (%s) for maintenance", ordinal, podName)
	svc := &corev1.Service{
		Labels: map[string]string{
			naming.NodeMaintenanceLabel: "",
		},
	}
	patch, err := helpers.CreateTwoWayMergePatch(&corev1.Service{}, svc)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = f.KubeClient().CoreV1().Services(sc.Namespace).Patch(
		ctx,
		svcName,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Manually draining ScyllaCluster node #%d (%s)", ordinal, podName)
	ec := &corev1.EphemeralContainer{
		TargetContainerName: naming.ScyllaContainerName,
		Name:                "e2e-drain-scylla",
		Image:               scylladbdatacenter.ImageForCluster(sc),
		ImagePullPolicy:     corev1.PullIfNotPresent,
		Command:             []string{"/usr/bin/nodetool", "drain"},
		Args:                []string{},
	}
	pod, err := utils.RunEphemeralContainerAndWaitForCompletion(ctx, f.KubeClient().CoreV1().Pods(sc.Namespace), podName, ec)
	o.Expect(err).NotTo(o.HaveOccurred())
	ephemeralContainerState := controllerhelpers.FindContainerStatus(pod, ec.Name)
	o.Expect(ephemeralContainerState).NotTo(o.BeNil())
	o.Expect(ephemeralContainerState.State.Terminated).NotTo(o.BeNil())
	o.Expect(ephemeralContainerState.State.Terminated.ExitCode).To(o.BeEquivalentTo(0))
}

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should scale-up vertically after SMP change", func(testCtx context.Context) {
		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating a ScyllaCluster with 3 members")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(testCtx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(testCtx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(testCtx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(testCtx, f.KubeClient().CoreV1(), sc)

		framework.By("Verifying that all ScyllaDB pods are running with --smp=1")
		assertPodsSMPEquals(testCtx, f, sc, 1)

		framework.By("Patching CPU limit from 1 to 2 to trigger SMP change")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			testCtx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources/limits/cpu", "value": "2"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		postUpdateRolloutCtx, postUpdateRolloutCtxCancel := utils.ContextForRollout(testCtx, sc)
		defer postUpdateRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(postUpdateRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that all ScyllaDB pods are running with --smp=2")
		assertPodsSMPEquals(testCtx, f, sc, 2)
	})
})

func assertPodsSMPEquals(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster, expectedSMP int) {
	stsName := naming.StatefulSetNameForRackForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc)
	for i := int32(0); i < sc.Spec.Datacenter.Racks[0].Members; i++ {
		podName := fmt.Sprintf("%s-%d", stsName, i)
		entrypointCommand, err := utils.GetScyllaDBDockerEntrypointCommand(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), f.Namespace(), podName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(entrypointCommand).To(o.ContainSubstring(fmt.Sprintf("--smp=%d", expectedSMP)), "pod %q should have --smp=%d in entrypoint command", podName, expectedSMP)
	}
}
