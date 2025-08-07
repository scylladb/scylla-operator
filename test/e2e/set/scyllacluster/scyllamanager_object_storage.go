// Copyright (C) 2024 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("Scylla Manager integration", framework.RequiresObjectStorage, func() {
	f := framework.NewFramework("scyllacluster")

	type entry struct {
		scyllaRepository           string
		scyllaVersion              string
		preTargetClusterCreateHook func(cluster *scyllav1.ScyllaCluster)
		postSchemaRestoreHook      func(context.Context, *framework.Framework, *scyllav1.ScyllaCluster)
	}

	g.DescribeTable("should register cluster, sync backup tasks and support manual restore procedure", func(ctx g.SpecContext, e entry) {
		sourceSC := f.GetDefaultScyllaCluster()
		sourceSC.Spec.Datacenter.Racks[0].Members = 1

		if len(e.scyllaRepository) != 0 {
			sourceSC.Spec.Repository = e.scyllaRepository
		}
		if len(e.scyllaVersion) != 0 {
			sourceSC.Spec.Version = e.scyllaVersion
		}

		objectStorageSettings, ok := f.GetClusterObjectStorageSettings()
		o.Expect(ok).To(o.BeTrue(), "cluster object storage settings must be configured for this test")

		setUpObjectStorageCredentials(ctx, f.Namespace(), f.Client, sourceSC, objectStorageSettings)

		objectStorageLocation := utils.LocationForScyllaManager(objectStorageSettings)

		framework.By("Creating source ScyllaCluster")
		sourceSC, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sourceSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for source ScyllaCluster to roll out (RV=%s)", sourceSC.ResourceVersion)
		sourceScyllaClusterRolloutCtx, sourceScyllaClusterRolloutCtxCancel := utils.ContextForRollout(ctx, sourceSC)
		defer sourceScyllaClusterRolloutCtxCancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(sourceScyllaClusterRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sourceSC)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sourceSC)

		sourceHosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sourceSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceHosts).To(o.HaveLen(int(utils.GetMemberCount(sourceSC))))
		di := verification.InsertAndVerifyCQLData(ctx, sourceHosts)
		defer di.Close()

		framework.By("Scheduling a backup for source ScyllaCluster")
		sourceSCCopy := sourceSC.DeepCopy()
		sourceSCCopy.Spec.Backups = append(sourceSCCopy.Spec.Backups, scyllav1.BackupTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
					NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
					RetryWait: &metav1.Duration{
						Duration: utils.ScyllaDBManagerTaskRetryWait,
					},
				},
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
						return false, errors.New(*b.Error)
					}

					return true, nil
				}
			}

			return false, nil
		}

		taskSchedulingCtx, taskSchedulingCtxCancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer taskSchedulingCtxCancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(taskSchedulingCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskScheduledCond)
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
						return false, errors.New("got unexpected empty task ID in status")
					}

					if b.Error != nil {
						return false, errors.New(*b.Error)
					}

					return b.Retention != nil && *b.Retention == int64(1), nil
				}
			}

			return false, nil
		}

		taskUpdateCtx, taskUpdateCtxCancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer taskUpdateCtxCancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(taskUpdateCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskUpdatedCond)
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

		// Sanity check to avoid panics.
		o.Expect(sourceSC.Status.ManagerID).NotTo(o.BeNil())
		sourceManagerClusterID := *sourceSC.Status.ManagerID
		o.Expect(sourceManagerClusterID).NotTo(o.BeEmpty())

		framework.By("Waiting for the backup task to finish")
		o.Eventually(verification.VerifyScyllaDBManagerBackupTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, sourceManagerClusterID, backupTask.ID).
			Should(o.Succeed())

		backupProgress, err := managerClient.BackupProgress(ctx, sourceManagerClusterID, backupTask.ID, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		snapshotTag := backupProgress.Progress.SnapshotTag
		o.Expect(snapshotTag).NotTo(o.BeEmpty())

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

		taskDeletionCtx, taskDeletionCtxCancel := utils.ContextForManagerSync(ctx, sourceSC)
		defer taskDeletionCtxCancel()
		sourceSC, err = controllerhelpers.WaitForScyllaClusterState(taskDeletionCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, backupTaskDeletedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that backup task deletion was synchronized")
		// Progressing conditions of the underlying ScyllaDBManagerTasks are not propagated to ScyllaClusters by the migration controller,
		// neither does the controller await the deletion of ScyllaDBManagerTasks.
		// Task deletion from ScyllaDB Manager state is asynchronous to ScyllaCluster state update.
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			tasks, err = managerClient.ListTasks(ctx, sourceManagerClusterID, "backup", false, "", "")
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(tasks.TaskListItemSlice).To(o.BeEmpty())
		}).
			WithContext(ctx).
			WithTimeout(utils.SyncTimeout).
			WithPolling(5 * time.Second).
			Should(o.Succeed())

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

		framework.By("Waiting for source ScyllaCluster to be deleted")
		sourceScyllaClusterDeletionCtx, sourceScyllaClusterDeletionCtxCancel := context.WithCancel(ctx)
		defer sourceScyllaClusterDeletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			sourceScyllaClusterDeletionCtx,
			f.DynamicClient(),
			scyllav1.GroupVersion.WithResource("scyllaclusters"),
			sourceSC.Namespace,
			sourceSC.Name,
			&sourceSC.UID,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		targetSC := f.GetDefaultScyllaCluster()
		targetSC.Spec.Datacenter.Racks[0].Members = sourceSC.Spec.Datacenter.Racks[0].Members
		targetSC.Spec.Repository = sourceSC.Spec.Repository
		targetSC.Spec.Version = sourceSC.Spec.Version

		if e.preTargetClusterCreateHook != nil {
			e.preTargetClusterCreateHook(targetSC)
		}

		setUpObjectStorageCredentials(ctx, f.Namespace(), f.Client, targetSC, objectStorageSettings)

		framework.By("Creating target ScyllaCluster")
		targetSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, targetSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for target ScyllaCluster to roll out (RV=%s)", targetSC.ResourceVersion)
		targetScyllaClusterRolloutCtx, targetScyllaClusterRolloutCtxCancel := utils.ContextForRollout(ctx, targetSC)
		defer targetScyllaClusterRolloutCtxCancel()
		targetSC, err = controllerhelpers.WaitForScyllaClusterState(targetScyllaClusterRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), targetSC)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), targetSC)

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
		var gocqlErr gocql.RequestError
		o.Expect(errors.As(err, &gocqlErr)).To(o.BeTrue())
		o.Expect(gocqlErr.Code()).To(o.Equal(gocql.ErrCodeInvalid))
		o.Expect(gocqlErr.Error()).To(o.And(o.HavePrefix("Keyspace"), o.HaveSuffix("does not exist")))

		// Close the existing session to avoid polluting the logs.
		di.Close()

		framework.By("Waiting for ScyllaDB Manager cluster ID to propagate to target ScyllaCluster's status")
		targetRegistrationCtx, targetRegistrationCtxCancel := utils.ContextForManagerSync(ctx, targetSC)
		defer targetRegistrationCtxCancel()
		targetSC, err = controllerhelpers.WaitForScyllaClusterState(targetRegistrationCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRegisteredWithManager)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Assert presence of target SC's manager ID before using it to register a restore task.
		o.Expect(targetSC.Status.ManagerID).NotTo(o.BeNil())
		targetManagerClusterID := *targetSC.Status.ManagerID
		o.Expect(targetManagerClusterID).NotTo(o.BeEmpty())

		scyllaManagerPods, err := f.KubeAdminClient().CoreV1().Pods(naming.ScyllaManagerNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: naming.ManagerSelector().String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scyllaManagerPods.Items).NotTo(o.BeEmpty())

		scyllaManagerPod := scyllaManagerPods.Items[0]

		framework.By("Creating a schema restore task")
		stdout, stderr, err := utils.ExecWithOptions(ctx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", objectStorageLocation),
				fmt.Sprintf("--snapshot-tag=%s", snapshotTag),
				"--restore-schema",
				fmt.Sprintf("--num-retries=%d", utils.ScyllaDBManagerTaskNumRetries),
				fmt.Sprintf("--retry-wait=%s", utils.ScyllaDBManagerTaskRetryWait),
			},
			Namespace:     scyllaManagerPod.Namespace,
			PodName:       scyllaManagerPod.Name,
			ContainerName: "scylla-manager",
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		_, schemaRestoreTaskID, err := managerClient.TaskSplit(ctx, targetManagerClusterID, strings.TrimSpace(stdout))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for schema restore to finish")
		o.Eventually(verification.VerifyScyllaDBManagerRestoreTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, schemaRestoreTaskID.String()).
			Should(o.Succeed())

		if e.postSchemaRestoreHook != nil {
			e.postSchemaRestoreHook(ctx, f, targetSC)
		}

		framework.By("Creating a tables restore task")
		stdout, stderr, err = utils.ExecWithOptions(ctx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", objectStorageLocation),
				fmt.Sprintf("--snapshot-tag=%s", snapshotTag),
				"--restore-tables",
				fmt.Sprintf("--num-retries=%d", utils.ScyllaDBManagerTaskNumRetries),
				fmt.Sprintf("--retry-wait=%s", utils.ScyllaDBManagerTaskRetryWait),
			},
			Namespace:     scyllaManagerPod.Namespace,
			PodName:       scyllaManagerPod.Name,
			ContainerName: "scylla-manager",
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		_, tablesRestoreTaskID, err := managerClient.TaskSplit(ctx, targetManagerClusterID, strings.TrimSpace(stdout))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for tables restore to finish")
		o.Eventually(verification.VerifyScyllaDBManagerRestoreTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, tablesRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Validating that the data restored from source cluster backup is available in target cluster")
		targetHosts, err = utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetHosts).To(o.HaveLen(int(utils.GetMemberCount(targetSC))))
		err = di.SetClientEndpoints(targetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())

		verification.VerifyCQLData(ctx, di)
	},
		g.Entry("using default ScyllaDB version", entry{}),
		// Restoring schema with ScyllaDB OS 5.4.X or ScyllaDB Enterprise 2024.1.X and consistent_cluster_management isn’t supported.
		// This test validates a workaround explained in the docs - https://operator.docs.scylladb.com/stable/nodeoperations/restore.html
		g.Entry("using workaround for consistent_cluster_management for ScyllaDB Enterprise", entry{
			scyllaRepository: configassets.ScyllaDBEnterpriseImageRepository,
			scyllaVersion:    configassets.Project.Operator.ScyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride,
			preTargetClusterCreateHook: func(targetCluster *scyllav1.ScyllaCluster) {
				targetCluster.Spec.ScyllaArgs = "--consistent-cluster-management=false"
			},
			postSchemaRestoreHook: func(ctx context.Context, f *framework.Framework, targetSC *scyllav1.ScyllaCluster) {
				var err error

				framework.By("Initiating a rolling restart of the target ScyllaCluster")
				targetSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
					ctx,
					targetSC.Name,
					types.MergePatchType,
					[]byte(`{"spec": {"forceRedeploymentReason": "schema restored"}}`),
					metav1.PatchOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Waiting for the target ScyllaCluster to roll out")
				postSchemaRestoreTargetRolloutCtx, postSchemaRestoreTargetRolloutCtxCancel := utils.ContextForRollout(ctx, targetSC)
				defer postSchemaRestoreTargetRolloutCtxCancel()
				targetSC, err = controllerhelpers.WaitForScyllaClusterState(postSchemaRestoreTargetRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), targetSC)
				scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), targetSC)

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
				waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, targetSC)
				defer waitCtxCancel()
				targetSC, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), targetSC)
				scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), targetSC)
			},
		}),
	)

	g.It("should discover cluster and sync errors for invalid tasks and invalid updates to existing tasks", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		objectStorageSettings, ok := f.GetClusterObjectStorageSettings()
		o.Expect(ok).To(o.BeTrue(), "cluster object storage settings must be configured for this test")

		setUpObjectStorageCredentials(ctx, f.Namespace(), f.Client, sc, objectStorageSettings)

		validObjectStorageLocation := utils.LocationForScyllaManager(objectStorageSettings)

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer rolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(rolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		// We get the ScyllaDB Manager cluster ID from the ScyllaCluster's status explicitly as the tasks may report errors without the cluster being registered with ScyllaDB Manager first.
		framework.By("Waiting for ScyllaDB Manager cluster ID to propagate to ScyllaCluster's status")
		registrationCtx, registrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer registrationCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(registrationCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRegisteredWithManager)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(sc.Status.ManagerID).NotTo(o.BeNil())
		managerClusterID := *sc.Status.ManagerID
		o.Expect(managerClusterID).NotTo(o.BeEmpty())

		framework.By("Scheduling invalid tasks for ScyllaCluster")
		scCopy := sc.DeepCopy()
		scCopy.Spec.Backups = append(scCopy.Spec.Backups, scyllav1.BackupTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				Name: "backup",
			},
			Location: []string{"invalid:invalid"},
		})
		scCopy.Spec.Repairs = append(scCopy.Spec.Repairs, scyllav1.RepairTaskSpec{
			TaskSpec: scyllav1.TaskSpec{
				Name: "repair",
			},
			Host: pointer.Ptr("invalid"),
		})

		patchData, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sc.Spec.Backups[0].Name).To(o.Equal("backup"))
		o.Expect(sc.Spec.Backups[0].Location).To(o.Equal([]string{"invalid:invalid"}))
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("repair"))
		o.Expect(sc.Spec.Repairs[0].Host).To(o.Equal(pointer.Ptr("invalid")))

		framework.By("Waiting for ScyllaCluster to sync task errors with Scylla Manager")
		backupTaskFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sc.Spec.Backups[0].Name {
					return b.Error != nil && len(*b.Error) != 0, nil
				}
			}

			return false, nil
		}
		repairTaskFailedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					return r.Error != nil && len(*r.Error) != 0, nil
				}
			}

			return false, nil
		}

		taskErrorSyncCtx, taskErrorSyncCtxCancel := utils.ContextForManagerSync(ctx, sc)
		defer taskErrorSyncCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(
			taskErrorSyncCtx,
			f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace),
			sc.Name,
			controllerhelpers.WaitForStateOptions{},
			repairTaskFailedCond,
			backupTaskFailedCond,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that only task names and errors were propagated to status")
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Error).NotTo(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Host).To(o.BeNil())
		o.Expect(sc.Status.Backups).To(o.HaveLen(1))
		o.Expect(sc.Status.Backups[0].Name).To(o.Equal(sc.Spec.Backups[0].Name))
		o.Expect(sc.Status.Backups[0].Error).NotTo(o.BeNil())
		o.Expect(sc.Status.Backups[0].Location).To(o.BeNil())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that tasks are not in manager state")
		repairTasks, err := managerClient.ListTasks(ctx, managerClusterID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(repairTasks.TaskListItemSlice).To(o.BeEmpty())
		backupTasks, err := managerClient.ListTasks(ctx, managerClusterID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupTasks.TaskListItemSlice).To(o.BeEmpty())

		framework.By("Scheduling valid backup and repair tasks")
		scCopy = sc.DeepCopy()
		scCopy.Spec.Backups = []scyllav1.BackupTaskSpec{
			{
				TaskSpec: scyllav1.TaskSpec{
					SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
						NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
						RetryWait: &metav1.Duration{
							Duration: utils.ScyllaDBManagerTaskRetryWait,
						},
					},
					Name: "backup",
				},
				Location: []string{validObjectStorageLocation},
			},
		}
		scCopy.Spec.Repairs = []scyllav1.RepairTaskSpec{
			{
				TaskSpec: scyllav1.TaskSpec{
					SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
						NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
						RetryWait: &metav1.Duration{
							Duration: utils.ScyllaDBManagerTaskRetryWait,
						},
					},
					Name: "repair",
				},
			},
		}

		patchData, err = controllerhelpers.GenerateMergePatch(sc, scCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, scCopy.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sc.Spec.Backups[0].Name).To(o.Equal("backup"))
		o.Expect(sc.Spec.Backups[0].Location).To(o.Equal([]string{validObjectStorageLocation}))
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("repair"))
		o.Expect(sc.Spec.Repairs[0].Host).To(o.BeNil())

		framework.By("Waiting for ScyllaCluster to sync valid tasks with Scylla Manager")
		backupTaskScheduledCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, b := range cluster.Status.Backups {
				if b.Name == sc.Spec.Backups[0].Name {
					if b.ID == nil || len(*b.ID) == 0 {
						return false, nil
					}

					if b.Error != nil {
						return false, nil
					}

					return true, nil
				}
			}
			return false, nil
		}
		repairTaskScheduledCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			for _, r := range cluster.Status.Repairs {
				if r.Name == sc.Spec.Repairs[0].Name {
					if r.ID == nil || len(*r.ID) == 0 {
						return false, nil
					}

					if r.Error != nil {
						return false, nil
					}

					return true, nil
				}
			}
			return false, nil
		}

		taskSchedulingCtx, taskSchedulingCtxCancel := utils.ContextForManagerSync(ctx, sc)
		defer taskSchedulingCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(
			taskSchedulingCtx,
			f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace),
			sc.Name,
			controllerhelpers.WaitForStateOptions{},
			backupTaskScheduledCond,
			repairTaskScheduledCond,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.Backups).To(o.HaveLen(1))
		o.Expect(sc.Status.Backups[0].ID).NotTo(o.BeNil())
		o.Expect(sc.Status.Backups[0].Name).To(o.Equal(sc.Spec.Backups[0].Name))
		o.Expect(sc.Status.Backups[0].Error).To(o.BeNil())
		o.Expect(sc.Status.Backups[0].Location).To(o.Equal(sc.Spec.Backups[0].Location))
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Error).To(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Host).To(o.BeNil())

		framework.By("Verifying that valid task statuses were synchronized")
		backupTasks, err = managerClient.ListTasks(ctx, managerClusterID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupTasks.TaskListItemSlice).To(o.HaveLen(1))
		backupTask := backupTasks.TaskListItemSlice[0]
		o.Expect(backupTask.Name).To(o.Equal(sc.Status.Backups[0].Name))
		o.Expect(backupTask.ID).To(o.Equal(*sc.Status.Backups[0].ID))

		repairTasks, err = managerClient.ListTasks(ctx, managerClusterID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(repairTasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask := repairTasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(*sc.Status.Repairs[0].ID))

		framework.By("Updating the tasks with invalid properties")
		scCopy = sc.DeepCopy()
		scCopy.Spec.Backups = []scyllav1.BackupTaskSpec{
			{
				TaskSpec: scyllav1.TaskSpec{
					Name: "backup",
				},
				Location: []string{"invalid:invalid"},
			},
		}
		scCopy.Spec.Repairs = []scyllav1.RepairTaskSpec{
			{
				TaskSpec: scyllav1.TaskSpec{
					Name: "repair",
				},
				Host: pointer.Ptr("invalid"),
			},
		}

		patchData, err = controllerhelpers.GenerateMergePatch(sc, scCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Backups).To(o.HaveLen(1))
		o.Expect(sc.Spec.Backups[0].Name).To(o.Equal("backup"))
		o.Expect(sc.Spec.Backups[0].Location).To(o.Equal([]string{"invalid:invalid"}))
		o.Expect(sc.Spec.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Spec.Repairs[0].Name).To(o.Equal("repair"))
		o.Expect(sc.Spec.Repairs[0].Host).To(o.Equal(pointer.Ptr("invalid")))

		framework.By("Waiting for ScyllaCluster to sync task errors with Scylla Manager")
		taskUpdateErrorSyncCtx, taskUpdateErrorSyncCtxCancel := utils.ContextForManagerSync(ctx, sc)
		defer taskUpdateErrorSyncCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(
			taskUpdateErrorSyncCtx,
			f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace),
			sc.Name,
			controllerhelpers.WaitForStateOptions{},
			backupTaskFailedCond,
			repairTaskFailedCond,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that task errors were propagated and task properties retained in status")
		o.Expect(sc.Status.Backups).To(o.HaveLen(1))
		o.Expect(sc.Status.Backups[0].Name).To(o.Equal(sc.Spec.Backups[0].Name))
		o.Expect(sc.Status.Backups[0].ID).NotTo(o.BeNil())
		o.Expect(*sc.Status.Backups[0].ID).To(o.Equal(backupTask.ID))
		o.Expect(sc.Status.Backups[0].Error).NotTo(o.BeNil())
		o.Expect(*sc.Status.Backups[0].Error).NotTo(o.BeEmpty())
		o.Expect(sc.Status.Backups[0].Location).To(o.Equal([]string{validObjectStorageLocation}))
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].ID).To(o.Equal(repairTask.ID))
		o.Expect(sc.Status.Repairs[0].Error).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Error).NotTo(o.BeEmpty())
		o.Expect(sc.Status.Repairs[0].Host).To(o.BeNil())

		framework.By("Verifying that tasks in manager state weren't modified")
		previousBackupTask := backupTask
		backupTasks, err = managerClient.ListTasks(ctx, managerClusterID, "backup", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupTasks.TaskListItemSlice).To(o.HaveLen(1))
		backupTask = backupTasks.TaskListItemSlice[0]
		o.Expect(backupTask.Name).To(o.Equal(sc.Status.Backups[0].Name))
		o.Expect(backupTask.ID).To(o.Equal(*sc.Status.Backups[0].ID))
		o.Expect(backupTask.Properties).To(o.Equal(previousBackupTask.Properties))

		previousRepairTask := repairTask
		repairTasks, err = managerClient.ListTasks(ctx, managerClusterID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(repairTasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask = repairTasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(*sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Properties).To(o.Equal(previousRepairTask.Properties))
	})
})

