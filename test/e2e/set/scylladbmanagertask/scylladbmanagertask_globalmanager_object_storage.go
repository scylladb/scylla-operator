// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("ScyllaDBManagerTask and ScyllaDBCluster integration with global ScyllaDB Manager", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbmanagertask")

	g.It("should synchronise a backup task for ScyllaDBCluster and support a manual restore procedure", func(ctx g.SpecContext) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		sourceSC := f.GetDefaultScyllaDBCluster(rkcMap)

		framework.By("Setting up storage credentials for all DCs")
		for dcIdx, dcSpec := range sourceSC.Spec.Datacenters {
			workerClusterKey := dcSpec.Name
			objectStorageSettings := f.GetObjectStorageSettingsForWorkerCluster(workerClusterKey)

			o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
			switch objectStorageSettings.Type() {
			case framework.ObjectStorageTypeGCS:
				gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
				o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

				o.Expect(err).NotTo(o.HaveOccurred())
				sourceSC.Spec.Datacenters[dcIdx] = *setUpGCSCredentialsForScyllaDBClusterDatacenter(ctx, nsClient.KubeClient().CoreV1(), &dcSpec, ns.Name, gcServiceAccountKey)

			case framework.ObjectStorageTypeS3:
				s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
				o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

				o.Expect(err).NotTo(o.HaveOccurred())
				sourceSC.Spec.Datacenters[dcIdx] = *setUpS3CredentialsForScyllaDBClusterDatacenter(ctx, nsClient.KubeClient().CoreV1(), &dcSpec, ns.Name, s3CredentialsFile)
			}
		}

		framework.By(`Creating a ScyllaDBCluster with the global ScyllaDB Manager registration label`)
		metav1.SetMetaDataLabel(&sourceSC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		sourceSC, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Create(ctx, sourceSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sourceSC, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sourceSC.Name, sourceSC.ResourceVersion)
		waitSDBCRolloutCtx, cancelSDBCRollout := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sourceSC)
		defer cancelSDBCRollout()
		sourceSC, err = controllerhelpers.WaitForScyllaDBClusterState(waitSDBCRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sourceSC.Namespace), sourceSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sourceSC, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sourceSC)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Inserting data into the ScyllaDBCluster")
		hostsByDC := make(map[string][]string, len(sourceSC.Spec.Datacenters))
		for _, dc := range sourceSC.Spec.Datacenters {
			remoteClusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
			o.Expect(ok).To(o.BeTrue(), "Remote Kubernetes cluster %q should be present in the map", dc.RemoteKubernetesClusterName)

			dcStatus, _, ok := oslices.Find(sourceSC.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
				return dc.Name == dcStatus.Name
			})
			o.Expect(ok).To(o.BeTrue(), "Datacenter %q should be present in ScyllaDBCluster status", dc.Name)
			o.Expect(dcStatus.RemoteNamespaceName).ToNot(o.BeNil(), "Remote namespace name for datacenter %q should not be nil", dc.Name)

			sdcName := naming.ScyllaDBDatacenterName(sourceSC, &dc)
			sdc, err := remoteClusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, sdcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceHosts, err := utilsv1alpha1.GetBroadcastRPCAddresses(ctx, remoteClusterClient.KubeAdminClient().CoreV1(), sdc)
			o.Expect(err).NotTo(o.HaveOccurred())
			hostsByDC[dc.Name] = sourceHosts
		}

		// TODO: debug why this fails with:
		// can't create keyspace: gocql: no response received from cassandra within timeout period (potentially executed: false)

		di := verification.InsertAndVerifyCQLDataByDC(ctx, hostsByDC)
		defer di.Close()

		framework.By("Preparing backup locations for every DC")
		var backupLocations []string
		for workerKey := range workerClusters {
			backupLocations = append(backupLocations, utils.LocationForScyllaManagerWithDC(
				f.GetObjectStorageSettingsForWorkerCluster(workerKey),
				workerKey,
			))
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Backup'")
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
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Create(ctx, smt, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBManagerTask to register with the global ScyllaDB Manager instance")
		smtRegistrationCtx, smtRegistrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer smtRegistrationCtxCancel()
		smt, err = controllerhelpers.WaitForScyllaDBManagerTaskState(
			smtRegistrationCtx,
			nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name),
			smt.Name,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerTaskRolledOut,
			scyllaDBManagerTaskHasDeletionFinalizer,
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

		framework.By("Verifying that ScyllaDBManagerTask was registered with global ScyllaDB Manager")
		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

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

		var backupProgress managerclient.BackupProgress
		framework.By("Waiting for the backup task to finish")
		err = apimachineryutilwait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(context.Context) (done bool, err error) {
			backupProgress, err = managerClient.BackupProgress(ctx, sourceManagerClusterID, managerTask.ID, "latest")
			if err != nil {
				return false, err
			}

			return backupProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupProgress.Progress.SnapshotTag).NotTo(o.BeEmpty())

		framework.By("Deleting ScyllaDBManagerTask")
		err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Delete(
			ctx,
			smt.Name,
			metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
				Preconditions: &metav1.Preconditions{
					UID: pointer.Ptr(smt.UID),
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBManagerTask to be deleted")
		smtDeletionCtx, smtDeletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer smtDeletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			smtDeletionCtx,
			nsClient.DynamicClient(),
			scyllav1alpha1.GroupVersion.WithResource("scylladbmanagertasks"),
			smt.Namespace,
			smt.Name,
			pointer.Ptr(smt.UID),
		)

		framework.By("Verifying that ScyllaDBManagerTask was deleted from global ScyllaDB Manager")
		// 'GetTask' is broken and does not return an error after the task has been deleted.
		// XRef: https://github.com/scylladb/scylla-manager/issues/4400
		tasks, err := managerClient.ListTasks(ctx, sourceManagerClusterID, managerclient.BackupTask, false, "", "")
		o.Expect(err).To(o.HaveOccurred())

		o.Expect(slices.ContainsFunc(tasks.TaskListItemSlice, func(t *managerclient.TaskListItem) bool {
			return t.ID == managerTaskID.String()
		})).To(o.BeFalse(), "ScyllaDBManagerTask should be deleted from global ScyllaDB Manager")

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
		scDeletionCtx, scDeletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer scDeletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			scDeletionCtx,
			nsClient.DynamicClient(),
			scyllav1alpha1.GroupVersion.WithResource("scylladbclusters"),
			sourceSC.Namespace,
			sourceSC.Name,
			pointer.Ptr(sourceSC.UID),
		)

		targetSC := f.GetDefaultScyllaDBCluster(rkcMap)
		targetSC.Spec.ScyllaDB.Image = sourceSC.Spec.ScyllaDB.Image

		framework.By("Preparing target ScyllaDBCluster storage credentials for all DCs")
		for dcIdx, dcSpec := range targetSC.Spec.Datacenters {
			workerClusterKey := dcSpec.Name
			objectStorageSettings := f.GetObjectStorageSettingsForWorkerCluster(workerClusterKey)

			o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
			switch objectStorageSettings.Type() {
			case framework.ObjectStorageTypeGCS:
				gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
				o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

				targetSC.Spec.Datacenters[dcIdx] = *setUpGCSCredentialsForScyllaDBClusterDatacenter(ctx, nsClient.KubeClient().CoreV1(), &dcSpec, ns.Name, gcServiceAccountKey)

			case framework.ObjectStorageTypeS3:
				s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
				o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

				targetSC.Spec.Datacenters[dcIdx] = *setUpS3CredentialsForScyllaDBClusterDatacenter(ctx, nsClient.KubeClient().CoreV1(), &dcSpec, ns.Name, s3CredentialsFile)
			}
		}

		framework.By("Creating a target ScyllaDBCluster with the global ScyllaDB Manager registration label")
		metav1.SetMetaDataLabel(&targetSC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)
		targetSC, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, targetSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, targetSC, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaDBCluster %q roll out (RV=%s)", targetSC.Name, targetSC.ResourceVersion)
		targetSDBCRolloutCtx, targetSDBCRolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, targetSC)
		defer targetSDBCRolloutCtxCancel()
		targetSC, err = controllerhelpers.WaitForScyllaDBClusterState(targetSDBCRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(targetSC.Namespace), targetSC.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, targetSC, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBCluster to register with global ScyllaDB Manager instance")
		targetSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(targetSC)
		o.Expect(err).NotTo(o.HaveOccurred())
		targetSMCRRegistrationCtx, targetSMCRRegistrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer targetSMCRRegistrationCtxCancel()
		targetSMCR, err := controllerhelpers.WaitForScyllaDBManagerClusterRegistrationState(
			targetSMCRRegistrationCtx,
			nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns.Name),
			targetSMCRName,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerClusterRegistrationRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(targetSMCR.Status.ClusterID).NotTo(o.BeNil())
		o.Expect(*targetSMCR.Status.ClusterID).NotTo(o.BeEmpty())
		targetManagerClusterID := *targetSMCR.Status.ClusterID

		targetHostsByDC := make(map[string][]string, len(targetSC.Spec.Datacenters))
		for _, dc := range targetSC.Spec.Datacenters {
			remoteClusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
			o.Expect(ok).To(o.BeTrue(), "Remote Kubernetes cluster %q should be present in the map", dc.RemoteKubernetesClusterName)

			dcStatus, _, ok := oslices.Find(targetSC.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
				return dc.Name == dcStatus.Name
			})
			o.Expect(ok).To(o.BeTrue(), "Datacenter %q should be present in ScyllaDBCluster status", dc.Name)
			o.Expect(dcStatus.RemoteNamespaceName).ToNot(o.BeNil(), "Remote namespace name for datacenter %q should not be nil", dc.Name)

			sdcName := naming.ScyllaDBDatacenterName(targetSC, &dc)
			sdc, err := remoteClusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, sdcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceHosts, err := utilsv1alpha1.GetBroadcastRPCAddresses(ctx, remoteClusterClient.KubeAdminClient().CoreV1(), sdc)
			o.Expect(err).NotTo(o.HaveOccurred())
			targetHostsByDC[dc.Name] = sourceHosts
		}

		framework.By("Ensuring that the target ScyllaDBCluster is empty")
		err = di.SetClientEndpoints(oslices.Flatten(helpers.GetMapValues(targetHostsByDC)))
		o.Expect(err).NotTo(o.HaveOccurred())
		err = di.AwaitSchemaAgreement(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = di.Read()
		o.Expect(err).To(o.HaveOccurred(), "Data should not be present yet in the target ScyllaDBCluster")
		var gocqlErr gocql.RequestError
		o.Expect(errors.As(err, &gocqlErr)).To(o.BeTrue(), "Error should be a gocql.RequestError")
		o.Expect(gocqlErr.Code()).To(o.Equal(gocql.ErrCodeInvalid))
		o.Expect(gocqlErr.Error()).To(o.And(o.HavePrefix("Keyspace"), o.HaveSuffix("does not exist")))

		// Close the existing session to avoid polluting the logs.
		di.Close()

		framework.By("Getting the global ScyllaDB Manager instance pod")
		globalScyllaDBManagerInstancePods, err := f.KubeAdminClient().CoreV1().Pods(naming.ScyllaManagerNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: naming.ManagerSelector().String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(globalScyllaDBManagerInstancePods.Items).NotTo(o.BeEmpty())
		globalScyllaDBManagerInstancePod := globalScyllaDBManagerInstancePods.Items[0]

		framework.By("Creating schema restore tasks against global ScyllaDB Manager instance")
		for _, dcSpec := range targetSC.Spec.Datacenters {
			workerClusterKey := dcSpec.Name
			objectStorageSettings := f.GetObjectStorageSettingsForWorkerCluster(workerClusterKey)

			framework.By("Creating a schema restore task for %q", dcSpec.Name)
			schemaRestoreCreationCtx, schemaRestoreCreationCtxCancel := context.WithTimeoutCause(ctx, utils.SyncTimeout, fmt.Errorf("schema restore task creation has not completed in time"))
			defer schemaRestoreCreationCtxCancel()
			stdout, stderr, err := utils.ExecWithOptions(schemaRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
				Command: []string{
					"sctool",
					"restore",
					fmt.Sprintf("--cluster=%s", targetManagerClusterID),
					fmt.Sprintf("--location=%s", utils.LocationForScyllaManager(objectStorageSettings)),
					fmt.Sprintf("--snapshot-tag=%s", backupProgress.Progress.SnapshotTag),
					fmt.Sprintf("--datacenter=%s", dcSpec.Name),
					"--restore-schema",
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

			verifyRestoreTaskCompletion := func(eo o.Gomega, ctx context.Context, targetManagerClusterID, restoreTaskID string) {
				restoreProgress, err := managerClient.RestoreProgress(ctx, targetManagerClusterID, restoreTaskID, "latest")
				eo.Expect(err).NotTo(o.HaveOccurred())

				eo.Expect(restoreProgress.Errors).To(o.BeEmpty())
				eo.Expect(restoreProgress.Run).NotTo(o.BeNil())
				eo.Expect(restoreProgress.Run.Status).To(o.Equal(managerclient.TaskStatusDone))
			}

			framework.By("Waiting for the schema restore task for %q to finish", dcSpec.Name)
			o.Eventually(verifyRestoreTaskCompletion).
				WithContext(ctx).
				WithTimeout(5*time.Minute).
				WithPolling(5*time.Second).
				WithArguments(targetManagerClusterID, schemaRestoreTaskID.String()).
				Should(o.Succeed())

			// TODO: restart remote ScyllaDBDatacenter to ensure that the restored schema is applied
		}

		// TODO: run tables restore tasks for each DC

		// TODO: verify that the data is present in the target ScyllaDBCluster
	})
})

var _ = g.Describe("ScyllaDBManagerTask and ScyllaDBDatacenter integration with global ScyllaDB Manager", func() {
	f := framework.NewFramework("scylladbmanagertask")

	type entry struct {
		scyllaDBImage              string
		preTargetClusterCreateHook func(*scyllav1alpha1.ScyllaDBDatacenter)
		postSchemaRestoreHook      func(context.Context, string, framework.Client, *scyllav1alpha1.ScyllaDBDatacenter)
	}

	g.DescribeTable("should synchronise a backup task for ScyllaDBDatacenter and support a manual restore procedure", func(ctx g.SpecContext, e entry) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sourceSDC := f.GetDefaultScyllaDBDatacenter()
		if len(e.scyllaDBImage) != 0 {
			sourceSDC.Spec.ScyllaDB.Image = e.scyllaDBImage
		}

		metav1.SetMetaDataLabel(&sourceSDC.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		objectStorageSettings, ok := f.GetClusterObjectStorageSettings()
		o.Expect(ok).To(o.BeTrue(), "cluster object storage settings must be configured for this test")

		o.Expect(objectStorageSettings.Type()).To(o.BeElementOf(framework.ObjectStorageTypeGCS, framework.ObjectStorageTypeS3))
		switch objectStorageSettings.Type() {
		case framework.ObjectStorageTypeGCS:
			gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
			o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

			sourceSDC = setUpGCSCredentialsForScyllaDBDatacenter(ctx, nsClient.KubeClient().CoreV1(), sourceSDC, ns.Name, gcServiceAccountKey)
		case framework.ObjectStorageTypeS3:
			s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
			o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

			sourceSDC = setUpS3CredentialsForScyllaDBDatacenter(ctx, nsClient.KubeClient().CoreV1(), sourceSDC, ns.Name, s3CredentialsFile)
		}

		framework.By("Creating a ScyllaDBDatacenter with the global ScyllaDB Manager registration label")
		sourceSDC, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sourceSDC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to roll out (RV=%s)", sourceSDC.ResourceVersion)
		sourceSDCRolloutCtx, sourceSDCRolloutCtxCancel := utilsv1alpha1.ContextForScyllaDBDatacenterRollout(ctx, sourceSDC)
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
						NumRetries: pointer.Ptr[int64](1),
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
			scyllaDBManagerTaskHasDeletionFinalizer,
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

		var backupProgress managerclient.BackupProgress
		framework.By("Waiting for the backup task to finish")
		err = apimachineryutilwait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(context.Context) (done bool, err error) {
			backupProgress, err = managerClient.BackupProgress(ctx, sourceManagerClusterID, managerTask.ID, "latest")
			if err != nil {
				return false, err
			}

			return backupProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(backupProgress.Progress.SnapshotTag).NotTo(o.BeEmpty())

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

		switch objectStorageSettings.Type() {
		case framework.ObjectStorageTypeGCS:
			gcServiceAccountKey := objectStorageSettings.GCSServiceAccountKey()
			o.Expect(gcServiceAccountKey).NotTo(o.BeEmpty())

			targetSDC = setUpGCSCredentialsForScyllaDBDatacenter(ctx, nsClient.KubeClient().CoreV1(), targetSDC, ns.Name, gcServiceAccountKey)
		case framework.ObjectStorageTypeS3:
			s3CredentialsFile := objectStorageSettings.S3CredentialsFile()
			o.Expect(s3CredentialsFile).NotTo(o.BeEmpty())

			targetSDC = setUpS3CredentialsForScyllaDBDatacenter(ctx, nsClient.KubeClient().CoreV1(), targetSDC, ns.Name, s3CredentialsFile)
		default:
			g.Fail("unsupported object storage type")
		}

		framework.By("Creating the target ScyllaDBDatacenter with the global ScyllaDB Manager registration label")
		targetSDC, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, targetSDC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaDBDatacenter to roll out (RV=%s)", targetSDC.ResourceVersion)
		targetSDCRolloutAfterForcedRedeploymentCtx, targetSDCRolloutCtxCancel := utilsv1alpha1.ContextForScyllaDBDatacenterRollout(ctx, targetSDC)
		defer targetSDCRolloutCtxCancel()
		targetSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(targetSDCRolloutAfterForcedRedeploymentCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), targetSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
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
		var gocqlErr gocql.RequestError
		o.Expect(errors.As(err, &gocqlErr)).To(o.BeTrue())
		o.Expect(gocqlErr.Code()).To(o.Equal(gocql.ErrCodeInvalid))
		o.Expect(gocqlErr.Error()).To(o.And(o.HavePrefix("Keyspace"), o.HaveSuffix("does not exist")))

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
				fmt.Sprintf("--snapshot-tag=%s", backupProgress.Progress.SnapshotTag),
				"--restore-schema",
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

		verifyRestoreTaskCompletion := func(eo o.Gomega, ctx context.Context, targetManagerClusterID, restoreTaskID string) {
			restoreProgress, err := managerClient.RestoreProgress(ctx, targetManagerClusterID, restoreTaskID, "latest")
			eo.Expect(err).NotTo(o.HaveOccurred())

			eo.Expect(restoreProgress.Errors).To(o.BeEmpty())
			eo.Expect(restoreProgress.Run).NotTo(o.BeNil())
			eo.Expect(restoreProgress.Run.Status).To(o.Equal(managerclient.TaskStatusDone))
		}

		framework.By("Waiting for the schema restore task to finish")
		o.Eventually(verifyRestoreTaskCompletion).
			WithContext(ctx).
			WithTimeout(5*time.Minute).
			WithPolling(5*time.Second).
			WithArguments(targetManagerClusterID, schemaRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Initiating a rolling restart of the target ScyllaDBDatacenter")
		targetSDC, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Patch(
			ctx,
			targetSDC.Name,
			types.MergePatchType,
			[]byte(`{"spec": {"forceRedeploymentReason": "schema restored"}}`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the target ScyllaDBDatacenter to roll out (RV=%s)", targetSDC.ResourceVersion)
		targetSDCRolloutAfterForcedRedeploymentCtx, targetSDCRolloutAfterForcedRedeploymentCtxCancel := utilsv1alpha1.ContextForScyllaDBDatacenterRollout(ctx, targetSDC)
		defer targetSDCRolloutAfterForcedRedeploymentCtxCancel()
		targetSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(targetSDCRolloutAfterForcedRedeploymentCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), targetSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), targetSDC)
		scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), targetSDC)

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
				fmt.Sprintf("--snapshot-tag=%s", backupProgress.Progress.SnapshotTag),
				"--restore-tables",
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
		o.Eventually(verifyRestoreTaskCompletion).
			WithContext(ctx).
			WithTimeout(5*time.Minute).
			WithPolling(5*time.Second).
			WithArguments(targetManagerClusterID, tablesRestoreTaskID.String()).
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
			scyllaDBImage: fmt.Sprintf("docker.io/scylladb/scylla-enterprise:%s", configassests.Project.Operator.ScyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride),
			preTargetClusterCreateHook: func(targetSDC *scyllav1alpha1.ScyllaDBDatacenter) {
				targetSDC.Spec.ScyllaDB.AdditionalScyllaDBArguments = []string{"--consistent-cluster-management=false"}
			},
			postSchemaRestoreHook: func(ctx context.Context, ns string, nsClient framework.Client, targetSDC *scyllav1alpha1.ScyllaDBDatacenter) {
				var err error

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
				targetSDCRolloutCtx, targetSDCRolloutCtxCancel := utilsv1alpha1.ContextForScyllaDBDatacenterRollout(ctx, targetSDC)
				defer targetSDCRolloutCtxCancel()
				targetSDC, err = controllerhelpers.WaitForScyllaDBDatacenterState(targetSDCRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns), targetSDC.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), targetSDC)
				scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), targetSDC)
			},
		}),
	)
})

