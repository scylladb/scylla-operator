// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// rackLayout is the racks of a ScyllaCluster, named explicitly, and the number of members in each of them,
// mirroring how ScyllaDBDatacenter itself layers a per-rack override (RackSpec.Nodes) under a datacenter-wide
// default (RackTemplate.Nodes): every named rack has defaultMemberCount members, unless memberCounts overrides it for
// that rack specifically.
type rackLayout struct {
	racks              []string
	defaultMemberCount int32
	// memberCounts overrides defaultMemberCount for the named racks. A key not present in racks is an error.
	memberCounts map[string]int32
}

func (rl rackLayout) String() string {
	if len(rl.memberCounts) != 0 {
		return fmt.Sprintf("racks %v, %d member(s) each except %v", rl.racks, rl.defaultMemberCount, rl.memberCounts)
	}
	return fmt.Sprintf("racks %v, %d member(s) each", rl.racks, rl.defaultMemberCount)
}

// membersOfRack returns the member count of the named rack: its override from memberCounts if it has one, else
// defaultMemberCount.
func (rl rackLayout) membersOfRack(name string) int32 {
	if members, ok := rl.memberCounts[name]; ok {
		return members
	}
	return rl.defaultMemberCount
}

// createClusterAndWaitForRollout creates a ScyllaCluster with the given rack layout and waits for rollout.
func createClusterAndWaitForRollout(
	ctx context.Context,
	f *framework.Framework,
	rl rackLayout,
) *scyllav1.ScyllaCluster {
	sc := f.GetDefaultScyllaCluster()
	sc.Spec.Datacenter.Racks = replicateRackSpecs(sc, rl)

	framework.By("Creating a ScyllaCluster with %s", rl)
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	expectRackLayout(sc, rl)

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
	scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

	return sc
}

// scaleClusterAndWaitForRollout scales the cluster to the given rack layout and waits for rollout. The entire rack
// array is replaced, so a single scaling operation can both add racks and change the member count of every rack at
// once.
func scaleClusterAndWaitForRollout(
	ctx context.Context,
	f *framework.Framework,
	sc *scyllav1.ScyllaCluster,
	rl rackLayout,
) *scyllav1.ScyllaCluster {
	scCopy := sc.DeepCopy()
	scCopy.Spec.Datacenter.Racks = replicateRackSpecs(sc, rl)
	patch, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Scaling the ScyllaCluster to %s", rl)
	sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	expectRackLayout(sc, rl)

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
	scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

	return sc
}

// replicateRackSpecs fans the ScyllaCluster's first rack out into the given rack layout.
func replicateRackSpecs(sc *scyllav1.ScyllaCluster, rl rackLayout) []scyllav1.RackSpec {
	o.Expect(sc.Spec.Datacenter.Racks).NotTo(o.BeEmpty())
	o.Expect(rl.racks).NotTo(o.BeEmpty())

	rackSpecs := make([]scyllav1.RackSpec, 0, len(rl.racks))
	for _, name := range rl.racks {
		rackSpec := sc.Spec.Datacenter.Racks[0].DeepCopy()
		rackSpec.Name = name
		rackSpec.Members = rl.membersOfRack(name)
		rackSpecs = append(rackSpecs, *rackSpec)
	}

	for name := range rl.memberCounts {
		o.Expect(rl.racks).To(o.ContainElement(name), "memberCounts names rack %q, which isn't among the layout's rack names %v", name, rl.racks)
	}

	return rackSpecs
}

// expectRackLayout asserts that the ScyllaCluster has the given rack layout.
func expectRackLayout(sc *scyllav1.ScyllaCluster, rl rackLayout) {
	o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(len(rl.racks)))
	for _, rackSpec := range sc.Spec.Datacenter.Racks {
		o.Expect(rackSpec.Members).To(o.BeEquivalentTo(rl.membersOfRack(rackSpec.Name)))
	}
}
