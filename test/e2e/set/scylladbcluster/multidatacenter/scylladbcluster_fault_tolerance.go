package multidatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("Multi datacenter ScyllaDBCluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should reconcile healthy datacenters when any of DCs are down", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		rkcs, rkcClusterMap, err := utils.SetUpRemoteKubernetesClustersFromRestConfigs(ctx, framework.TestContext.RestConfigs, f)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(len(rkcs)).To(o.BeNumerically(">=", 2))

		framework.By("Creating ScyllaDBCluster")
		sc := f.GetDefaultScyllaDBCluster(rkcs)
		metaCluster := f.Cluster(0)
		sc, err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx2, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Removing Operator access to remote Kubernetes cluster associated with last DC")
		lastDCRKC := rkcs[len(rkcs)-1]

		lastDCRKCKubeconfigSecret, err := metaCluster.KubeAdminClient().CoreV1().Secrets(lastDCRKC.Spec.KubeconfigSecretRef.Namespace).Get(ctx, lastDCRKC.Spec.KubeconfigSecretRef.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		const kubeconfigKey = "kubeconfig"

		o.Expect(lastDCRKCKubeconfigSecret.Data).To(o.HaveKey(kubeconfigKey))
		o.Expect(lastDCRKCKubeconfigSecret.Data[kubeconfigKey]).ToNot(o.BeEmpty())
		lastDCRKCWorkingKubeconfig := lastDCRKCKubeconfigSecret.Data[kubeconfigKey]

		lastDCRKCKubeconfigSecret.Data[kubeconfigKey] = scyllafixture.UnauthorizedKubeconfigBytes
		lastDCRKCKubeconfigSecret, err = metaCluster.KubeAdminClient().CoreV1().Secrets(lastDCRKCKubeconfigSecret.Namespace).Update(ctx, lastDCRKCKubeconfigSecret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scaling the ScyllaDBCluster up to create a new node in each datacenter")
		oldNodes := *sc.Spec.DatacenterTemplate.RackTemplate.Nodes
		newNodes := oldNodes + 1
		sc, err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenterTemplate/rackTemplate/nodes", "value": %d}]`,
				newNodes,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.DatacenterTemplate.RackTemplate.Nodes).ToNot(o.BeNil())
		o.Expect(*sc.Spec.DatacenterTemplate.RackTemplate.Nodes).To(o.Equal(newNodes))

		framework.By("Awaiting until ScyllaDBCluster is degraded and healthy DCs are rolled out")
		waitCtx3, waitCtx3Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx3, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{},
			utils.IsScyllaDBClusterDegraded,
			func() []func(cluster *scyllav1alpha1.ScyllaDBCluster) (bool, error) {
				lastDCIdx := len(sc.Spec.Datacenters) - 1
				var conditions []func(cluster *scyllav1alpha1.ScyllaDBCluster) (bool, error)
				for i := 0; i < lastDCIdx; i++ {
					conditions = append(conditions, utils.IsScyllaDBClusterDatacenterRolledOutFunc(&sc.Spec.Datacenters[i]))
				}
				conditions = append(conditions, utils.IsScyllaDBClusterDatacenterDegradedFunc(&sc.Spec.Datacenters[lastDCIdx]))
				return conditions
			}()...,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Fixing Operator access to remote Kubernetes cluster associated with last DC")
		lastDCRKCKubeconfigSecret.Data[kubeconfigKey] = lastDCRKCWorkingKubeconfig
		lastDCRKCKubeconfigSecret, err = metaCluster.KubeAdminClient().CoreV1().Secrets(lastDCRKCKubeconfigSecret.Namespace).Update(ctx, lastDCRKCKubeconfigSecret, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx4, waitCtx4Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx4, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
