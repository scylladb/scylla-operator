// Copyright (C) 2025 ScyllaDB

package scylladbdatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaDBDatacenter integration with global ScyllaDB Manager", func() {
	f := framework.NewFramework("scylladbdatacenter")

	hasDeletionFinalizer := func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (bool, error) {
		return slices.ContainsItem(smcr.Finalizers, naming.ScyllaDBManagerClusterRegistrationFinalizer), nil
	}

	g.It("should register labeled ScyllaDBDatacenter and deregister it when it's unlabeled", func(ctx g.SpecContext) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sdc := f.GetDefaultScyllaDBDatacenter()
		metav1.SetMetaDataLabel(&sdc.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		framework.By(`Creating a ScyllaDBDatacenter with the global ScyllaDB Manager registration label`)
		sdc, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
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

		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to register with global ScyllaDB Manager instance")
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

		framework.By("Verifying that ScyllaDBDatacenter was registered with global ScyllaDB Manager")
		managerCluster, err := managerClient.GetCluster(ctx, *smcr.Status.ClusterID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerCluster.Labels).NotTo(o.BeNil())
		o.Expect(managerCluster.Labels[naming.OwnerUIDLabel]).To(o.Equal(string(smcr.UID)))

		framework.By(`Removing the global ScyllaDB Manager registration label from ScyllaDBDatacenter`)
		sdcCopy := sdc.DeepCopy()
		delete(sdcCopy.Labels, naming.GlobalScyllaDBManagerRegistrationLabel)

		patch, err := controllerhelpers.GenerateMergePatch(sdc, sdcCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		sdc, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Patch(
			ctx,
			sdc.Name,
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By(`Waiting for ScyllaDBManagerClusterRegistration to be deleted`)
		deletionCtx, deletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
			f.DynamicClient(),
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
	})

	g.It("should register labeled ScyllaDBDatacenter and deregister it when it's deleted", func(ctx g.SpecContext) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sdc := f.GetDefaultScyllaDBDatacenter()
		metav1.SetMetaDataLabel(&sdc.ObjectMeta, naming.GlobalScyllaDBManagerRegistrationLabel, naming.LabelValueTrue)

		framework.By(`Creating a ScyllaDBDatacenter with the global ScyllaDB Manager registration label`)
		sdc, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
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

		smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBDatacenter to register with global ScyllaDB Manager instance")
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

		framework.By("Verifying that ScyllaDBDatacenter was registered with global ScyllaDB Manager")
		managerCluster, err := managerClient.GetCluster(ctx, *smcr.Status.ClusterID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(managerCluster.Labels).NotTo(o.BeNil())
		o.Expect(managerCluster.Labels[naming.OwnerUIDLabel]).To(o.Equal(string(smcr.UID)))

		framework.By(`Deleting ScyllaDBDatacenter`)
		err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Delete(
			ctx,
			sdc.Name,
			metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
				Preconditions: &metav1.Preconditions{
					UID: &sdc.UID,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By(`Waiting for ScyllaDBManagerClusterRegistration to be deleted`)
		deletionCtx, deletionCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
			f.DynamicClient(),
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
	})
})
