// Copyright (c) 2024 ScyllaDB.

package multidatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaDBCluster finalizer", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should delete remote Namespaces when ScyllaDBCluster is deleted", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating ScyllaDBCluster")
		sc := f.GetDefaultScyllaDBCluster(rkcMap)
		sc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sc, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx2, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbcluster.Verify(ctx, sc, rkcClusterMap)
		err = scylladbcluster.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		const expectedFinalizer = "scylla-operator.scylladb.com/scylladbcluster-protection"
		o.Expect(sc.Finalizers).To(o.ContainElement(expectedFinalizer))

		framework.By("Validating there are remote Namespaces matching ScyllaDBCluster selector")
		o.Expect(rkcClusterMap).NotTo(o.BeEmpty())
		for _, rkcCluster := range rkcClusterMap {
			namespaces, err := rkcCluster.KubeAdminClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{
				LabelSelector: naming.ScyllaDBClusterSelector(sc).String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(namespaces.Items).NotTo(o.BeEmpty())
		}

		framework.By("Deleting ScyllaDBCluster")
		err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Delete(ctx, sc.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBCluster %q to be removed.", sc.Name)
		err = framework.WaitForObjectDeletion(ctx, f.DynamicClient(), scyllav1alpha1.SchemeGroupVersion.WithResource("scylladbclusters"), sc.Namespace, sc.Name, &sc.UID)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying if all remote Namespaces having ScyllaDBCluster %q selector are gone.", sc.Name)
		o.Expect(sc.Spec.Datacenters).ToNot(o.BeEmpty())
		for _, rkcCluster := range rkcClusterMap {
			namespaces, err := rkcCluster.KubeAdminClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{
				LabelSelector: naming.ScyllaDBClusterSelector(sc).String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(namespaces.Items).To(o.BeEmpty())
		}
	})
})
