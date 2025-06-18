package multidatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Multi datacenter ScyllaDBCluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should boostrap all datacenters and form single cluster", func() {
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

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx2, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sc, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
