package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-openapi/strfmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	managerfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/manager"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.FDescribe("Backup task", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllamanager")

	g.It("Backup task should end without an error", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sm := managerfixture.Manager.ReadOrFail()
		keyspace := []string{"test"}
		location := []string{"s3:my-bucket"}
		sm.Spec.Backups = []v1alpha1.BackupTaskSpec{
			{
				SchedulerTaskSpec: v1alpha1.SchedulerTaskSpec{
					Name: "test-e2e-backup",
				},
				Keyspace: keyspace,
				Location: location,
			},
		}

		mc := scyllafixture.BasicScyllaCluster.ReadOrFail()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.AgentVersion = "3.0.0"
		sc.Spec.Datacenter.Racks[0].ScyllaAgentConfig = AgentConfigName

		framework.By("Creating a ScyllaManager Cluster")
		mc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, mc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the ScyllaManager Cluster to deploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, mc)
		defer waitCtx2Cancel()
		mc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), mc.Namespace, mc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating Manager Agent Config")
		_, err = f.KubeClient().CoreV1().Secrets(f.Namespace()).Create(ctx, AgentSecret(), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaCluster")
		sc.Labels = sm.Spec.ScyllaClusterSelector.MatchLabels
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Insert sample data to ScyllaCluster")
		inserter, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), "test", sc, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = inserter.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaManager with backup task")
		sm.Spec.Database.Connection.Server = naming.CrossNamespaceServiceNameForCluster(mc)
		sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaManagers(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		waitCtx5, waitCtx5Cancel := utils.ContextForScyllaManagerTask(ctx)
		defer waitCtx5Cancel()
		framework.By("Waiting for a Manager to register Cluster")
		sm, err = utils.WaitForManagerState(waitCtx5, f.ScyllaClient().ScyllaV1alpha1(), sm.Namespace, sm.Name, utils.WaitForStateOptions{}, utils.IsScyllaManagerRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the backup task to end")
		waitCtx6, waitCtx6Cancel := utils.ContextForScyllaManagerTask(ctx)
		defer waitCtx6Cancel()
		sm, err = utils.WaitForManagerState(waitCtx6, f.ScyllaClient().ScyllaV1alpha1(), sm.Namespace, sm.Name, utils.WaitForStateOptions{}, utils.IsBackupDone(sc, sm.Spec.Backups[0].Name))
		o.Expect(err).NotTo(o.HaveOccurred())

		s, err := f.KubeAdminClient().CoreV1().Services(sm.Namespace).Get(ctx, sm.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		client, err := managerclient.NewClient(fmt.Sprintf("http://%s/api/v1", s.Spec.ClusterIP), &http.Transport{})
		o.Expect(err).NotTo(o.HaveOccurred())
		clusters, err := client.ListClusters(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(clusters)).To(o.BeEquivalentTo(1))
		cluster := clusters[0]
		tasks, err := client.ListTasks(ctx, cluster.ID, "backup", true, "DONE")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(tasks.ExtendedTaskSlice)).To(o.BeEquivalentTo(1))
		task := tasks.ExtendedTaskSlice[0]
		o.Expect(task.Name).To(o.BeEquivalentTo(sm.Spec.Backups[0].Name))
		backups, err := client.ListBackups(ctx, cluster.ID, location, false, keyspace, strfmt.NewDateTime(), strfmt.DateTime(time.Now()))
		o.Expect(len(backups)).To(o.BeEquivalentTo(1))
		backup := backups[0]
		o.Expect(backup.TaskID).To(o.BeEquivalentTo(task.ID))
	})
})

const AgentConfigName = "scylla-agent-config"

func AgentSecret() *corev1.Secret {
	type S3 struct {
		AccessKeyID     string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_access_key"`
		Provider        string `json:"provider"`
		Region          string `json:"region"`
		Endpoint        string `json:"endpoint"`
	}
	type AgentConfig struct {
		S3 S3 `json:"s3"`
	}
	data, err := json.Marshal(AgentConfig{
		S3: S3{
			AccessKeyID:     "miniouser",
			SecretAccessKey: "minio1234",
			Provider:        "Minio",
			Region:          "us-east-1",
			Endpoint:        "http://minio.minio.svc:9000",
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: AgentConfigName,
		},
		Data: map[string][]byte{"scylla-manager-agent.yaml": data},
		Type: corev1.SecretTypeOpaque,
	}
}