func setUpObjectStorageCredentials(ctx context.Context, ns string, nsClient framework.Client, sc *scyllav1.ScyllaCluster, objectStorageSettings framework.ClusterObjectStorageSettings) {
	g.GinkgoHelper()

	o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
	switch objectStorageSettings.Type() {
	case framework.ObjectStorageTypeGCS:
		gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
		o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

		setUpGCSCredentials(ctx, nsClient.KubeClient().CoreV1(), sc, ns, gcServiceAccountKey)

	case framework.ObjectStorageTypeS3:
		s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
		o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

		setUpS3Credentials(ctx, nsClient.KubeClient().CoreV1(), sc, ns, s3CredentialsFile)

	}
}

func setUpGCSCredentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, namespace string, serviceAccountKey []byte) {
	g.GinkgoHelper()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gcs-service-account-key-",
		},
		Data: map[string][]byte{
			"gcs-service-account.json": serviceAccountKey,
		},
	}

	secret, err := coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for i := range sc.Spec.Datacenter.Racks {
		sc.Spec.Datacenter.Racks[i].Volumes = append(sc.Spec.Datacenter.Racks[i].Volumes, corev1.Volume{
			Name: "gcs-service-account",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  "gcs-service-account.json",
							Path: "gcs-service-account.json",
						},
					},
				},
			},
		})
		sc.Spec.Datacenter.Racks[i].AgentVolumeMounts = append(sc.Spec.Datacenter.Racks[i].AgentVolumeMounts, corev1.VolumeMount{
			Name:      "gcs-service-account",
			ReadOnly:  true,
			MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
			SubPath:   "gcs-service-account.json",
		})
	}
}

func setUpS3Credentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, namespace string, s3CredentialsFile []byte) {
	g.GinkgoHelper()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "s3-credentials-file-",
		},
		Data: map[string][]byte{
			"credentials": s3CredentialsFile,
		},
	}

	secret, err := coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for i := range sc.Spec.Datacenter.Racks {
		sc.Spec.Datacenter.Racks[i].Volumes = append(sc.Spec.Datacenter.Racks[i].Volumes, corev1.Volume{
			Name: "aws-credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  "credentials",
							Path: "credentials",
						},
					},
				},
			},
		})
		sc.Spec.Datacenter.Racks[i].AgentVolumeMounts = append(sc.Spec.Datacenter.Racks[i].AgentVolumeMounts, corev1.VolumeMount{
			Name:      "aws-credentials",
			ReadOnly:  true,
			MountPath: "/var/lib/scylla-manager/.aws/credentials",
			SubPath:   "credentials",
		})
	}
}
