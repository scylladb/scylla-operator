// Copyright (C) 2026 ScyllaDB

package multidatacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Requires object storage configured for every worker cluster, which the multi-datacenter kind runner sets up and the
// single-datacenter one does not.
var _ = g.Describe("ScyllaDBManagerTask and a multi-datacenter ScyllaDB cluster integration with global ScyllaDB Manager", framework.SuiteMultiDatacenterParallel, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scylladbmanagertask")
	})

	g.It("should synchronise a backup task and support a manual restore procedure", func(ctx g.SpecContext) {
		// The target cluster is restored into a new set of ScyllaClusters, named differently from the source ones, so
		// that they get their own StatefulSets and, with them, freshly provisioned storage.
		const (
			sourceClusterName = "source-multi-datacenter-cluster"
			targetClusterName = "target-multi-datacenter-cluster"
		)

		workerClusterKeys := orderWorkerClusterKeysByControlPlaneAffinity(f)
		// Every datacenter backs up to its own, datacenter-scoped location, so each one needs its own object storage
		// entry.
		o.Expect(len(workerClusterKeys)).To(o.BeNumerically(">=", 2), "at least 2 worker clusters are required")

		// ScyllaDB Manager only ever runs in the control plane cluster, so the datacenter registering the ring, and
		// with it the ScyllaDBManagerTask, has to be created there.
		controlPlaneNS, controlPlaneNSClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		remoteWorkerClusterKey := workerClusterKeys[len(workerClusterKeys)-1]
		remoteWorkerCluster := f.WorkerClusters()[remoteWorkerClusterKey]
		remoteNS, remoteNSClient := remoteWorkerCluster.CreateUserNamespace(ctx)

		newDatacenters := func() []*datacenter {
			return []*datacenter{
				{
					name:             "dc1",
					workerClusterKey: workerClusterKeys[0],
					namespace:        controlPlaneNS.Name,
					client:           controlPlaneNSClient,
				},
				{
					name:             "dc2",
					workerClusterKey: remoteWorkerClusterKey,
					namespace:        remoteNS.Name,
					client:           remoteNSClient,
				},
			}
		}

		// Nothing mirrors Secrets across the datacenters' namespaces, so each one gets its own credentials for its own
		// object storage location.
		setUpDatacenterObjectStorageCredentials := func(dc *datacenter, sc *scyllav1.ScyllaCluster) {
			utils.SetUpObjectStorageCredentials(ctx, dc.namespace, dc.client, sc, f.GetObjectStorageSettingsForWorkerCluster(dc.workerClusterKey))
		}

		sourceDatacenters := newDatacenters()
		setUpMultiDatacenterScyllaCluster(ctx, f, sourceClusterName, sourceDatacenters, setUpDatacenterObjectStorageCredentials)

		sourceHostsByDC, _, err := utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcCoreClientMap(sourceDatacenters), datacenterScyllaClusters(sourceDatacenters))
		o.Expect(err).NotTo(o.HaveOccurred())

		allSourceHosts := slices.Concat(slices.Collect(maps.Values(sourceHostsByDC))...)
		di := verification.InsertAndVerifyCQLData(ctx, allSourceHosts)
		defer di.Close()

		backupLocations := datacenterBackupLocations(f, sourceDatacenters)

		smt := &scyllav1alpha1.ScyllaDBManagerTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup",
				Namespace: controlPlaneNS.Name,
			},
			Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
				ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
					Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
					Name: sourceDatacenters[0].sc.Name,
				},
				Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
				Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
					ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
						NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
						RetryWait: &metav1.Duration{
							Duration: utils.ScyllaDBManagerTaskRetryWait,
						},
					},
					Location:  backupLocations,
					Retention: pointer.Ptr[int64](1),
				},
			},
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Backup'")
		smt, err = controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(controlPlaneNS.Name).Create(ctx, smt, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBManagerTask to register with global ScyllaDB Manager instance")
		registrationCtx, registrationCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("ScyllaDBManagerTask %q has not registered with global ScyllaDB Manager instance in time", naming.ObjRef(smt)),
		)
		defer registrationCtxCancel()

		smt, err = controllerhelpers.WaitForScyllaDBManagerTaskState(
			registrationCtx,
			controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(controlPlaneNS.Name),
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

		sourceSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
		o.Expect(err).NotTo(o.HaveOccurred())
		sourceSMCR, err := controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(controlPlaneNS.Name).Get(ctx, sourceSMCRName, metav1.GetOptions{})
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

		framework.By("Waiting for the backup task to finish")
		backupTaskCompletionCtx, backupTaskCompletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerMultiDatacenterTaskCompletionTimeout,
			fmt.Errorf("backup task %q has not finished in time", managerTaskID),
		)
		defer backupTaskCompletionCtxCancel()

		o.Eventually(verification.VerifyScyllaDBManagerBackupTaskCompleted).
			WithContext(backupTaskCompletionCtx).
			WithPolling(5*time.Second).
			WithArguments(managerClient, sourceManagerClusterID, managerTask.ID).
			Should(o.Succeed())

		// The backup covers every datacenter, so by now ScyllaDB Manager has talked to the agent of every node of the
		// source cluster using the shared auth token.
		framework.By("Verifying that global ScyllaDB Manager can reach every node of the source cluster")
		verifyManagerClusterStatusHealthy(ctx, managerClient, sourceManagerClusterID, sourceDatacenters)

		backupProgress, err := managerClient.BackupProgress(ctx, sourceManagerClusterID, managerTask.ID, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		snapshotTag := backupProgress.Progress.SnapshotTag
		o.Expect(snapshotTag).NotTo(o.BeEmpty())

		framework.By("Deleting ScyllaDBManagerTask")
		err = controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(controlPlaneNS.Name).Delete(
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
		taskDeletionCtx, taskDeletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("ScyllaDBManagerTask %q has not been deleted in time", naming.ObjRef(smt)),
		)
		defer taskDeletionCtxCancel()

		err = framework.WaitForObjectDeletion(
			taskDeletionCtx,
			controlPlaneNSClient.DynamicClient(),
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

		// Every datacenter is deleted, so that the data can only come back from the backup. Deleting the ScyllaClusters
		// with foreground propagation also tears down their ScyllaDBDatacenters and, with them, the cluster's
		// registration with global ScyllaDB Manager, before the target cluster registers.
		for _, dc := range sourceDatacenters {
			framework.By("Deleting the source ScyllaCluster of datacenter %q", dc.name)
			err = dc.client.ScyllaClient().ScyllaV1().ScyllaClusters(dc.namespace).Delete(
				ctx,
				dc.sc.Name,
				metav1.DeleteOptions{
					PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
					Preconditions: &metav1.Preconditions{
						UID: &dc.sc.UID,
					},
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		for _, dc := range sourceDatacenters {
			framework.By("Waiting for the source ScyllaCluster of datacenter %q to be deleted", dc.name)
			sourceSCDeletionCtx, sourceSCDeletionCtxCancel := context.WithTimeoutCause(
				ctx,
				utils.MultiDatacenterScyllaClusterTerminationTimeout,
				fmt.Errorf("source ScyllaCluster %q has not been deleted in time", naming.ObjRef(dc.sc)),
			)
			defer sourceSCDeletionCtxCancel()

			err = framework.WaitForObjectDeletion(
				sourceSCDeletionCtx,
				dc.client.DynamicClient(),
				scyllav1.GroupVersion.WithResource("scyllaclusters"),
				dc.sc.Namespace,
				dc.sc.Name,
				pointer.Ptr(dc.sc.UID),
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		targetDatacenters := newDatacenters()
		setUpMultiDatacenterScyllaCluster(ctx, f, targetClusterName, targetDatacenters, setUpDatacenterObjectStorageCredentials)

		framework.By("Verifying that the target cluster does not have the data that's yet to be restored")
		targetHostsByDC, _, err := utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcCoreClientMap(targetDatacenters), datacenterScyllaClusters(targetDatacenters))
		o.Expect(err).NotTo(o.HaveOccurred())
		allTargetHosts := slices.Concat(slices.Collect(maps.Values(targetHostsByDC))...)

		// After a fresh cluster is provisioned, nodes may still be finishing gossip stabilisation.
		// During this window, CQL queries can receive transport-level errors instead of the expected
		// protocol-level error. Retry with a fresh session on each attempt until the cluster
		// is reachable and confirms the keyspace does not yet exist.
		// Note: gocql may wrap protocol-level errors inside a QueryError, losing the typed
		// RequestError interface. We assert on the error message instead of the error type.
		cqlStabilizationCtx, cqlStabilizationCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaClusterCQLStabilizationTimeout,
			fmt.Errorf("target cluster did not reach CQL stability in time"),
		)
		defer cqlStabilizationCtxCancel()

		o.Eventually(func(eo o.Gomega) {
			eo.Expect(di.SetClientEndpoints(allTargetHosts)).NotTo(o.HaveOccurred())
			eo.Expect(di.AwaitSchemaAgreement(cqlStabilizationCtx)).NotTo(o.HaveOccurred())
			_, readErr := di.Read()
			eo.Expect(readErr).To(o.HaveOccurred())
			eo.Expect(readErr.Error()).To(o.ContainSubstring("does not exist"))
		}).WithContext(cqlStabilizationCtx).WithPolling(5 * time.Second).Should(o.Succeed())

		// Close the existing session to avoid polluting the logs.
		di.Close()

		framework.By("Waiting for the target cluster to register with global ScyllaDB Manager instance")
		targetSCRegistrationCtx, targetSCRegistrationCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerClusterSyncTimeout,
			fmt.Errorf("target cluster has not registered with global ScyllaDB Manager instance in time"),
		)
		defer targetSCRegistrationCtxCancel()

		targetSMCRName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(&scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: targetDatacenters[0].sc.Name,
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		targetSMCR, err := controllerhelpers.WaitForScyllaDBManagerClusterRegistrationState(
			targetSCRegistrationCtx,
			controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(controlPlaneNS.Name),
			targetSMCRName,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerClusterRegistrationRolledOut,
		)
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

		// A restore reads from every datacenter's location, so they are all passed to sctool at once.
		restoreLocation := strings.Join(datacenterBackupLocations(f, targetDatacenters), ",")

		framework.By("Creating a schema restore task against global ScyllaDB Manager instance")
		schemaRestoreCreationCtx, schemaRestoreCreationCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("schema restore task creation has not completed in time"),
		)
		defer schemaRestoreCreationCtxCancel()

		stdout, stderr, err := utils.ExecWithOptions(schemaRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", restoreLocation),
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
		schemaRestoreTaskCompletionCtx, schemaRestoreTaskCompletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerMultiDatacenterTaskCompletionTimeout,
			fmt.Errorf("schema restore task %q has not finished in time", schemaRestoreTaskID),
		)
		defer schemaRestoreTaskCompletionCtxCancel()

		o.Eventually(verification.VerifyScyllaDBManagerRestoreTaskCompleted).
			WithContext(schemaRestoreTaskCompletionCtx).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, schemaRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Creating a tables restore task against global ScyllaDB Manager instance")
		tablesRestoreCreationCtx, tablesRestoreCreationCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("tables restore task creation has not completed in time"),
		)
		defer tablesRestoreCreationCtxCancel()

		stdout, stderr, err = utils.ExecWithOptions(tablesRestoreCreationCtx, f.AdminClientConfig(), f.KubeAdminClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"sctool",
				"restore",
				fmt.Sprintf("--cluster=%s", targetManagerClusterID),
				fmt.Sprintf("--location=%s", restoreLocation),
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
		tablesRestoreTaskCompletionCtx, tablesRestoreTaskCompletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerMultiDatacenterTaskCompletionTimeout,
			fmt.Errorf("tables restore task %q has not finished in time", tablesRestoreTaskID),
		)
		defer tablesRestoreTaskCompletionCtxCancel()

		o.Eventually(verification.VerifyScyllaDBManagerRestoreTaskCompleted).
			WithContext(tablesRestoreTaskCompletionCtx).
			WithPolling(5*time.Second).
			WithArguments(managerClient, targetManagerClusterID, tablesRestoreTaskID.String()).
			Should(o.Succeed())

		framework.By("Validating that the data restored from the source cluster backup is available in the target cluster")
		err = di.SetClientEndpoints(allTargetHosts)
		o.Expect(err).NotTo(o.HaveOccurred())

		verification.VerifyCQLData(ctx, di)
	})
})

// datacenterBackupLocations returns the datacenter-scoped ScyllaDB Manager locations of the given datacenters, one per
// datacenter. The location is scoped by the ScyllaDB datacenter name, so that every datacenter backs up to, and
// restores from, its own bucket.
func datacenterBackupLocations(f *framework.Framework, datacenters []*datacenter) []string {
	g.GinkgoHelper()

	locations := make([]string, 0, len(datacenters))
	for _, dc := range datacenters {
		locations = append(locations, utils.LocationForScyllaManagerWithDC(
			f.GetObjectStorageSettingsForWorkerCluster(dc.workerClusterKey),
			dc.name,
		))
	}

	return locations
}
