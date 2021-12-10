// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
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

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
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
		err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, utils.GetMemberCount(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

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
		conditions := []func(cluster *scyllav1.ScyllaCluster) (bool, error){
			registeredInManagerCond,
			repairTaskScheduledCond,
			backupTaskSyncFailedCond,
		}
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, conditions...)
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
