package manager

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	managerfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/manager"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaManager register cluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllamanager")

	g.It("should register and unregister ScyllaCluster", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sm := managerfixture.Manager.ReadOrFail()

		mc := scyllafixture.BasicScyllaCluster.ReadOrFail()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()

		framework.By("Creating a ScyllaManager Cluster")
		mc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, mc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the ScyllaManager Cluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, mc)
		defer waitCtx1Cancel()
		mc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), mc.Namespace, mc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaCluster")
		sc.Labels = sm.Spec.ScyllaClusterSelector.MatchLabels
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaManager")
		sm.Spec.Database.Connection.Server = naming.CrossNamespaceServiceNameForCluster(mc)
		sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaManagers(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the SM to register the SC")
		waitCtx3, waitCtx3Cancel := utils.ContextForScyllaManagerTask(ctx)
		defer waitCtx3Cancel()
		sm, err = utils.WaitForManagerState(waitCtx3, f.ScyllaClient().ScyllaV1alpha1(), sm.Namespace, sm.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRegisteredInManager(sc))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Deleting Scylla Cluster")
		err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Delete(ctx, sc.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the SM to unregister the SC")
		waitCtx4, waitCtx4Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx4Cancel()
		sm, err = utils.WaitForManagerState(waitCtx4, f.ScyllaClient().ScyllaV1alpha1(), sm.Namespace, sm.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterNotRegisteredInManager(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
