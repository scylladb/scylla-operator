// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("Scylla Manager integration", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should discover cluster and sync tasks", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Waiting for ScyllaCluster to register with Scylla Manager")
		registeredInManagerCond := func(sc *scyllav1.ScyllaCluster) (bool, error) {
			return sc.Status.ManagerID != nil, nil
		}

		waitCtx2, waitCtx2Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, registeredInManagerCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scheduling a backup and repair for ScyllaCluster")
		scCopy := sc.DeepCopy()
		scCopy.Spec.Repairs = append(scCopy.Spec.Repairs, scyllav1.RepairTaskSpec{
			SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
				Name:     "weekly",
				Interval: "7d",
			},
			Parallel: 123,
		})
		scCopy.Spec.Backups = append(scCopy.Spec.Backups, scyllav1.BackupTaskSpec{
			SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
				Name:     "daily",
				Interval: "1d",
			},
			Location: []string{"s3:bucket"},
		})

		patchData, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("weekly"))
		o.Expect(sc.Spec.Repairs[0].Interval).To(o.Equal("7d"))
		o.Expect(sc.Spec.Repairs[0].Parallel).To(o.Equal(int64(123)))
		o.Expect(sc.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sc.Spec.Backups[0].Name).To(o.Equal("daily"))

		framework.By("Waiting for ScyllaCluster to sync tasks with Scylla Manager")
		repairTaskScheduledCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					if len(r.ID) < 1 {
						return false, nil
					}

					if len(r.Error) > 0 {
						return false, fmt.Errorf(r.Error)
					}

					return true, nil
				}
			}
			return false, nil
		}

		backupTaskSchedulingFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sc.Spec.Backups[0].Name {
					return b.Error != "", nil
				}
			}
			return false, nil
		}

		waitCtx3, waitCtx3Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskScheduledCond, backupTaskSchedulingFailedCond)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Interval).To(o.Equal(sc.Spec.Repairs[0].Interval))
		o.Expect(sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that repair task properties were synchronized")
		tasks, err := managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask := tasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Schedule.Interval).To(o.Equal(sc.Status.Repairs[0].Interval))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(sc.Status.Repairs[0].Parallel))

		framework.By("Updating the repair task for ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/repairs/0/parallel","value":1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs[0].Parallel).To(o.Equal(int64(1)))

		framework.By("Waiting for ScyllaCluster to sync repair task update with Scylla Manager")
		repairTaskUpdatedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					if len(r.ID) == 0 {
						return false, fmt.Errorf("got unexpected empty task ID in status")
					}

					if len(r.Error) > 0 {
						return false, fmt.Errorf(r.Error)
					}

					return r.Parallel == int64(1), nil
				}
			}
			return false, nil
		}

		waitCtx4, waitCtx4Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskUpdatedCond)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Interval).To(o.Equal(sc.Spec.Repairs[0].Interval))
		o.Expect(sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))

		framework.By("Verifying that updated repair task properties were synchronized")
		tasks, err = managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask = tasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Schedule.Interval).To(o.Equal(sc.Status.Repairs[0].Interval))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(sc.Status.Repairs[0].Parallel))

		// Sanity check to avoid panics in the polling func.
		o.Expect(sc.Status.ManagerID).NotTo(o.BeNil())

		framework.By("Waiting for repair to finish")
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (done bool, err error) {
			repairProgress, err := managerClient.RepairProgress(ctx, *sc.Status.ManagerID, repairTask.ID, "latest")
			if err != nil {
				return false, err
			}

			return repairProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Deleting the repair task for ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"remove","path":"/spec/repairs/0"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs).To(o.BeEmpty())

		framework.By("Waiting for ScyllaCluster to sync repair task deletion with Scylla Manager")
		repairTaskDeletedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			return len(cluster.Status.Repairs) == 0, nil
		}

		waitCtx5, waitCtx5Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx5Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskDeletedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that repair task deletion was synchronized")
		tasks, err = managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.BeEmpty())
	})
})
