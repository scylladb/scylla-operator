// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("ScyllaDBManagerTask and ScyllaDBDatacenter integration with global ScyllaDB Manager", func() {
	f := framework.NewFramework("scylladbmanagertask")

	type entry struct {
		scyllaDBImage              string
		preTargetClusterCreateHook func(*scyllav1alpha1.ScyllaDBDatacenter)
		postSchemaRestoreHook      func(context.Context, string, framework.Client, *scyllav1alpha1.ScyllaDBDatacenter)
	}

	g.DescribeTable("should synchronise a backup task and support a manual restore procedure", func(ctx g.SpecContext, e entry) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sourceSDC := f.GetDefaultScyllaDBDatacenter()
		if len(e.scyllaDBImage) != 0 {
			sourceSDC.Spec.ScyllaDB.Image = e.scyllaDBImage
		}

		metav1.SetMetaDataLabel(&sourceSDC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		objectStorageSettings, ok := f.GetClusterObjectStorageSettings()
		o.Expect(ok).To(o.BeTrue(), "cluster object storage settings must be configured for this test")

		setUpObjectStorageCredentials(ctx, ns.Name, nsClient, sourceSDC, objectStorageSettings)

		framework.By("Creating a source ScyllaDBDatacenter with the global ScyllaDB Manager registration label")
		sourceSDC, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sourceSDC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the source ScyllaDBDatacenter to roll out (RV=%s)", sourceSDC.ResourceVersion)
		sourceSDCRolloutCtx, sourceSDCRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sourceSDC)
		defer sourceSDCRolloutCtxCancel()
		sourceSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(sourceSDCRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sourceSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sourceSDC)
		scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sourceSDC)

		sourceHosts, err := utilsv1alpha1.GetBroadcastRPCAddresses(ctx, nsClient.KubeClient().CoreV1(), sourceSDC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceHosts).To(o.HaveLen(int(utilsv1alpha1.GetNodeCount(sourceSDC))))
		di := verification.InsertAndVerifyCQLData(ctx, sourceHosts)
		defer di.Close()

		smt := &scyllav1alpha1.ScyllaDBManagerTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup",
				Namespace: ns.Name,
			},
			Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
				ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
					Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
					Name: sourceSDC.Name,
				},
				Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
				Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
					ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
						NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
						RetryWait: &metav1.Duration{
							Duration: utils.ScyllaDBManagerTaskRetryWait,
						},
					},
					Location: []string{
						utils.LocationForScyllaManager(objectStorageSettings),
					},
					Retention: pointer.Ptr[int64](2),
				},
			},
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Backup'")
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Create(
			ctx,
			smt,
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBManagerTask to register with global ScyllaDB Manager instance")
		scyllaDBManagerTaskRegistrationCtx, scyllaDBManagerTaskRegistrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer scyllaDBManagerTaskRegistrationCtxCancel()
		smt, err = controllerhelpers.WaitForScyllaDBManagerTaskState(
			scyllaDBManagerTaskRegistrationCtx,
			nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name),
			smt.Name,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerTaskRolledOut,
			utilsv1alpha1.ScyllaDBManagerTaskHasDeletionFinalizer,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(smt.Status.TaskID).NotTo(o.BeNil())
		o.Expect(*smt.Status.TaskID).NotTo(o.BeEmpty())
		managerTaskID, err := uuid.Parse(*smt.Status.TaskID)
		o.Expect(err).NotTo(o.HaveOccurred())

		sourceSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sourceSDC)
		o.Expect(err).NotTo(o.HaveOccurred())
		sourceSMCR, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns.Name).Get(ctx, sourceSMCRName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sourceSMCR.Status.ClusterID).NotTo(o.BeNil())
		o.Expect(*sourceSMCR.Status.ClusterID).NotTo(o.BeEmpty())
		sourceManagerClusterID := *sourceSMCR.Status.ClusterID

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that ScyllaDBManagerTask was registered with global ScyllaDB Manager")
		managerTask, err := managerClient.GetTask(ctx, sourceManagerClusterID, managerclient.BackupTask, managerTaskID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerTask.Labels).NotTo(o.BeNil())
		o.Expect(managerTask.Labels[naming.OwnerUIDLabel]).To(o.Equal(string(smt.UID)))

		framework.By("Verifying that ScyllaDBManagerTask properties were propagated to ScyllaDB Manager state")
		o.Expect(managerTask.Schedule).NotTo(o.BeNil())
		o.Expect(managerTask.Schedule.NumRetries).To(o.Equal(*smt.Spec.Backup.NumRetries))
		o.Expect(managerTask.Properties.(map[string]interface{})["location"]).To(o.ConsistOf(smt.Spec.Backup.Location))
		o.Expect(managerTask.Properties.(map[string]interface{})["retention"].(json.Number).Int64()).To(o.Equal(*smt.Spec.Backup.Retention))

		framework.By("Updating the ScyllaDBManagerTask")
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Patch(
			ctx,
			smt.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/backup/retention","value":1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(smt.Spec.Backup).NotTo(o.BeNil())
		o.Expect(smt.Spec.Backup.Retention).NotTo(o.BeNil())
		o.Expect(*smt.Spec.Backup.Retention).To(o.Equal(int64(1)))

		framework.By("Waiting for ScyllaDBManagerTask update to propagate")
		updateCtx, updateCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer updateCtxCancel()
		smt, err = controllerhelpers.WaitForScyllaDBManagerTaskState(
			updateCtx,
			nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name),
			smt.Name,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerTaskRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that the ScyllaDBManagerTask update propagated to ScyllaDB Manager state")
		updatePropagationCtx, updatePropagationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer updatePropagationCtxCancel()
		managerTask, err = managerClient.GetTask(updatePropagationCtx, sourceManagerClusterID, managerclient.BackupTask, managerTaskID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerTask.Properties.(map[string]interface{})["retention"].(json.Number).Int64()).To(o.Equal(*smt.Spec.Backup.Retention))

		framework.By("Waiting for the backup task to finish")
		o.Eventually(verification.VerifyScyllaDBManagerBackupTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, sourceManagerClusterID, managerTask.ID).
			Should(o.Succeed())

		backupProgress, err := managerClient.BackupProgress(ctx, sourceManagerClusterID, managerTask.ID, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		snapshotTag := backupProgress.Progress.SnapshotTag
		o.Expect(snapshotTag).NotTo(o.BeEmpty())

		framework.By("Deleting ScyllaDBManagerTask")
		err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Delete(
			ctx,
			smt.Name,
			metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
				Preconditions: &metav1.Preconditions{
					UID: &smt.UID,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By(`Waiting for ScyllaDBManagerTask to be deleted`)
		taskDeletionCtx, taskDeletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer taskDeletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			taskDeletionCtx,
			nsClient.DynamicClient(),
			scyllav1alpha1.GroupVersion.WithResource("scylladbmanagertasks"),
			smt.Namespace,
			smt.Name,
			pointer.Ptr(smt.UID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that the task has been removed from the global ScyllaDB Manager state")
		// 'GetTask' is broken and does not return an error after the task has been deleted.
		// XRef: https://github.com/scylladb/scylla-manager/issues/4400
		tasks, err := managerClient.ListTasks(ctx, sourceManagerClusterID, managerclient.BackupTask, false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(slices.ContainsFunc(tasks.TaskListItemSlice, func(t *managerclient.TaskListItem) bool {
			return t.ID == managerTaskID.String()
		})).To(o.BeFalse())

		// Close the existing session to avoid polluting the logs.
		di.Close()

		framework.By("Deleting the source ScyllaDBDatacenter")
		err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Delete(
			ctx,
			sourceSDC.Name,
			metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
				Preconditions: &metav1.Preconditions{
					UID: &sourceSDC.UID,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the source ScyllaDBDatacenter to be deleted")
		sourceSDCDeletionCtx, sourceSDCDeletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer sourceSDCDeletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			sourceSDCDeletionCtx,
			nsClient.DynamicClient(),
			scyllav1alpha1.GroupVersion.WithResource("scylladbdatacenters"),
			sourceSDC.Namespace,
			sourceSDC.Name,
			pointer.Ptr(sourceSDC.UID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		targetSDC := f.GetDefaultScyllaDBDatacenter()
		targetSDC.Spec.ScyllaDB.Image = sourceSDC.Spec.ScyllaDB.Image

		metav1.SetMetaDataLabel(&targetSDC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		if e.preTargetClusterCreateHook != nil {
			e.preTargetClusterCreateHook(targetSDC)
		}

		setUpObjectStorageCredentials(ctx, ns.Name, nsClient, targetSDC, objectStorageSettings)

		framework.By("Creating the target ScyllaDBDatacenter with the global ScyllaDB Manager registration label")
		targetSDC, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, targetSDC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaDBDatacenter to roll out (RV=%s)", targetSDC.ResourceVersion)
		targetSDCCtx, targetSDCRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, targetSDC)
		defer targetSDCRolloutCtxCancel()
		targetSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(targetSDCCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), targetSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), targetSDC)
		scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), targetSDC)

		targetHosts, err := utilsv1alpha1.GetBroadcastRPCAddresses(ctx, nsClient.KubeClient().CoreV1(), targetSDC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetHosts).To(o.HaveLen(int(utilsv1alpha1.GetNodeCount(targetSDC))))
		err = di.SetClientEndpoints(targetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = di.AwaitSchemaAgreement(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = di.Read()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err).To(o.MatchError(o.And(o.ContainSubstring("Keyspace"), o.ContainSubstring("does not exist"))))

		// Close the existing session to avoid polluting the logs.
		di.Close()

		targetSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(targetSDC)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for target ScyllaDBDatacenter to register with global ScyllaDB Manager instance")
		targetSDCRegistrationCtx, targetSDCRegistrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer targetSDCRegistrationCtxCancel()
		targetSMCR, err := controllerhelpers.WaitForScyllaDBManagerClusterRegistrationState(targetSDCRegistrationCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns.Name), targetSMCRName, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBManagerClusterRegistrationRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetSMCR.Status.ClusterID).NotTo(o.BeNil())
		o.Expect(*targetSMCR.Status.ClusterID).NotTo(o.BeEmpty())
		targetManagerClusterID := *targetSMCR.Status.ClusterID

		globalScyllaDBManagerInstancePods, err := f.KubeAdminClient().CoreV1().Pods(naming.ScyllaManagerNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: naming.ManagerSelector().String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(globalScyllaDBManagerInstancePods.Items).NotTo(o.BeEmpty())

		globalScyllaDBManagerInstancePod := globalScyllaDBManagerInstancePods.Items[0]

		framework.By("Creating a schema restore task against global ScyllaDB Manager instance")
		schemaRestoreCreationCtx, schemaRestoreCreationCtxCancel := context.WithTimeoutCause(ctx, utils.SyncTimeout, fmt.Errorf("schema restore task creation has not completed in time"))
		defer schemaRestoreCreationCtxCancel()
		stdout, stderr, err := utils.ExecWithOptions(schemaRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", utils.LocationForScyllaManager(objectStorageSettings)),
				fmt.Sprintf("--snapshot-tag=%s", snapshotTag),
				"--restore-schema",
				fmt.Sprintf("--num-retries=%d", utils.ScyllaDBManagerTaskNumRetries),
				fmt.Sprintf("--retry-wait=%s", utils.ScyllaDBManagerTaskRetryWait),
			},
			Namespace:     globalScyllaDBManagerInstancePod.Namespace,
			PodName:       globalScyllaDBManagerInstancePod.Name,
			ContainerName: "scylla-manager",
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr, context.Cause(schemaRestoreCreationCtx))

		_, schemaRestoreTaskID, err := managerClient.TaskSplit(ctx, targetManagerClusterID, strings.TrimSpace(stdout))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the schema restore task to finish")
		o.Eventually(verification.VerifyScyllaDBManagerRestoreTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, schemaRestoreTaskID.String()).
			Should(o.Succeed())

		if e.postSchemaRestoreHook != nil {
			e.postSchemaRestoreHook(ctx, ns.Name, nsClient, targetSDC)
		}

		framework.By("Creating a tables restore task against global ScyllaDB Manager instance")
		tablesRestoreCreationCtx, tablesRestoreCreationCtxCancel := context.WithTimeoutCause(ctx, utils.SyncTimeout, fmt.Errorf("tables restore task creation has not completed in time"))
		defer tablesRestoreCreationCtxCancel()
		stdout, stderr, err = utils.ExecWithOptions(tablesRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", utils.LocationForScyllaManager(objectStorageSettings)),
				fmt.Sprintf("--snapshot-tag=%s", snapshotTag),
				"--restore-tables",
				fmt.Sprintf("--num-retries=%d", utils.ScyllaDBManagerTaskNumRetries),
				fmt.Sprintf("--retry-wait=%s", utils.ScyllaDBManagerTaskRetryWait),
			},
			Namespace:     globalScyllaDBManagerInstancePod.Namespace,
			PodName:       globalScyllaDBManagerInstancePod.Name,
			ContainerName: "scylla-manager",
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr, context.Cause(tablesRestoreCreationCtx))

		_, tablesRestoreTaskID, err := managerClient.TaskSplit(ctx, targetManagerClusterID, strings.TrimSpace(stdout))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the tables restore task to finish")
		o.Eventually(verification.VerifyScyllaDBManagerRestoreTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, tablesRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Validating that the data restored from the source cluster backup is available in the target cluster")
		targetHosts, err = utilsv1alpha1.GetBroadcastRPCAddresses(ctx, nsClient.KubeClient().CoreV1(), targetSDC)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetHosts).To(o.HaveLen(int(utilsv1alpha1.GetNodeCount(targetSDC))))
		err = di.SetClientEndpoints(targetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())

		verification.VerifyCQLData(ctx, di)
	},
		g.Entry("using the default ScyllaDB image", entry{}),
		// Restoring schema with ScyllaDB OS 5.4.X or ScyllaDB Enterprise 2024.1.X and consistent_cluster_management isn’t supported.
		// This test validates a workaround explained in the docs - https://operator.docs.scylladb.com/stable/nodeoperations/restore.html
		g.Entry("using a workaround for consistent_cluster_management for ScyllaDB Enterprise image", entry{
			scyllaDBImage: fmt.Sprintf("%s:%s", configassets.ScyllaDBEnterpriseImageRepository, configassets.Project.Operator.ScyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride),
			preTargetClusterCreateHook: func(targetSDC *scyllav1alpha1.ScyllaDBDatacenter) {
				targetSDC.Spec.ScyllaDB.AdditionalScyllaDBArguments = append(targetSDC.Spec.ScyllaDB.AdditionalScyllaDBArguments, "--consistent-cluster-management=false")
			},
			postSchemaRestoreHook: func(ctx context.Context, ns string, nsClient framework.Client, targetSDC *scyllav1alpha1.ScyllaDBDatacenter) {
				var err error

				framework.By("Initiating a rolling restart of the target ScyllaDBDatacenter")
				targetSDC, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns).Patch(
					ctx,
					targetSDC.Name,
					types.MergePatchType,
					[]byte(`{"spec": {"forceRedeploymentReason": "schema restored"}}`),
					metav1.PatchOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Waiting for the target ScyllaDBDatacenter to roll out (RV=%s)", targetSDC.ResourceVersion)
				targetSDCRolloutAfterForcedRedeploymentCtx, targetSDCRolloutAfterForcedRedeploymentCtxCancel := utilsv1alpha1.ContextForRollout(ctx, targetSDC)
				defer targetSDCRolloutAfterForcedRedeploymentCtxCancel()
				targetSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(targetSDCRolloutAfterForcedRedeploymentCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns), targetSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), targetSDC)
				scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), targetSDC)

				framework.By("Enabling raft in target cluster")
				targetSDC, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns).Patch(
					ctx,
					targetSDC.Name,
					types.JSONPatchType,
					[]byte(`[{"op":"replace","path":"/spec/scyllaDB/additionalScyllaDBArguments/0","value":"--consistent-cluster-management=true"}]`),
					metav1.PatchOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Waiting for the target ScyllaDBDatacenter to roll out (RV=%s)", targetSDC.ResourceVersion)
				targetSDCRolloutCtx, targetSDCRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, targetSDC)
				defer targetSDCRolloutCtxCancel()
				targetSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(targetSDCRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns), targetSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), targetSDC)
				scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), targetSDC)
			},
		}),
	)
})

func setUpObjectStorageCredentials(ctx context.Context, ns string, nsClient framework.Client, sdc *scyllav1alpha1.ScyllaDBDatacenter, objectStorageSettings framework.ClusterObjectStorageSettings) {
	g.GinkgoHelper()

	o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
	switch objectStorageSettings.Type() {
	case framework.ObjectStorageTypeGCS:
		gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
		o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

		setUpGCSCredentials(ctx, nsClient.KubeClient().CoreV1(), sdc, ns, gcServiceAccountKey)

	case framework.ObjectStorageTypeS3:
		s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
		o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

		setUpS3Credentials(ctx, nsClient.KubeClient().CoreV1(), sdc, ns, s3CredentialsFile)

	}
}

func setUpGCSCredentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, namespace string, serviceAccountKey []byte) {
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "gcs-service-account",
		ReadOnly:  true,
		MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
		SubPath:   "gcs-service-account.json",
	})
}

func setUpS3Credentials(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, namespace string, s3CredentialsFile []byte) {
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdc.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "aws-credentials",
		ReadOnly:  true,
		MountPath: "/var/lib/scylla-manager/.aws/credentials",
		SubPath:   "credentials",
	})
}
