// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("Scylla Manager integration", func() {
	f := framework.NewFramework("scyllacluster")

	g.DescribeTable("should register and deregister a ScyllaCluster", func(ctx g.SpecContext, deregistrationHook func(context.Context, *scyllav1.ScyllaCluster)) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sc := f.GetDefaultScyllaCluster()

		framework.By(`Creating a ScyllaCluster`)
		sc, err := nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(ns.Name).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer rolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(rolloutCtx, nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, nsClient.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Waiting for ScyllaDB Manager cluster ID to propagate to ScyllaCluster's status")
		registrationCtx, registrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer registrationCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(
			registrationCtx,
			nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace),
			sc.Name,
			controllerhelpers.WaitForStateOptions{},
			utils.IsScyllaClusterRegisteredWithManager,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.ManagerID).NotTo(o.BeNil())
		managerClusterID := *sc.Status.ManagerID
		o.Expect(managerClusterID).NotTo(o.BeEmpty())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that ScyllaCluster is registered with ScyllaDB Manager")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := managerClient.GetCluster(ctx, managerClusterID)
			eo.Expect(err).NotTo(o.HaveOccurred())
		}).
			WithContext(ctx).
			WithTimeout(utils.SyncTimeout).
			WithPolling(5 * time.Second).
			Should(o.Succeed())

		deregistrationHook(ctx, sc)

		framework.By("Verifying that the cluster was removed from the global ScyllaDB Manager state")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := managerClient.GetCluster(ctx, managerClusterID)
			eo.Expect(err).To(o.HaveOccurred())
			eo.Expect(err).To(o.Satisfy(managerclienterrors.IsNotFound))
		}).
			WithContext(ctx).
			WithTimeout(utils.SyncTimeout).
			WithPolling(5 * time.Second).
			Should(o.Succeed())
	},
		g.Entry("when manager integration is disabled", func(ctx context.Context, sc *scyllav1.ScyllaCluster) {
			framework.By(`Annotating ScyllaCluster to disable ScyllaDB Manager integration`)
			scCopy := sc.DeepCopy()
			metav1.SetMetaDataAnnotation(&scCopy.ObjectMeta, naming.DisableGlobalScyllaDBManagerIntegrationAnnotation, naming.LabelValueTrue)

			patch, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
			o.Expect(err).NotTo(o.HaveOccurred())

			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Patch(
				ctx,
				sc.Name,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}),
		g.Entry("when is deleted", func(ctx context.Context, sc *scyllav1.ScyllaCluster) {
			framework.By("Deleting ScyllaCluster")
			err := f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Delete(
				ctx,
				sc.Name,
				metav1.DeleteOptions{
					PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
					Preconditions: &metav1.Preconditions{
						UID: &sc.UID,
					},
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting for ScyllaCluster to be deleted")
			deletionCtx, deletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
			defer deletionCtxCancel()
			err = framework.WaitForObjectDeletion(
				deletionCtx,
				f.DynamicClient(),
				scyllav1.GroupVersion.WithResource("scyllaclusters"),
				sc.Namespace,
				sc.Name,
				&sc.UID,
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}),
	)

	type repairTaskTableEntry struct {
		taskDeletionHook             func(context.Context, *scyllav1.ScyllaCluster)
		managerStateVerificationHook func(context.Context, *managerclient.Client, string)
	}

	g.DescribeTable("should register cluster, sync repair tasks, and delete them", func(ctx g.SpecContext, e repairTaskTableEntry) {
		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

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
						return false, errors.New(*r.Error)
					}

					return true, nil
				}
			}
			return false, nil
		}

		taskSchedulingCtx, taskSchedulingCtxCancel := utils.ContextForManagerSync(ctx, sc)
		defer taskSchedulingCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(taskSchedulingCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskScheduledCond)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].Parallel).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))
		o.Expect(sc.Status.Backups).To(o.BeEmpty())

		// Sanity check to avoid panics.
		o.Expect(sc.Status.ManagerID).NotTo(o.BeNil())
		managerClusterID := *sc.Status.ManagerID
		o.Expect(managerClusterID).NotTo(o.BeEmpty())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that repair task status was synchronized")
		tasks, err := managerClient.ListTasks(ctx, managerClusterID, "repair", false, "", "")
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
						return false, errors.New(*r.Error)
					}

					return r.Parallel != nil && *r.Parallel == int64(1), nil
				}
			}
			return false, nil
		}

		taskUpdateCtx, taskUpdateCtxCancel := utils.ContextForManagerSync(ctx, sc)
		defer taskUpdateCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(taskUpdateCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskUpdatedCond)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Status.Repairs).To(o.HaveLen(1))
		o.Expect(sc.Status.Repairs[0].Name).To(o.Equal(sc.Spec.Repairs[0].Name))
		o.Expect(sc.Status.Repairs[0].ID).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].ID).To(o.Equal(repairTask.ID))
		o.Expect(sc.Status.Repairs[0].Parallel).NotTo(o.BeNil())
		o.Expect(*sc.Status.Repairs[0].Parallel).To(o.Equal(sc.Spec.Repairs[0].Parallel))
		o.Expect(sc.Status.Backups).To(o.BeEmpty())

		framework.By("Verifying that updated repair task status was synchronized")
		tasks, err = managerClient.ListTasks(ctx, managerClusterID, "repair", false, "", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.TaskListItemSlice).To(o.HaveLen(1))
		repairTask = tasks.TaskListItemSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sc.Status.Repairs[0].Name))
		o.Expect(repairTask.ID).To(o.Equal(*sc.Status.Repairs[0].ID))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(*sc.Status.Repairs[0].Parallel))

		framework.By("Waiting for repair to finish")
		err = apimachineryutilwait.PollUntilContextTimeout(ctx, 5*time.Second, 10*time.Minute, true, func(context.Context) (done bool, err error) {
			repairProgress, err := managerClient.RepairProgress(ctx, managerClusterID, repairTask.ID, "latest")
			if err != nil {
				return false, err
			}

			return repairProgress.Run.Status == managerclient.TaskStatusDone, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		e.taskDeletionHook(ctx, sc)

		framework.By("Waiting for ScyllaCluster to sync repair task deletion with Scylla Manager")
		repairTaskDeletedCond := func(cluster *scyllav1.ScyllaCluster) (bool, error) {
			return len(cluster.Status.Repairs) == 0, nil
		}

		taskDeletionCtx, taskDeletionCtxCancel := utils.ContextForManagerSync(ctx, sc)
		defer taskDeletionCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(taskDeletionCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, repairTaskDeletedCond)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Progressing conditions of the underlying ScyllaDBManagerTasks are not propagated to ScyllaClusters by the migration controller,
		// neither does the controller await the deletion of ScyllaDBManagerTasks.
		// Task deletion from ScyllaDB Manager state is asynchronous to ScyllaCluster state update.
		e.managerStateVerificationHook(ctx, managerClient, managerClusterID)
	},
		g.Entry("when manager integration is disabled", repairTaskTableEntry{
			taskDeletionHook: func(ctx context.Context, sc *scyllav1.ScyllaCluster) {
				framework.By(`Annotating ScyllaCluster to disable ScyllaDB Manager integration`)
				scCopy := sc.DeepCopy()
				metav1.SetMetaDataAnnotation(&scCopy.ObjectMeta, naming.DisableGlobalScyllaDBManagerIntegrationAnnotation, naming.LabelValueTrue)

				patch, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
				o.Expect(err).NotTo(o.HaveOccurred())

				sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Patch(
					ctx,
					sc.Name,
					types.MergePatchType,
					patch,
					metav1.PatchOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())
			},
			managerStateVerificationHook: func(ctx context.Context, managerClient *managerclient.Client, managerClusterID string) {
				framework.By("Verifying that the cluster was removed from the manager state")
				o.Eventually(func(eo o.Gomega, ctx context.Context) {
					_, err := managerClient.GetCluster(ctx, managerClusterID)
					eo.Expect(err).To(o.HaveOccurred())
					eo.Expect(err).To(o.Satisfy(managerclienterrors.IsNotFound))
				}).
					WithContext(ctx).
					WithTimeout(utils.SyncTimeout).
					WithPolling(5 * time.Second).
					Should(o.Succeed())
			},
		}),
		g.Entry("when repair task is deleted", repairTaskTableEntry{
			taskDeletionHook: func(ctx context.Context, sc *scyllav1.ScyllaCluster) {
				framework.By("Deleting the repair task")
				sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
					ctx,
					sc.Name,
					types.JSONPatchType,
					[]byte(`[{"op":"remove","path":"/spec/repairs/0"}]`),
					metav1.PatchOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(sc.Spec.Repairs).To(o.BeEmpty())
			},
			managerStateVerificationHook: func(ctx context.Context, managerClient *managerclient.Client, managerClusterID string) {
				framework.By("Verifying that the repair task was removed from the manager state")
				o.Eventually(func(eo o.Gomega, ctx context.Context) {
					tasks, err := managerClient.ListTasks(ctx, managerClusterID, "repair", false, "", "")
					eo.Expect(err).NotTo(o.HaveOccurred())
					eo.Expect(tasks.TaskListItemSlice).To(o.BeEmpty())
				}).
					WithContext(ctx).
					WithTimeout(utils.SyncTimeout).
					WithPolling(5 * time.Second).
					Should(o.Succeed())
			},
		}),
	)
})
