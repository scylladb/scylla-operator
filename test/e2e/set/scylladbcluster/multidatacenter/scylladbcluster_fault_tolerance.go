package multidatacenter

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1alpha1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("Multi datacenter ScyllaDBCluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should reconcile healthy datacenters when any of DCs are down", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(len(rkcMap)).To(o.BeNumerically(">=", 2))

		framework.By("Creating ScyllaDBCluster")
		scyllaConfigCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "scylla-config",
			},
			Data: map[string]string{
				"scylla.yaml": `read_request_timeout_in_ms: 1234`,
			},
		}

		const agentAuthToken = "foobar"
		scyllaManagerAgentConfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "scylla-manager-agent-config",
			},
			StringData: map[string]string{
				"scylla-manager-agent.yaml": fmt.Sprintf(`auth_token: %s`, agentAuthToken),
			},
		}

		scyllaConfigCM, err = f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Create(ctx, scyllaConfigCM, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaManagerAgentConfigSecret, err = f.KubeClient().CoreV1().Secrets(f.Namespace()).Create(ctx, scyllaManagerAgentConfigSecret, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := f.GetDefaultScyllaDBCluster(rkcMap)
		sc.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr(scyllaConfigCM.Name)
		sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
			CustomConfigSecretRef: pointer.Ptr(scyllaManagerAgentConfigSecret.Name),
		}

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

		framework.By("Removing Operator access to remote Kubernetes cluster associated with last DC")
		lastDCRKC := rkcMap[sc.Spec.Datacenters[len(sc.Spec.Datacenters)-1].Name]

		lastDCRKCKubeconfigSecret, err := f.KubeAdminClient().CoreV1().Secrets(lastDCRKC.Spec.KubeconfigSecretRef.Namespace).Get(ctx, lastDCRKC.Spec.KubeconfigSecretRef.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		const kubeconfigKey = "kubeconfig"

		o.Expect(lastDCRKCKubeconfigSecret.Data).To(o.HaveKey(kubeconfigKey))
		o.Expect(lastDCRKCKubeconfigSecret.Data[kubeconfigKey]).ToNot(o.BeEmpty())
		lastDCRKCWorkingKubeconfigBytes := lastDCRKCKubeconfigSecret.Data[kubeconfigKey]

		_, err = f.KubeAdminClient().CoreV1().Secrets(lastDCRKCKubeconfigSecret.Namespace).Patch(
			ctx,
			lastDCRKCKubeconfigSecret.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/data/%s", "value": "%s"}]`, kubeconfigKey, base64.StdEncoding.EncodeToString(scyllafixture.UnauthorizedKubeconfigBytes),
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Changing ScyllaDB configuration")
		const scyllaDBRackReadRequestTimeoutInMs = 4321
		_, err = f.KubeClient().CoreV1().ConfigMaps(scyllaConfigCM.Namespace).Patch(
			ctx,
			scyllaConfigCM.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/data/scylla.yaml", "value": "read_request_timeout_in_ms: %d"}]`, scyllaDBRackReadRequestTimeoutInMs,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Forcing a ScyllaDBCluster rollout")
		sc, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/forceRedeploymentReason", "value": "scylla config changed"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Awaiting until ScyllaDBCluster is degraded")
		waitCtx3, waitCtx3Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx3, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{},
			utils.IsScyllaDBClusterDegraded,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying if configuration change of ScyllaDB has been applied on healthy datacenters")
		o.Eventually(func(eo o.Gomega) {
			for _, dc := range sc.Spec.Datacenters[:len(sc.Spec.Datacenters)-1] {
				clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]

				scyllaConfigClient, err := v1alpha1utils.GetRemoteDatacenterScyllaConfigClient(ctx, sc, &dc, clusterClient.ScyllaAdminClient(), clusterClient.KubeAdminClient(), agentAuthToken)
				eo.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Verifying if configuration of ScyllaDB is being used by %q datacenter", dc.Name)
				gotReadRequestTimeoutInMs, err := scyllaConfigClient.ReadRequestTimeoutInMs(ctx)
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(gotReadRequestTimeoutInMs).To(o.Equal(int64(scyllaDBRackReadRequestTimeoutInMs)))
			}
		}).WithContext(waitCtx3).WithPolling(time.Second).Should(o.Succeed())

		framework.By("Fixing Operator access to remote Kubernetes cluster associated with last DC")
		_, err = f.KubeAdminClient().CoreV1().Secrets(lastDCRKCKubeconfigSecret.Namespace).Patch(
			ctx,
			lastDCRKCKubeconfigSecret.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/data/%s", "value": "%s"}]`, kubeconfigKey, base64.StdEncoding.EncodeToString(lastDCRKCWorkingKubeconfigBytes),
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx4, waitCtx4Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx4, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying if configuration change of ScyllaDB has been applied on all datacenters")
		for _, dc := range sc.Spec.Datacenters {
			clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]

			scyllaConfigClient, err := v1alpha1utils.GetRemoteDatacenterScyllaConfigClient(ctx, sc, &dc, clusterClient.ScyllaAdminClient(), clusterClient.KubeAdminClient(), agentAuthToken)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Verifying if configuration of ScyllaDB is being used by %q datacenter", dc.Name)
			gotReadRequestTimeoutInMs, err := scyllaConfigClient.ReadRequestTimeoutInMs(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(gotReadRequestTimeoutInMs).To(o.Equal(int64(scyllaDBRackReadRequestTimeoutInMs)))
		}
	})
})