func setUpGCSCredentialsForScyllaDBClusterDatacenter(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBClusterDatacenter, namespace string, serviceAccountKey []byte) *scyllav1alpha1.ScyllaDBClusterDatacenter {
	// Create a copy of the ScyllaDBDatacenter to avoid modifying the original object.
	sdcCopy := sdc.DeepCopy()

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
	if sdcCopy.RackTemplate == nil {
		sdcCopy.RackTemplate = &scyllav1alpha1.RackTemplate{}
	}
	if sdcCopy.RackTemplate.ScyllaDBManagerAgent == nil {
		sdcCopy.RackTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{}
	}
	sdcCopy.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdcCopy.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdcCopy.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdcCopy.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "gcs-service-account",
		ReadOnly:  true,
		MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
		SubPath:   "gcs-service-account.json",
	})

	return sdcCopy
}

func setUpGCSCredentialsForScyllaDBDatacenter(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, namespace string, serviceAccountKey []byte) *scyllav1alpha1.ScyllaDBDatacenter {
	sdcCopy := sdc.DeepCopy()

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

	sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "gcs-service-account",
		ReadOnly:  true,
		MountPath: "/etc/scylla-manager-agent/gcs-service-account.json",
		SubPath:   "gcs-service-account.json",
	})

	return sdcCopy
}

func setUpS3CredentialsForScyllaDBClusterDatacenter(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBClusterDatacenter, namespace string, s3CredentialsFile []byte) *scyllav1alpha1.ScyllaDBClusterDatacenter {
	// Create a copy of the ScyllaDBDatacenter to avoid modifying the original object.
	sdcCopy := sdc.DeepCopy()

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
	sdcCopy.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdcCopy.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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
	sdcCopy.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdcCopy.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "aws-credentials",
		ReadOnly:  true,
		MountPath: "/var/lib/scylla-manager/.aws/credentials",
		SubPath:   "credentials",
	})

	return sdcCopy
}

func setUpS3CredentialsForScyllaDBDatacenter(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, namespace string, s3CredentialsFile []byte) *scyllav1alpha1.ScyllaDBDatacenter {
	sdcCopy := sdc.DeepCopy()

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

	sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes = append(sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.Volumes, corev1.Volume{
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

	sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts = append(sdcCopy.Spec.RackTemplate.ScyllaDBManagerAgent.VolumeMounts, corev1.VolumeMount{
		Name:      "aws-credentials",
		ReadOnly:  true,
		MountPath: "/var/lib/scylla-manager/.aws/credentials",
		SubPath:   "credentials",
	})

	return sdcCopy
}
