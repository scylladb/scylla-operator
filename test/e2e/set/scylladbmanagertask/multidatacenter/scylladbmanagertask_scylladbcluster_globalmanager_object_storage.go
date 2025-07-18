// Copyright (C) 2025 ScyllaDB

package multidatacenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	scyllaDBManagerRestoreNumRetries = 3
	scyllaDBManagerRestoreRetryWait  = time.Minute
)

var _ = g.Describe("ScyllaDBManagerTask and ScyllaDBCluster integration with global ScyllaDB Manager", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbmanagertask")

	g.It("should synchronise a backup task and support a manual restore procedure", func(ctx g.SpecContext) {
		ns, nsClient := f.CreateUserNamespace(ctx)

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		sourceSC := f.GetDefaultScyllaDBCluster(rkcMap)
		metav1.SetMetaDataLabel(&sourceSC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		setUpObjectStorageCredentials(ctx, ns.Name, nsClient, sourceSC, f.GetObjectStorageSettingsForWorkerCluster)

		framework.By(`Creating a source ScyllaDBCluster with the global ScyllaDB Manager registration label`)
		sourceSC, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Create(ctx, sourceSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sourceSC, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the source ScyllaDBCluster %q roll out (RV=%s)", sourceSC.Name, sourceSC.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sourceSC)
		defer rolloutCtxCancel()
		sourceSC, err = controllerhelpers.WaitForScyllaDBClusterState(rolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sourceSC, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sourceSC)
		o.Expect(err).NotTo(o.HaveOccurred())

		sourceHostsByDC, err := utilsv1alpha1.GetBroadcastRPCAddressesForScyllaDBCluster(ctx, rkcClusterMap, sourceSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		allSourceHosts := slices.Concat(slices.Collect(maps.Values(sourceHostsByDC))...)
		o.Expect(allSourceHosts).To(o.HaveLen(int(controllerhelpers.GetScyllaDBClusterNodeCount(sourceSC))))
		di := verification.InsertAndVerifyCQLDataByDC(ctx, sourceHostsByDC)
		defer di.Close()

		var backupLocations []string
		for workerClusterKey := range workerClusters {
			backupLocations = append(backupLocations, utils.LocationForScyllaManagerWithDC(
				f.GetObjectStorageSettingsForWorkerCluster(workerClusterKey),
				workerClusterKey,
			))
		}

		smt := &scyllav1alpha1.ScyllaDBManagerTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup",
				Namespace: ns.Name,
			},
			Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
				ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
					Kind: scyllav1alpha1.ScyllaDBClusterGVK.Kind,
					Name: sourceSC.Name,
				},
				Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
				Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
					ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
						NumRetries: pointer.Ptr[int64](1),
					},
					Location:  backupLocations,
					Retention: pointer.Ptr[int64](2),
				},
			},
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Backup'")
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Create(ctx, smt, metav1.CreateOptions{})
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

		sourceSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(sourceSC)
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
			WithTimeout(3*time.Minute).
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

		framework.By("Deleting the source ScyllaDBCluster")
		err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Delete(
			ctx,
			sourceSC.Name,
			metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
				Preconditions: &metav1.Preconditions{
					UID: &sourceSC.UID,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the source ScyllaDBCluster to be deleted")
		err = framework.WaitForObjectDeletion(
			ctx,
			nsClient.DynamicClient(),
			scyllav1alpha1.GroupVersion.WithResource("scylladbclusters"),
			sourceSC.Namespace,
			sourceSC.Name,
			pointer.Ptr(sourceSC.UID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		targetSC := f.GetDefaultScyllaDBCluster(rkcMap)
		metav1.SetMetaDataLabel(&targetSC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		setUpObjectStorageCredentials(ctx, ns.Name, nsClient, targetSC, f.GetObjectStorageSettingsForWorkerCluster)

		framework.By(`Creating a target ScyllaDBCluster with the global ScyllaDB Manager registration label`)
		targetSC, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Create(ctx, targetSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, targetSC, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaDBCluster %q roll out (RV=%s)", targetSC.Name, targetSC.ResourceVersion)
		targetSCRolloutCtx, targetSCRolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, targetSC)
		defer targetSCRolloutCtxCancel()
		targetSC, err = controllerhelpers.WaitForScyllaDBClusterState(targetSCRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, targetSC, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that the target ScyllaDBCluster does not have the data that's yet to be restored")

		targetHostsByDC, err := utilsv1alpha1.GetBroadcastRPCAddressesForScyllaDBCluster(ctx, rkcClusterMap, targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		allTargetHosts := slices.Concat(slices.Collect(maps.Values(targetHostsByDC))...)
		o.Expect(allTargetHosts).To(o.HaveLen(int(controllerhelpers.GetScyllaDBClusterNodeCount(targetSC))))
		err = di.SetClientEndpoints(allTargetHosts)
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

		targetSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for target ScyllaDBCluster to register with global ScyllaDB Manager instance")
		targetSCRegistrationCtx, targetSCRegistrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer targetSCRegistrationCtxCancel()
		targetSMCR, err := controllerhelpers.WaitForScyllaDBManagerClusterRegistrationState(targetSCRegistrationCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns.Name), targetSMCRName, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBManagerClusterRegistrationRolledOut)
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

		var restoreLocations []string
		for workerClusterKey := range workerClusters {
			restoreLocations = append(restoreLocations, utils.LocationForScyllaManagerWithDC(
				f.GetObjectStorageSettingsForWorkerCluster(workerClusterKey),
				workerClusterKey,
			))
		}
		restoreLocation := strings.Join(restoreLocations, ",")

		framework.By("Creating a schema restore task against global ScyllaDB Manager instance")
		schemaRestoreCreationCtx, schemaRestoreCreationCtxCancel := context.WithTimeoutCause(ctx, utils.SyncTimeout, fmt.Errorf("schema restore task creation has not completed in time"))
		defer schemaRestoreCreationCtxCancel()
		stdout, stderr, err := utils.ExecWithOptions(schemaRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", restoreLocation),
				fmt.Sprintf("--snapshot-tag=%s", snapshotTag),
				"--restore-schema",
				fmt.Sprintf("--num-retries=%d", scyllaDBManagerRestoreNumRetries),
				fmt.Sprintf("--retry-wait=%s", scyllaDBManagerRestoreRetryWait),
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
			WithTimeout(15*time.Minute).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, schemaRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Creating a tables restore task against global ScyllaDB Manager instance")
		tablesRestoreCreationCtx, tablesRestoreCreationCtxCancel := context.WithTimeoutCause(ctx, utils.SyncTimeout, fmt.Errorf("tables restore task creation has not completed in time"))
		defer tablesRestoreCreationCtxCancel()
		stdout, stderr, err = utils.ExecWithOptions(tablesRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", restoreLocation),
				fmt.Sprintf("--snapshot-tag=%s", snapshotTag),
				"--restore-tables",
				fmt.Sprintf("--num-retries=%d", scyllaDBManagerRestoreNumRetries),
				fmt.Sprintf("--retry-wait=%s", scyllaDBManagerRestoreRetryWait),
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
			WithTimeout(15*time.Minute).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, tablesRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Validating that the data restored from the source cluster backup is available in the target cluster")
		targetHostsByDC, err = utilsv1alpha1.GetBroadcastRPCAddressesForScyllaDBCluster(ctx, rkcClusterMap, targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		allTargetHosts = slices.Concat(slices.Collect(maps.Values(targetHostsByDC))...)
		o.Expect(allTargetHosts).To(o.HaveLen(int(controllerhelpers.GetScyllaDBClusterNodeCount(targetSC))))
		err = di.SetClientEndpoints(allTargetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())

		verification.VerifyCQLData(ctx, di)
	})
})

func setUpObjectStorageCredentials(ctx context.Context, ns string, nsClient framework.Client, sc *scyllav1alpha1.ScyllaDBCluster, getObjectStorageSettingsForScyllaDBClusterDatacenter func(string) framework.ClusterObjectStorageSettings) {
	g.GinkgoHelper()

	for dcIdx := range sc.Spec.Datacenters {
		dcSpec := &sc.Spec.Datacenters[dcIdx]

		objectStorageSettings := getObjectStorageSettingsForScyllaDBClusterDatacenter(dcSpec.Name)

		o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
		switch objectStorageSettings.Type() {
		case framework.ObjectStorageTypeGCS:
			gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
			o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

			setUpGCSCredentialsForScyllaDBClusterDatacenter(ctx, nsClient.KubeClient().CoreV1(), dcSpec, ns, gcServiceAccountKey)

		case framework.ObjectStorageTypeS3:
			s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
			o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

			setUpS3CredentialsForScyllaDBClusterDatacenter(ctx, nsClient.KubeClient().CoreV1(), dcSpec, ns, s3CredentialsFile)

		}
	}
}

func setUpGCSCredentialsForScyllaDBClusterDatacenter(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBClusterDatacenter, namespace string, serviceAccountKey []byte) {
	g.GinkgoHelper()

	// Create a secret to hold the GCS service account key.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gcs-service-account-key-",
		},
		Data: map[string][]byte{
			"gcs-service-account.json": serviceAccountKey,
		},
	}

	// Create the secret in the specified namespace.
	secret, err := coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Add the secret as a volume and mount it in the ScyllaDBManagerAgent.
	if sdc.RackTemplate == nil {
		sdc.RackTemplate = &scyllav1alpha1.RackTemplate{}
	}
	if sdc.RackTemplate.ScyllaDBManagerAgent == nil {
		sdc.RackTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{}
	}
	sdc.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdc.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdc.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdc.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "gcs-service-account",
		ReadOnly:  true,
		MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
		SubPath:   "gcs-service-account.json",
	})
}

func setUpS3CredentialsForScyllaDBClusterDatacenter(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBClusterDatacenter, namespace string, s3CredentialsFile []byte) {
	g.GinkgoHelper()

	// Create a secret to hold the S3 credentials file.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "s3-credentials-file-",
		},
		Data: map[string][]byte{
			"credentials": s3CredentialsFile,
		},
	}

	// Create the secret in the specified namespace.
	secret, err := coreClient.Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Add the secret as a volume and mount it in the ScyllaDBManagerAgent.
	sdc.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdc.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
		Name: "aws-credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.Name,
				Items: []corev1.KeyToPath{
					{
						Key: "credentials",
					},
				},
			},
		},
	})
	sdc.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdc.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "aws-credentials",
		ReadOnly:  true,
		MountPath: "/var/lib/scylla-manager/.aws/credentials",
		SubPath:   "credentials",
	})
}
