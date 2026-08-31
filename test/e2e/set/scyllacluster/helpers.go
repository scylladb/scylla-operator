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

// createClusterAndWaitForRollout creates a ScyllaCluster with the specified number of members and waits for rollout.
func createClusterAndWaitForRollout(
	ctx context.Context,
	f *framework.Framework,
	members int32,
) *scyllav1.ScyllaCluster {
	sc := f.GetDefaultScyllaCluster()
	sc.Spec.Datacenter.Racks[0].Members = members

	framework.By("Creating a %d node ScyllaCluster", members)
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

	return sc
}

// scaleClusterAndWaitForRollout scales the cluster to the given number of members and waits for rollout.
func scaleClusterAndWaitForRollout(
	ctx context.Context,
	f *framework.Framework,
	sc *scyllav1.ScyllaCluster,
	members int32,
) *scyllav1.ScyllaCluster {
	patchData := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`, members))

	framework.By("Scaling the ScyllaCluster to %d members", members)
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
		ctx,
		sc.Name,
		types.JSONPatchType,
		patchData,
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
	o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(members))

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

	return sc
}
