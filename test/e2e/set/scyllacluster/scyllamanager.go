// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Scylla Manager integration", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should discover cluster and sync tasks", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.Repairs = append(sc.Spec.Repairs, scyllav1.RepairTaskSpec{
			SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
				Name:     "weekly repair",
				Interval: "7d",
			},
			Parallel: 123,
		})

		// Backup scheduling is going to fail because location is not accessible in our env.
		sc.Spec.Backups = append(sc.Spec.Backups, scyllav1.BackupTaskSpec{
			SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
				Name:     "daily backup",
				Interval: "1d",
			},
			Location: []string{"s3:bucket"},
		})

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Waiting for the cluster sync with Scylla Manager")
		registeredInManagerCond := func(sc *scyllav1.ScyllaCluster) (bool, error) {
			return sc.Status.ManagerID != nil, nil
		}

		repairTaskScheduledCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					return r.ID != "", nil
				}
			}
			return false, nil
		}

		backupTaskSyncFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sc.Spec.Backups[0].Name {
					return b.Error != "", nil
				}
			}
			return false, nil
		}

		waitCtx2, waitCtx2Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, registeredInManagerCond, repairTaskScheduledCond, backupTaskSyncFailedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that task properties were synchronized")
		tasks, err := managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.ExtendedTaskSlice).To(o.HaveLen(1))
		repairTask := tasks.ExtendedTaskSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(int64(123)))
	})
})
