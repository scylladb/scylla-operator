package manager

import (
	"context"
	"fmt"
	"net/http"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	managerfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/manager"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Repair task", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllamanager")

	g.It("Repair task should end without an error", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sm := managerfixture.Manager.ReadOrFail()
		sm.Spec.Repairs = []v1alpha1.RepairTaskSpec{
			{
				SchedulerTaskSpec: v1alpha1.SchedulerTaskSpec{
					Name: "test-e2e-repair",
				},
			},
		}
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

		framework.By("Creating a ScyllaManager with repair task")
		sm.Spec.Database.Connection.Server = naming.CrossNamespaceServiceNameForCluster(mc)
		sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaManagers(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the repair task to end")
		waitCtx3, waitCtx3Cancel := utils.ContextForScyllaManagerTask(ctx)
		defer waitCtx3Cancel()
		sm, err = utils.WaitForManagerState(waitCtx3, f.ScyllaClient().ScyllaV1alpha1(), sm.Namespace, sm.Name, utils.WaitForStateOptions{}, utils.IsRepairDone(sc, sm.Spec.Repairs[0].Name))
		o.Expect(err).NotTo(o.HaveOccurred())

		s, err := f.KubeAdminClient().CoreV1().Services(sm.Namespace).Get(ctx, sm.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		client, err := managerclient.NewClient(fmt.Sprintf("http://%s/api/v1", s.Spec.ClusterIP), &http.Transport{})
		o.Expect(err).NotTo(o.HaveOccurred())
		clusters, err := client.ListClusters(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(clusters)).To(o.BeEquivalentTo(1))
		cluster := clusters[0]
		tasks, err := client.ListTasks(ctx, cluster.ID, "repair", true, "DONE")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(tasks.ExtendedTaskSlice)).To(o.BeEquivalentTo(1))
		task := tasks.ExtendedTaskSlice[0]
		o.Expect(task.Name).To(o.BeEquivalentTo(sm.Spec.Repairs[0].Name))
	})
})
