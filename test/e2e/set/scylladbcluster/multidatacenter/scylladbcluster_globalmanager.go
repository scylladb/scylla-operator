// Copyright (C) 2025 ScyllaDB

package multidatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaDBCluster integration with global ScyllaDB Manager", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	hasDeletionFinalizer := func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (bool, error) {
		return oslices.ContainsItem(smcr.Finalizers, naming.ScyllaDBManagerClusterRegistrationFinalizer), nil
	}

	g.DescribeTable("should register labeled ScyllaDBCluster and deregister it", func(ctx g.SpecContext, deregistrationHook func(context.Context, *scyllav1alpha1.ScyllaDBCluster)) {
		ns, nsClient := f.CreateUserNamespace(ctx)

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By(`Creating a ScyllaDBCluster with the global ScyllaDB Manager registration label`)
		sc := f.GetDefaultScyllaDBCluster(rkcMap)
		metav1.SetMetaDataLabel(&sc.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

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

		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBCluster to register with global ScyllaDB Manager instance")
		registrationCtx, registrationCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer registrationCtxCancel()
		smcr, err := controllerhelpers.WaitForScyllaDBManagerClusterRegistrationState(
			registrationCtx,
			nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns.Name),
			smcrName,
			controllerhelpers.WaitForStateOptions{},
			utilsv1alpha1.IsScyllaDBManagerClusterRegistrationRolledOut,
			hasDeletionFinalizer,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(smcr.Status.ClusterID).NotTo(o.BeNil())
		o.Expect(*smcr.Status.ClusterID).NotTo(o.BeEmpty())
		managerClusterID := *smcr.Status.ClusterID

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that ScyllaDBCluster was registered with global ScyllaDB Manager")
		managerCluster, err := managerClient.GetCluster(ctx, managerClusterID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerCluster.Labels).NotTo(o.BeNil())
		o.Expect(managerCluster.Labels[naming.OwnerUIDLabel]).To(o.Equal(string(smcr.UID)))

		deregistrationHook(ctx, sc)

		framework.By(`Waiting for ScyllaDBManagerClusterRegistration to be deleted`)
		deletionCtx, deletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
			nsClient.DynamicClient(),
			scyllav1alpha1.GroupVersion.WithResource("scylladbmanagerclusterregistrations"),
			smcr.Namespace,
			smcr.Name,
			pointer.Ptr(smcr.UID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that the cluster was removed from the global ScyllaDB Manager state")
		_, err = managerClient.GetCluster(ctx, managerClusterID)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err).To(o.Satisfy(managerclienterrors.IsNotFound))
	},
		g.Entry("when unlabeled", func(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster) {
			framework.By(`Removing the global ScyllaDB Manager registration label from ScyllaDBCluster`)
			scCopy := sc.DeepCopy()
			delete(scCopy.Labels, naming.GlobalScyllaDBManagerRegistrationLabel)

			patch, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
			o.Expect(err).NotTo(o.HaveOccurred())

			sc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Patch(
				ctx,
				sc.Name,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}),
		g.Entry("when deleted", func(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster) {
			framework.By(`Deleting ScyllaDBCluster`)
			err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Delete(
				ctx,
				sc.Name,
				metav1.DeleteOptions{
					PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
					Preconditions: &metav1.Preconditions{
						UID: &sc.UID,
					},
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}),
	)
})
