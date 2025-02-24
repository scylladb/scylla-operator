// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1alpha1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = g.Describe("ScyllaDBCluster finalizer", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should delete remote Namespaces when ScyllaDBCluster is deleted", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		availableClusters := len(framework.TestContext.RestConfigs)

		framework.By("Creating RemoteKubernetesClusters")
		rkcs := make([]*scyllav1alpha1.RemoteKubernetesCluster, 0, availableClusters)
		rkcClusterMap := make(map[string]framework.ClusterInterface, availableClusters)

		metaCluster := f.Cluster(0)
		for idx := range availableClusters {
			cluster := f.Cluster(idx)
			userNs, _ := cluster.CreateUserNamespace(ctx)

			clusterName := fmt.Sprintf("%s-%d", f.Namespace(), idx)

			framework.By("Creating SA having Operator ClusterRole in #%d cluster", idx)
			adminKubeconfig, err := utils.GetKubeConfigHavingOperatorRemoteClusterRole(ctx, cluster.KubeAdminClient(), cluster.AdminClientConfig(), clusterName, userNs.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			kubeconfig, err := clientcmd.Write(adminKubeconfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			rkc, err := utils.GetRemoteKubernetesClusterWithKubeconfig(ctx, metaCluster.KubeAdminClient(), kubeconfig, clusterName, f.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())

			rc := framework.NewRestoringCleaner(
				ctx,
				f.KubeAdminClient(),
				f.DynamicAdminClient(),
				remoteKubernetesClusterResourceInfo,
				rkc.Namespace,
				rkc.Name,
				framework.RestoreStrategyRecreate,
			)
			f.AddCleaners(rc)
			rc.DeleteObject(ctx, true)

			framework.By("Creating RemoteKubernetesCluster %q with credentials to cluster #%d", clusterName, idx)
			rkc, err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Create(ctx, rkc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			rkcs = append(rkcs, rkc)
			rkcClusterMap[rkc.Name] = cluster
		}

		for _, rkc := range rkcs {
			func() {
				framework.By("Waiting for the RemoteKubernetesCluster %q to roll out (RV=%s)", rkc.Name, rkc.ResourceVersion)
				waitCtx1, waitCtx1Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
				defer waitCtx1Cancel()

				_, err := controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx1, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsRemoteKubernetesClusterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())
			}()
		}

		framework.By("Creating ScyllaDBCluster")
		var err error
		sc := f.GetDefaultScyllaDBCluster(rkcs)
		sc, err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx2, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDBCluster(ctx, sc, rkcClusterMap)
		err = v1alpha1utils.WaitForFullScyllaDBClusterQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		const expectedFinalizer = "scylla-operator.scylladb.com/scylladbcluster-protection"
		o.Expect(sc.Finalizers).To(o.ContainElement(expectedFinalizer))

		framework.By("Validating there are remote Namespaces matching ScyllaDBCluster selector")
		o.Expect(rkcs).NotTo(o.BeEmpty())
		for i := range rkcs {
			namespaces, err := f.Cluster(i).KubeAdminClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{
				LabelSelector: naming.ScyllaDBClusterSelector(sc).String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(namespaces.Items).NotTo(o.BeEmpty())
		}

		framework.By("Deleting ScyllaDBCluster")
		err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Delete(ctx, sc.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDBCluster %q to be removed.", sc.Name)
		err = framework.WaitForObjectDeletion(ctx, f.DynamicClient(), scyllav1alpha1.SchemeGroupVersion.WithResource("scylladbclusters"), sc.Namespace, sc.Name, &sc.UID)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying if all remote Namespaces having ScyllaDBCluster %q selector are gone.", sc.Name)
		o.Expect(sc.Spec.Datacenters).ToNot(o.BeEmpty())
		for i := range rkcs {
			namespaces, err := f.Cluster(i).KubeAdminClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{
				LabelSelector: naming.ScyllaDBClusterSelector(sc).String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(namespaces.Items).To(o.BeEmpty())
		}
	})
})
