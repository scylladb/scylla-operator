// Copyright (C) 2025 ScyllaDB

package multidatacenter

import (
	"context"
	"encoding/json"
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
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaDBManagerTask and ScyllaDBCluster integration with global ScyllaDB Manager", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbmanagertask")

	g.It("should synchronise a repair task", func(ctx g.SpecContext) {
		ns, nsClient := f.CreateUserNamespace(ctx)

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := f.GetDefaultScyllaDBCluster(rkcMap)
		metav1.SetMetaDataLabel(&sc.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		framework.By(`Creating a ScyllaDBCluster with the global ScyllaDB Manager registration label`)
		sc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sc, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer rolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(rolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		hostsByDC, err := utilsv1alpha1.GetBroadcastRPCAddressesForScyllaDBCluster(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		allHosts := slices.Concat(slices.Collect(maps.Values(hostsByDC))...)
		o.Expect(allHosts).To(o.HaveLen(int(controllerhelpers.GetScyllaDBClusterNodeCount(sc))))

		di := verification.InsertAndVerifyCQLDataByDC(ctx, hostsByDC)
		defer di.Close()

		smt := &scyllav1alpha1.ScyllaDBManagerTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repair",
				Namespace: ns.Name,
			},
			Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
				ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
					Kind: scyllav1alpha1.ScyllaDBClusterGVK.Kind,
					Name: sc.Name,
				},
				Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
				Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
					ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
						NumRetries: pointer.Ptr[int64](utils.ScyllaDBManagerTaskNumRetries),
						RetryWait: &metav1.Duration{
							Duration: utils.ScyllaDBManagerTaskRetryWait,
						},
					},
					Parallel: pointer.Ptr[int64](2),
				},
			},
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Repair'")
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Create(ctx, smt, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBManagerTask to register with global ScyllaDB Manager instance")
		registrationCtx, registrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer registrationCtxCancel()
		smt, err = controllerhelpers.WaitForScyllaDBManagerTaskState(
			registrationCtx,
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

		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		smcr, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns.Name).Get(ctx, smcrName, metav1.GetOptions{})
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
		o.Expect(managerTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(*smt.Spec.Repair.Parallel))

		framework.By("Updating the ScyllaDBManagerTask")
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Patch(
			ctx,
			smt.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace","path":"/spec/repair/parallel","value":1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(smt.Spec.Repair).NotTo(o.BeNil())
		o.Expect(smt.Spec.Repair.Parallel).NotTo(o.BeNil())
		o.Expect(*smt.Spec.Repair.Parallel).To(o.Equal(int64(1)))

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
		managerTask, err = managerClient.GetTask(updatePropagationCtx, managerClusterID, managerclient.RepairTask, managerTaskID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(*smt.Spec.Repair.Parallel))

		framework.By("Waiting for the repair task to finish")
		o.Eventually(verification.VerifyScyllaDBManagerRepairTaskCompleted).
			WithContext(ctx).
			WithTimeout(utils.ScyllaDBManagerMultiDatacenterTaskCompletionTimeout).
			WithPolling(5*time.Second).
			WithArguments(managerClient, managerClusterID, managerTask.ID).
			Should(o.Succeed())

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
		deletionCtx, deletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
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
		tasks, err := managerClient.ListTasks(ctx, managerClusterID, managerclient.RepairTask, false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(slices.ContainsFunc(tasks.TaskListItemSlice, func(t *managerclient.TaskListItem) bool {
			return t.ID == managerTaskID.String()
		})).To(o.BeFalse())
	})
})
