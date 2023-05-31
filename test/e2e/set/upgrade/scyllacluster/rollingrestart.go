// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ScyllaClusterRolloutUpgradeTest struct {
	scyllaCluster *scyllav1.ScyllaCluster
}

func (t *ScyllaClusterRolloutUpgradeTest) Name() string {
	return "ScyllaCluster rolling restart after upgrade"
}

func (t *ScyllaClusterRolloutUpgradeTest) Setup(ctx context.Context, f *framework.Framework) {
	sc := scyllafixture.BasicScyllaCluster.ReadOrFail()

	framework.By("Creating a ScyllaCluster")
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
	waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
	defer waitCtx1Cancel()
	sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	t.scyllaCluster = sc
}

func (t *ScyllaClusterRolloutUpgradeTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}) {
	framework.By("Waiting for upgrade to complete before rolling restart")
	<-done

	framework.By("Initiating a rolling restart of ScyllaCluster")

	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
		ctx,
		t.scyllaCluster.Name,
		types.MergePatchType,
		[]byte(`{"spec": {"forceRedeploymentReason": "after upgrade rollout"}}`),
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
	waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
	defer waitCtx1Cancel()
	sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (t *ScyllaClusterRolloutUpgradeTest) Teardown(ctx context.Context, f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
