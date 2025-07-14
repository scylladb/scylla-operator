// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"encoding/json"
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
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("ScyllaDBManagerTask integration with global ScyllaDB Manager", func() {
	f := framework.NewFramework("scylladbmanagertask")

	g.It("should synchronise a repair task for ScyllaDBDatacenter", func(ctx g.SpecContext) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sdc := f.GetDefaultScyllaDBDatacenter()
		metav1.SetMetaDataLabel(&sdc.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		framework.By(`Creating a ScyllaDBDatacenter with the global ScyllaDB Manager registration label`)
		sdc, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utilsv1alpha1.ContextForScyllaDBDatacenterRollout(ctx, sdc)
		defer rolloutCtxCancel()
		sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(rolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sdc.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sdc)
		scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sdc)

		hosts, err := utilsv1alpha1.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sdc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		smt := &scyllav1alpha1.ScyllaDBManagerTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repair",
				Namespace: ns.Name,
			},
			Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
				ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
					Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
					Name: sdc.Name,
				},
				Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
				Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
					ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
						NumRetries: pointer.Ptr[int64](1),
					},
					Parallel: pointer.Ptr[int64](2),
				},
			},
		}

		framework.By("Creating a ScyllaDBManagerTask of type 'Repair'")
		smt, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerTasks(ns.Name).Create(
			ctx,
			smt,
			metav1.CreateOptions{},
		)
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
			scyllaDBManagerTaskHasDeletionFinalizer,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(smt.Status.TaskID).NotTo(o.BeNil())
		o.Expect(*smt.Status.TaskID).NotTo(o.BeEmpty())
		managerTaskID, err := uuid.Parse(*smt.Status.TaskID)
		o.Expect(err).NotTo(o.HaveOccurred())

		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
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
		err = apimachineryutilwait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(context.Context) (done bool, err error) {
			repairProgress, err := managerClient.RepairProgress(ctx, managerClusterID, managerTask.ID, "latest")
			if err != nil {
				return false, err
			}

			return repairProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

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
			f.DynamicClient(),
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

func scyllaDBManagerTaskHasDeletionFinalizer(smt *scyllav1alpha1.ScyllaDBManagerTask) (bool, error) {
	return slices.Contains(smt.Finalizers, naming.ScyllaDBManagerTaskFinalizer), nil
}
