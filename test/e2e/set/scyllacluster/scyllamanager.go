// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("Scylla Manager integration", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should register cluster and sync repair tasks", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
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

		framework.By("Scheduling a repair task")
		scCopy := sc.DeepCopy()
		scCopy.Spec.Repairs = append(scCopy.Spec.Repairs, scyllav1.RepairTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				Name: "repair",
			},
			Parallel: 2,
		})

		patchData, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("repair"))
		o.Expect(sc.Spec.Repairs[0].Parallel).To(o.Equal(int64(2)))
		o.Expect(sc.Spec.Backups).To(o.BeEmpty())

		framework.By("Waiting for ScyllaCluster to sync repair tasks with Scylla Manager")
		repairTaskScheduledCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					if r.ID == nil || len(*r.ID) == 0 {
						return false, nil
					}

					if r.Error != nil {
						return false, fmt.Errorf(*r.Error)
					}

					return true, nil
				}
			}
			return false, nil
		}

		waitCtx3, waitCtx3Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskScheduledCond)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Parallel).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))
		o.Expect(sc.Status.Backups).To(o.BeEmpty())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that repair task status was synchronized")
		tasks, err := managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask := tasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(*sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(*sc.Status.Repairs[0].Parallel))

		framework.By("Updating the repair task")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/repairs/0/parallel","value":1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("repair"))
		o.Expect(sc.Spec.Repairs[0].Parallel).To(o.Equal(int64(1)))
		o.Expect(sc.Spec.Backups).To(o.BeEmpty())

		framework.By("Waiting for ScyllaCluster to sync repair task update with Scylla Manager")
		repairTaskUpdatedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					if r.ID == nil || len(*r.ID) == 0 {
						return false, fmt.Errorf("got unexpected empty task ID in status")
					}

					if r.Error != nil {
						return false, fmt.Errorf(*r.Error)
					}

					return r.Parallel != nil && *r.Parallel == int64(1), nil
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
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].ID).To(o.Equal(repairTask.ID))
		o.Expect(sc.Status.Repairs[0].Parallel).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))
		o.Expect(sc.Status.Backups).To(o.BeEmpty())

		framework.By("Verifying that updated repair task status was synchronized")
		tasks, err = managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask = tasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(*sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(*sc.Status.Repairs[0].Parallel))

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

		framework.By("Updating the repair task with invalid properties")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/repairs/0/host","value":"invalid"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("repair"))
		o.Expect(sc.Spec.Repairs[0].Host).NotTo(o.BeNil())
		o.Expect(*sc.Spec.Repairs[0].Host).To(o.Equal("invalid"))
		o.Expect(sc.Spec.Repairs[0].Parallel).To(o.Equal(int64(1)))
		o.Expect(sc.Spec.Backups).To(o.BeEmpty())

		framework.By("Waiting for ScyllaCluster to sync repair task error with Scylla Manager")
		repairTaskFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					return r.Error != nil && len(*r.Error) != 0, nil
				}
			}
			return false, nil
		}

		waitCtx5, waitCtx5Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx5Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskFailedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that repair task error was propagated and properties retained in status")
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].ID).To(o.Equal(repairTask.ID))
		o.Expect(sc.Status.Repairs[0].Parallel).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))
		o.Expect(sc.Status.Repairs[0].Error).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Error).NotTo(o.BeEmpty())
		o.Expect(sc.Status.Backups).To(o.BeEmpty())

		previousRepairTask := repairTask
		framework.By("Verifying that repair task in manager state wasn't modified")
		tasks, err = managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask = tasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(*sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Properties).To(o.Equal(previousRepairTask.Properties))

		framework.By("Deleting the repair task")
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

		waitCtx6, waitCtx6Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx6Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx6, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskDeletedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that repair task deletion was synchronized")
		tasks, err = managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.BeEmpty())
	})

	provideGCSCredentials := func(ctx context.Context, sc *scyllav1.ScyllaCluster) *scyllav1.ScyllaCluster {
		scCopy := sc.DeepCopy()

		gcServiceAccountKey := f.GetGCSServiceAccountKey()
		o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "gcs-service-account-key-",
			},
			Data: map[string][]byte{
				"gcs-service-account.json": gcServiceAccountKey,
			},
		}

		secret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Create(ctx, secret, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for i := range scCopy.Spec.Datacenter.Racks {
			scCopy.Spec.Datacenter.Racks[i].Volumes = append(scCopy.Spec.Datacenter.Racks[i].Volumes, corev1.Volume{
				Name: "gcs-service-account",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			})
			scCopy.Spec.Datacenter.Racks[i].AgentVolumeMounts = append(scCopy.Spec.Datacenter.Racks[i].AgentVolumeMounts, corev1.VolumeMount{
				Name:      "gcs-service-account",
				ReadOnly:  true,
				MountPath: "/etc/scylla-manager-agent",
			})
		}

		return scCopy
	}

	g.It("should register cluster, sync backup tasks and support manual restore procedure", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sourceSC := f.GetDefaultScyllaCluster()
		sourceSC.Spec.Datacenter.Racks[0].Members = 1

		objectStorageType := f.GetObjectStorageType()
		switch objectStorageType {
		case framework.ObjectStorageTypeGCS:
			sourceSC = provideGCSCredentials(ctx, sourceSC)
		default:
			g.Fail("unsupported object storage type")
		}

		objectStorageLocation := fmt.Sprintf("%s:%s", f.GetObjectStorageProvider(), f.GetObjectStorageBucket())

		framework.By("Creating source ScyllaCluster")
		sourceSC, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sourceSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for source ScyllaCluster to roll out (RV=%s)", sourceSC.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sourceSC)
		defer waitCtx1Cancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sourceSC)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sourceSC)

		sourceHosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sourceSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceHosts).To(o.HaveLen(int(utils.GetMemberCount(sourceSC))))
		di := insertAndVerifyCQLData(ctx, sourceHosts)
		defer di.Close()

		framework.By("Waiting for source ScyllaCluster to register with Scylla Manager")
		registeredInManagerCond := func(sc *scyllav1.ScyllaCluster) (bool, error) {
			return sc.Status.ManagerID != nil, nil
		}

		waitCtx2, waitCtx2Cancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer waitCtx2Cancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, registeredInManagerCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scheduling a backup for source ScyllaCluster")
		sourceSCCopy := sourceSC.DeepCopy()
		sourceSCCopy.Spec.Backups = append(sourceSCCopy.Spec.Backups, scyllav1.BackupTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				Name: "backup",
			},
			Location:  []string{objectStorageLocation},
			Retention: 2,
		})

		patchData, err := controllerhelpers.GenerateMergePatch(sourceSC, sourceSCCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sourceSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sourceSC.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceSC.Spec.Repairs).To(o.BeEmpty())
		o.Expect(sourceSC.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sourceSC.Spec.Backups[0].Name).To(o.Equal("backup"))
		o.Expect(sourceSC.Spec.Backups[0].Location).To(o.Equal([]string{objectStorageLocation}))
		o.Expect(sourceSC.Spec.Backups[0].Retention).To(o.Equal(int64(2)))

		framework.By("Waiting for source ScyllaCluster to sync backups with Scylla Manager")
		backupTaskScheduledCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sourceSC.Spec.Backups[0].Name {
					if b.ID == nil || len(*b.ID) == 0 {
						return false, nil
					}

					if b.Error != nil {
						return false, fmt.Errorf(*b.Error)
					}

					return true, nil
				}
			}

			return false, nil
		}

		waitCtx3, waitCtx3Cancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer waitCtx3Cancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskScheduledCond)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceSC.Status.Repairs).To(o.BeEmpty())
		o.Expect(sourceSC.Status.Backups).To(o.HaveLen(1))
		o.Expect(sourceSC.Status.Backups[0].Name).To(o.Equal(sourceSC.Spec.Backups[0].Name))
		o.Expect(sourceSC.Status.Backups[0].Location).To(o.Equal(sourceSC.Spec.Backups[0].Location))
		o.Expect(sourceSC.Status.Backups[0].Retention).NotTo(o.BeNil())
		o.Expect(*sourceSC.Status.Backups[0].Retention).To(o.Equal(sourceSC.Spec.Backups[0].Retention))

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that backup task properties were synchronized")
		tasks, err := managerClient.ListTasks(ctx, *sourceSC.Status.ManagerID, managerclient.BackupTask, false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		backupTask := tasks.TaskListItemSlice[0]
		o.Expect(backupTask.Name).To(o.Equal(sourceSC.Status.Backups[0].Name))
		o.Expect(backupTask.ID).To(o.Equal(*sourceSC.Status.Backups[0].ID))
		o.Expect(backupTask.Properties.(map[string]interface{})["location"]).To(o.ConsistOf(sourceSC.Status.Backups[0].Location))
		o.Expect(backupTask.Properties.(map[string]interface{})["retention"].(json.Number).Int64()).To(o.Equal(*sourceSC.Status.Backups[0].Retention))

		framework.By("Updating the backup task for ScyllaCluster")
		sourceSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sourceSC.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/backups/0/retention","value":1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceSC.Spec.Backups[0].Retention).To(o.Equal(int64(1)))

		framework.By("Waiting for source ScyllaCluster to sync backup task update with Scylla Manager")
		backupTaskUpdatedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sourceSC.Spec.Backups[0].Name {
					if b.ID == nil || len(*b.ID) == 0 {
						return false, fmt.Errorf("got unexpected empty task ID in status")
					}

					if b.Error != nil {
						return false, fmt.Errorf(*b.Error)
					}

					return b.Retention != nil && *b.Retention == int64(1), nil
				}
			}

			return false, nil
		}

		waitCtx4, waitCtx4Cancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer waitCtx4Cancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskUpdatedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(sourceSC.Status.Backups).To(o.HaveLen(1))
		o.Expect(sourceSC.Status.Backups[0].Name).To(o.Equal(sourceSC.Spec.Backups[0].Name))
		o.Expect(sourceSC.Status.Backups[0].Location).To(o.Equal(sourceSC.Spec.Backups[0].Location))
		o.Expect(sourceSC.Status.Backups[0].Retention).NotTo(o.BeNil())
		o.Expect(*sourceSC.Status.Backups[0].Retention).To(o.Equal(sourceSC.Spec.Backups[0].Retention))
		o.Expect(sourceSC.Status.Repairs).To(o.BeEmpty())

		framework.By("Verifying that updated backups task properties were synchronized")
		tasks, err = managerClient.ListTasks(ctx, *sourceSC.Status.ManagerID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		backupTask = tasks.TaskListItemSlice[0]
		o.Expect(backupTask.Name).To(o.Equal(sourceSC.Status.Backups[0].Name))
		o.Expect(backupTask.ID).To(o.Equal(*sourceSC.Status.Backups[0].ID))
		o.Expect(backupTask.Properties.(map[string]interface{})["location"]).To(o.ConsistOf(sourceSC.Status.Backups[0].Location))
		o.Expect(backupTask.Properties.(map[string]interface{})["retention"].(json.Number).Int64()).To(o.Equal(*sourceSC.Status.Backups[0].Retention))

		// Sanity check to avoid panics in the polling func.
		o.Expect(sourceSC.Status.ManagerID).NotTo(o.BeNil())

		var backupProgress managerclient.BackupProgress
		framework.By("Waiting for backup to finish")
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (done bool, err error) {
			backupProgress, err = managerClient.BackupProgress(ctx, *sourceSC.Status.ManagerID, backupTask.ID, "latest")
			if err != nil {
				return false, err
			}

			return backupProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupProgress.Progress.SnapshotTag).NotTo(o.BeEmpty())

		framework.By("Updating the backup task with invalid properties")
		sourceSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sourceSC.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/backups/0/location/0","value":"invalid:invalid"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceSC.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sourceSC.Spec.Backups[0].Name).To(o.Equal("backup"))
		o.Expect(sourceSC.Spec.Backups[0].Location).To(o.Equal([]string{"invalid:invalid"}))
		o.Expect(sourceSC.Spec.Backups[0].Retention).To(o.Equal(int64(1)))
		o.Expect(sourceSC.Spec.Repairs).To(o.BeEmpty())

		framework.By("Waiting for ScyllaCluster to sync backup task error with Scylla Manager")
		backupTaskFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sourceSC.Spec.Backups[0].Name {
					return b.Error != nil && len(*b.Error) != 0, nil
				}
			}
			return false, nil
		}

		waitCtx5, waitCtx5Cancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer waitCtx5Cancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskFailedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that backup task error was propagated and properties retained in status")
		o.Expect(sourceSC.Status.Backups).To(o.HaveLen(1))
		o.Expect(sourceSC.Status.Backups[0].Name).To(o.Equal(sourceSC.Spec.Backups[0].Name))
		o.Expect(sourceSC.Status.Backups[0].ID).NotTo(o.BeNil())
		o.Expect(*sourceSC.Status.Backups[0].ID).To(o.Equal(backupTask.ID))
		o.Expect(sourceSC.Status.Backups[0].Location).To(o.Equal([]string{objectStorageLocation}))
		o.Expect(sourceSC.Status.Backups[0].Retention).NotTo(o.BeNil())
		o.Expect(*sourceSC.Status.Backups[0].Retention).To(o.Equal(int64(1)))
		o.Expect(sourceSC.Status.Backups[0].Error).NotTo(o.BeNil())
		o.Expect(*sourceSC.Status.Backups[0].Error).NotTo(o.BeEmpty())
		o.Expect(sourceSC.Status.Repairs).To(o.BeEmpty())

		previousBackupTask := backupTask
		framework.By("Verifying that backup task in manager state wasn't modified")
		tasks, err = managerClient.ListTasks(ctx, *sourceSC.Status.ManagerID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		backupTask = tasks.TaskListItemSlice[0]
		o.Expect(backupTask.Name).To(o.Equal(sourceSC.Status.Backups[0].Name))
		o.Expect(backupTask.ID).To(o.Equal(*sourceSC.Status.Backups[0].ID))
		o.Expect(backupTask.Properties).To(o.Equal(previousBackupTask.Properties))

		framework.By("Deleting the backup task for source ScyllaCluster")
		sourceSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sourceSC.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"remove","path":"/spec/backups/0"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceSC.Spec.Backups).To(o.BeEmpty())

		framework.By("Waiting for source ScyllaCluster to sync backup task deletion with Scylla Manager")
		backupTaskDeletedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			return len(cluster.Status.Backups) == 0, nil
		}

		waitCtx6, waitCtx6Cancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer waitCtx6Cancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx6, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskDeletedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that backup task deletion was synchronized")
		tasks, err = managerClient.ListTasks(ctx, *sourceSC.Status.ManagerID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.BeEmpty())

		// Close the existing session to avoid polluting the logs.
		di.Close()

		framework.By("Deleting source ScyllaCluster")
		var propagationPolicy = metav1.DeletePropagationForeground
		err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace).Delete(ctx, sourceSC.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sourceSC.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx7, waitCtx7Cancel := context.WithCancel(ctx)
		defer waitCtx7Cancel()
		err = framework.WaitForObjectDeletion(
			waitCtx7,
			f.DynamicClient(),
			scyllav1.GroupVersion.WithResource("scyllaclusters"),
			sourceSC.Namespace,
			sourceSC.Name,
			&sourceSC.UID,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		targetSC := f.GetDefaultScyllaCluster()
		targetSC.Spec.Datacenter.Racks[0].Members = sourceSC.Spec.Datacenter.Racks[0].Members
		targetSC.Spec.ScyllaArgs = "--consistent-cluster-management=false"

		switch objectStorageType {
		case framework.ObjectStorageTypeGCS:
			targetSC = provideGCSCredentials(ctx, targetSC)
		default:
			g.Fail("unsupported object storage type")
		}

		framework.By("Creating target ScyllaCluster")
		targetSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, targetSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for target ScyllaCluster to roll out (RV=%s)", targetSC.ResourceVersion)
		waitCtx8, waitCtx8Cancel := utils.ContextForRollout(ctx, targetSC)
		defer waitCtx8Cancel()
		targetSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx8, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), targetSC)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), targetSC)

		framework.By("Verifying that target cluster is missing the source cluster data")
		targetHosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetHosts).To(o.HaveLen(int(utils.GetMemberCount(targetSC))))
		err = di.SetClientEndpoints(targetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = di.AwaitSchemaAgreement(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = di.Read()
		o.Expect(err).To(o.HaveOccurred())

		// Close the existing session to avoid polluting the logs.
		di.Close()

		framework.By("Waiting for source ScyllaCluster to sync with Scylla Manager")
		waitCtx9, waitCtx9Cancel := utils.ContextForManagerSync(ctx, targetSC)
		defer waitCtx9Cancel()
		targetSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx9, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, registeredInManagerCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		podList, err := f.KubeAdminClient().CoreV1().Pods(naming.ScyllaManagerNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: naming.ManagerSelector().String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podList.Items).NotTo(o.BeEmpty())

		scyllaManagerPod := podList.Items[0]

		framework.By("Creating a schema restore task")
		stdout, stderr, err := utils.ExecWithOptions(f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command:       []string{"sctool", "restore", "--cluster", *targetSC.Status.ManagerID, "--location", objectStorageLocation, "--snapshot-tag", backupProgress.Progress.SnapshotTag, "--restore-schema"},
			Namespace:     scyllaManagerPod.Namespace,
			PodName:       scyllaManagerPod.Name,
			ContainerName: "scylla-manager",
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		_, schemaRestoreUUID, err := managerClient.TaskSplit(ctx, *targetSC.Status.ManagerID, strings.TrimSpace(stdout))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for schema restore to finish")
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (done bool, err error) {
			schemaRestoreProgress, err := managerClient.RestoreProgress(ctx, *targetSC.Status.ManagerID, schemaRestoreUUID.String(), "latest")
			if err != nil {
				return false, err
			}

			return schemaRestoreProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Initiating a rolling restart of the target ScyllaCluster")
		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			targetSC.Name,
			types.MergePatchType,
			[]byte(`{"spec": {"forceRedeploymentReason": "schema restored"}}`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaCluster to roll out")
		waitCtx10, waitCtx10Cancel := utils.ContextForRollout(ctx, targetSC)
		defer waitCtx10Cancel()
		targetSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx10, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), targetSC)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), targetSC)

		framework.By("Enabling raft in target cluster")
		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			targetSC.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/scyllaArgs","value":"--consistent-cluster-management=true"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaCluster to roll out")
		waitCtx11, waitCtx11Cancel := utils.ContextForRollout(ctx, targetSC)
		defer waitCtx11Cancel()
		targetSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx11, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), targetSC)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), targetSC)

		framework.By("Creating a tables restore task")
		stdout, stderr, err = utils.ExecWithOptions(f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command:       []string{"sctool", "restore", "--cluster", *targetSC.Status.ManagerID, "--location", objectStorageLocation, "--snapshot-tag", backupProgress.Progress.SnapshotTag, "--restore-tables"},
			Namespace:     scyllaManagerPod.Namespace,
			PodName:       scyllaManagerPod.Name,
			ContainerName: "scylla-manager",
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		_, tablesRestoreUUID, err := managerClient.TaskSplit(ctx, *targetSC.Status.ManagerID, strings.TrimSpace(stdout))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for tables restore to finish")
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(context.Context) (done bool, err error) {
			tablesRestoreProgress, err := managerClient.RestoreProgress(ctx, *targetSC.Status.ManagerID, tablesRestoreUUID.String(), "latest")
			if err != nil {
				return false, err
			}

			return tablesRestoreProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		targetHosts, err = utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetHosts).To(o.HaveLen(int(utils.GetMemberCount(targetSC))))
		err = di.SetClientEndpoints(targetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCQLData(ctx, di)
	})

	g.It("should discover cluster and sync errors for invalid tasks", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
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

		framework.By("Scheduling tasks for ScyllaCluster")
		scCopy := sc.DeepCopy()
		scCopy.Spec.Repairs = append(scCopy.Spec.Repairs, scyllav1.RepairTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				Name: "invalid",
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					Cron: pointer.Ptr("* * * * *"),
				},
			},
			Host: pointer.Ptr("invalid"),
		})
		scCopy.Spec.Backups = append(scCopy.Spec.Backups, scyllav1.BackupTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				Name: "invalid",
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					Cron: pointer.Ptr("* * * * *"),
				},
			},
			Location: []string{"invalid:invalid"},
		})

		patchData, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("invalid"))
		o.Expect(sc.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sc.Spec.Backups[0].Name).To(o.Equal("invalid"))

		framework.By("Waiting for ScyllaCluster to sync task errors with Scylla Manager")
		repairTaskFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					return r.Error != nil && len(*r.Error) != 0, nil
				}
			}

			return false, nil
		}
		backupTaskFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sc.Spec.Backups[0].Name {
					return b.Error != nil && len(*b.Error) != 0, nil
				}
			}

			return false, nil
		}

		waitCtx3, waitCtx3Cancel := utils.ContextForManagerSync(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskFailedCond, backupTaskFailedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that only task names and errors were propagated to status")
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Error).NotTo(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Cron).To(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Host).To(o.BeNil())
		o.Expect(sc.Status.Backups).To(o.HaveLen(1))
		o.Expect(sc.Status.Backups[0].Name).To(o.Equal(sc.Spec.Backups[0].Name))
		o.Expect(sc.Status.Backups[0].Error).NotTo(o.BeNil())
		o.Expect(sc.Status.Backups[0].Cron).To(o.BeNil())
		o.Expect(sc.Status.Backups[0].Location).To(o.BeNil())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that tasks are not in manager state")
		repairTasks, err := managerClient.ListTasks(ctx, *sc.Status.ManagerID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(repairTasks.TaskListItemSlice).To(o.BeEmpty())
		backupTasks, err := managerClient.ListTasks(ctx, *sc.Status.ManagerID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupTasks.TaskListItemSlice).To(o.BeEmpty())
	})
})
