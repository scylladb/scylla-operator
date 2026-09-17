// Copyright (C) 2026 ScyllaDB

package multidatacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	repairTaskInitialParallel = int64(1)
	repairTaskUpdatedParallel = int64(2)
)

var _ = g.Describe("ScyllaDBManagerTask and a multi-datacenter ScyllaDB cluster integration with global ScyllaDB Manager", framework.SuiteMultiDatacenterParallel, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scylladbmanagertask")
	})

	g.It("should synchronise a repair task running over the entire multi-datacenter ring", func(ctx g.SpecContext) {
		workerClusterKeys := orderWorkerClusterKeysByControlPlaneAffinity(f)
		o.Expect(workerClusterKeys).NotTo(o.BeEmpty(), "at least 1 worker cluster is required")

		// The global ScyllaDB Manager instance only ever runs in the control plane cluster, and its controller only
		// registers objects living in its own cluster, so the datacenter registering the ring has to be created there.
		controlPlaneNS, controlPlaneNSClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		remoteWorkerClusterKey := workerClusterKeys[len(workerClusterKeys)-1]
		remoteWorkerCluster := f.WorkerClusters()[remoteWorkerClusterKey]
		remoteNS, remoteNSClient := remoteWorkerCluster.CreateUserNamespace(ctx)

		datacenters := []*datacenter{
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

		setUpMultiDatacenterScyllaCluster(ctx, f, "multi-datacenter-cluster", datacenters, nil)

		hostsByDC, _, err := utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcCoreClientMap(datacenters), datacenterScyllaClusters(datacenters))
		o.Expect(err).NotTo(o.HaveOccurred())

		allHosts := slices.Concat(slices.Collect(maps.Values(hostsByDC))...)
		di := verification.InsertAndVerifyCQLData(ctx, allHosts)
		g.DeferCleanup(di.Close)

		smt := &scyllav1alpha1.ScyllaDBManagerTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repair",
				Namespace: controlPlaneNS.Name,
			},
			Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
				ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
					Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
					Name: datacenters[0].sc.Name,
				},
				Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
				Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
					ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
						NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
						RetryWait: &metav1.Duration{
							Duration: utils.ScyllaDBManagerTaskRetryWait,
						},
					},
					Parallel: pointer.Ptr(repairTaskInitialParallel),
				},
			},
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Repair'")
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

		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
		o.Expect(err).NotTo(o.HaveOccurred())
		smcr, err := controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(controlPlaneNS.Name).Get(ctx, smcrName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(smcr.Status.ClusterID).NotTo(o.BeNil())
		o.Expect(*smcr.Status.ClusterID).NotTo(o.BeEmpty())
		managerClusterID := *smcr.Status.ClusterID

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that ScyllaDBManagerTask was registered with global ScyllaDB Manager")
		managerTask, err := managerClient.GetTask(ctx, managerClusterID, managerclient.RepairTask, managerTaskID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerTask.Labels).NotTo(o.BeNil())
		o.Expect(managerTask.Labels[naming.OwnerUIDLabel]).To(o.Equal(string(smt.UID)))

		framework.By("Verifying that ScyllaDBManagerTask properties were propagated to ScyllaDB Manager state")
		o.Expect(managerTask.Schedule).NotTo(o.BeNil())
		o.Expect(managerTask.Schedule.NumRetries).To(o.Equal(*smt.Spec.Repair.NumRetries))
		o.Expect(managerTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(repairTaskInitialParallel))

		framework.By("Waiting for the repair task to finish")
		repairTaskCompletionCtx, repairTaskCompletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerMultiDatacenterTaskCompletionTimeout,
			fmt.Errorf("repair task %q has not completed in time", managerTaskID),
		)
		defer repairTaskCompletionCtxCancel()

		o.Eventually(verification.VerifyScyllaDBManagerRepairTaskCompleted).
			WithContext(repairTaskCompletionCtx).
			WithPolling(5*time.Second).
			WithArguments(managerClient, managerClusterID, managerTask.ID).
			Should(o.Succeed())

		// The repair covers the whole ring, so by now ScyllaDB Manager has talked to the agent of every node of every
		// datacenter using the shared auth token.
		framework.By("Verifying that global ScyllaDB Manager can reach every node of the multi-datacenter cluster")
		verifyManagerClusterStatusHealthy(ctx, managerClient, managerClusterID, datacenters)

		framework.By("Updating the ScyllaDBManagerTask")
		updatedSMT, err := controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(controlPlaneNS.Name).Patch(
			ctx,
			smt.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op":"replace","path":"/spec/repair/parallel","value":%d}]`, repairTaskUpdatedParallel)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(updatedSMT.Spec.Repair).NotTo(o.BeNil())
		o.Expect(updatedSMT.Spec.Repair.Parallel).NotTo(o.BeNil())
		o.Expect(*updatedSMT.Spec.Repair.Parallel).To(o.Equal(repairTaskUpdatedParallel))

		framework.By("Waiting for ScyllaDBManagerTask update to be reconciled")
		updateCtx, updateCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("ScyllaDBManagerTask %q update has not been reconciled in time", naming.ObjRef(updatedSMT)),
		)
		defer updateCtxCancel()

		updatedSMT, err = controllerhelpers.WaitForScyllaDBManagerTaskState(
			updateCtx,
			controlPlaneNSClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(controlPlaneNS.Name),
			updatedSMT.Name,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerTaskRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that the ScyllaDBManagerTask update propagated to ScyllaDB Manager state")
		updatePropagationCtx, updatePropagationCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("ScyllaDBManagerTask %q update has not propagated to global ScyllaDB Manager instance in time", naming.ObjRef(updatedSMT)),
		)
		defer updatePropagationCtxCancel()

		updatedManagerTask, err := managerClient.GetTask(updatePropagationCtx, managerClusterID, managerclient.RepairTask, managerTaskID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(updatedManagerTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(repairTaskUpdatedParallel))

		framework.By("Waiting for repair with updated properties to finish")
		updatedRepairTaskCompletionCtx, updatedRepairTaskCompletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerMultiDatacenterTaskCompletionTimeout,
			fmt.Errorf("repair task %q has not completed in time after property update", managerTaskID),
		)
		defer updatedRepairTaskCompletionCtxCancel()

		o.Eventually(verification.VerifyScyllaDBManagerRepairTaskCompleted).
			WithContext(updatedRepairTaskCompletionCtx).
			WithPolling(5*time.Second).
			WithArguments(managerClient, managerClusterID, updatedManagerTask.ID).
			Should(o.Succeed())

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
		deletionCtx, deletionCtxCancel := context.WithTimeoutCause(
			ctx,
			utils.ScyllaDBManagerTaskSyncTimeout,
			fmt.Errorf("ScyllaDBManagerTask %q has not been deleted in time", naming.ObjRef(smt)),
		)
		defer deletionCtxCancel()

		err = framework.WaitForObjectDeletion(
			deletionCtx,
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
		tasks, err := managerClient.ListTasks(ctx, managerClusterID, managerclient.RepairTask, false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(slices.ContainsFunc(tasks.TaskListItemSlice, func(t *managerclient.TaskListItem) bool {
			return t.ID == managerTaskID.String()
		})).To(o.BeFalse())
	})
})
