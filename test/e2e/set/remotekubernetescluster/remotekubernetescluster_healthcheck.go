package remotekubernetescluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = g.Describe("RemoteKubernetesCluster", func() {
	f := framework.NewFramework("remotekubernetescluster")

	var (
		rkcs          []*scyllav1alpha1.RemoteKubernetesCluster
		rkcClusterMap map[string]framework.ClusterInterface
	)

	g.JustBeforeEach(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testSetupTimeout)
		defer cancel()

		metaCluster := f.Cluster()
		// Use the meta cluster to simulate the remote clusters.
		availableClusters := map[string]framework.ClusterInterface{metaCluster.Name(): metaCluster}
		rkcs = make([]*scyllav1alpha1.RemoteKubernetesCluster, 0, len(availableClusters))
		rkcClusterMap = make(map[string]framework.ClusterInterface, len(availableClusters))

		for _, cluster := range availableClusters {
			userNs, _ := cluster.CreateUserNamespace(ctx)

			rkcName := cluster.Name()
			framework.By("Creating RemoteKubernetesCluster %q with credentials to cluster %q", rkcName, cluster.Name())
			rkc, err := utils.GetRemoteKubernetesClusterWithOperatorClusterRole(ctx, cluster.KubeAdminClient(), cluster.AdminClientConfig(), rkcName, userNs.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			rkc, err = cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Create(ctx, rkc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			rkcs = append(rkcs, rkc)
			rkcClusterMap[rkc.Name] = cluster
		}
	})

	g.JustAfterEach(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTeardownTimeout)
		defer cancel()

		for _, rkc := range rkcs {
			rkcCluster, ok := rkcClusterMap[rkc.Name]
			// Sanity check
			o.Expect(ok).To(o.BeTrue())

			// The framework does not clean non-namespaced resources.
			err := rkcCluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Delete(ctx, rkc.Name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.It("should run healthcheck probes and propagate results to status conditions", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		for _, rkc := range rkcs {
			framework.By("Waiting for the RemoteKubernetesCluster %q to roll out (RV=%s)", rkc.Name, rkc.ResourceVersion)
			waitCtx1, waitCtx1Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
			defer waitCtx1Cancel()

			var err error
			rkc, err = controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx1, f.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{},
				utils.IsRemoteKubernetesClusterRolledOut,
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Breaking access to remote Kubernetes cluster")
			kubeconfig, err := utils.GetKubeConfigHavingOperatorRemoteClusterRole(ctx, f.KubeAdminClient(), f.AdminClientConfig(), rkc.Name, rkc.Spec.KubeconfigSecretRef.Namespace)
			o.Expect(err).NotTo(o.HaveOccurred())

			validToken := kubeconfig.AuthInfos[kubeconfig.CurrentContext].Token

			kubeconfig.AuthInfos[kubeconfig.CurrentContext].Token = "foo"

			noAccessKubeconfig, err := clientcmd.Write(kubeconfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			rkcSecret, err := f.KubeAdminClient().CoreV1().Secrets(rkc.Spec.KubeconfigSecretRef.Namespace).Get(ctx, rkc.Spec.KubeconfigSecretRef.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = f.KubeAdminClient().CoreV1().Secrets(rkcSecret.Namespace).Patch(
				ctx,
				rkcSecret.Name,
				types.MergePatchType,
				[]byte(fmt.Sprintf(`{"stringData": {"kubeconfig": %q } }`, string(noAccessKubeconfig))),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Awaiting until RemoteKubernetesCluster %q becomes unavailable", rkc.Name)
			waitCtx2, waitCtx2Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
			defer waitCtx2Cancel()
			rkc, err = controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx2, f.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{},
				func(rkc *scyllav1alpha1.RemoteKubernetesCluster) (bool, error) {
					notAvailable := helpers.IsStatusConditionPresentAndFalse(rkc.Status.Conditions, scyllav1alpha1.AvailableCondition, rkc.Generation)
					notProgressing := helpers.IsStatusConditionPresentAndFalse(rkc.Status.Conditions, scyllav1alpha1.ProgressingCondition, rkc.Generation)
					notDegraded := helpers.IsStatusConditionPresentAndFalse(rkc.Status.Conditions, scyllav1alpha1.DegradedCondition, rkc.Generation)
					return notAvailable && notProgressing && notDegraded, nil
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Restoring access to remote Kubernetes cluster")
			kubeconfig.AuthInfos[kubeconfig.CurrentContext].Token = validToken

			fixedKubeconfig, err := clientcmd.Write(kubeconfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = f.KubeAdminClient().CoreV1().Secrets(rkcSecret.Namespace).Patch(
				ctx,
				rkcSecret.Name,
				types.MergePatchType,
				[]byte(fmt.Sprintf(`{"stringData": {"kubeconfig": %q } }`, string(fixedKubeconfig))),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting for the RemoteKubernetesCluster %q to roll out (RV=%s)", rkc.Name, rkc.ResourceVersion)
			waitCtx3, waitCtx3Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
			defer waitCtx3Cancel()

			rkc, err = controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx3, f.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{},
				utils.IsRemoteKubernetesClusterRolledOut,
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
