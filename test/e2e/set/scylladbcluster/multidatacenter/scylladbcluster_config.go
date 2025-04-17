package multidatacenter

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Multi datacenter ScyllaDBCluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should mirror ConfigMaps and Secrets referenced by ScyllaDBCluster into remote datacenters", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		rkcs, rkcClusterMap, err := utils.SetUpRemoteKubernetesClustersFromRestConfigs(ctx, framework.TestContext.RestConfigs, f)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating configs for ScyllaDB and ScyllaDB Manager Agent")

		metaCluster := f.Cluster(0)
		userNS, userClient, ok := metaCluster.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		makeScyllaDBConfigMap := func(name, configValue string) *corev1.ConfigMap {
			return &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: userNS.Name,
				},
				Data: map[string]string{
					"scylla.yaml": fmt.Sprintf(`read_request_timeout_in_ms: %s`, configValue),
				},
			}
		}

		makeScyllaDBManagerAgentSecret := func(name, configValue string) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: userNS.Name,
				},
				StringData: map[string]string{
					"scylla-manager-agent.yaml": fmt.Sprintf(`auth_token: %s`, configValue),
				},
			}
		}

		scyllaDBDatacenterTemplateConfigMap := makeScyllaDBConfigMap("datacenter-template", "1111")
		scyllaDBDatacenterTemplateRackTemplateConfigMap := makeScyllaDBConfigMap("datacenter-template-rack-template", "2222")
		scyllaDBDatacenterRackTemplateConfigMap := makeScyllaDBConfigMap("datacenter-rack-template", "3333")
		scyllaDBDatacenterDatacenterConfigMap := makeScyllaDBConfigMap("datacenter", "4444")
		const scyllaDBRackReadRequestTimeoutInMs = 5555
		scyllaDBDatacenterRackConfigMap := makeScyllaDBConfigMap("rack", fmt.Sprintf("%d", scyllaDBRackReadRequestTimeoutInMs))

		scyllaDBConfigMaps := []*corev1.ConfigMap{
			scyllaDBDatacenterTemplateConfigMap,
			scyllaDBDatacenterTemplateRackTemplateConfigMap,
			scyllaDBDatacenterRackTemplateConfigMap,
			scyllaDBDatacenterDatacenterConfigMap,
			scyllaDBDatacenterRackConfigMap,
		}

		scyllaDBManagerAgentDatacenterTemplateSecret := makeScyllaDBManagerAgentSecret("datacenter-template", "aaaa")
		scyllaDBManagerAgentDatacenterTemplateRackTemplateSecret := makeScyllaDBManagerAgentSecret("datacenter-template-rack-template", "bbbb")
		scyllaDBManagerAgentDatacenterRackTemplateSecret := makeScyllaDBManagerAgentSecret("datacenter-rack-template", "cccc")
		scyllaDBManagerAgentDatacenterSecret := makeScyllaDBManagerAgentSecret("datacenter", "dddd")
		const scyllaDBManagerAgentRackAuthToken = "eeee"
		scyllaDBManagerAgentDatacenterRackSecret := makeScyllaDBManagerAgentSecret("rack", scyllaDBManagerAgentRackAuthToken)

		scyllaDBManagerAgentSecrets := []*corev1.Secret{
			scyllaDBManagerAgentDatacenterTemplateSecret,
			scyllaDBManagerAgentDatacenterTemplateRackTemplateSecret,
			scyllaDBManagerAgentDatacenterRackTemplateSecret,
			scyllaDBManagerAgentDatacenterSecret,
			scyllaDBManagerAgentDatacenterRackSecret,
		}

		for _, cm := range scyllaDBConfigMaps {
			_, err := userClient.KubeClient().CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		for _, secret := range scyllaDBManagerAgentSecrets {
			_, err := userClient.KubeClient().CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		framework.By("Creating ScyllaDBCluster")
		sc := f.GetDefaultScyllaDBCluster(rkcs)

		sc.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr(scyllaDBDatacenterTemplateConfigMap.Name)
		sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
			CustomConfigSecretRef: pointer.Ptr(scyllaDBManagerAgentDatacenterTemplateSecret.Name),
		}
		sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
			CustomConfigMapRef: pointer.Ptr(scyllaDBDatacenterTemplateRackTemplateConfigMap.Name),
		}
		sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
			CustomConfigSecretRef: pointer.Ptr(scyllaDBManagerAgentDatacenterTemplateRackTemplateSecret.Name),
		}
		o.Expect(len(sc.Spec.Datacenters)).To(o.BeNumerically(">", 0))
		for idx := range sc.Spec.Datacenters {
			sc.Spec.Datacenters[idx].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
				CustomConfigMapRef: pointer.Ptr(scyllaDBDatacenterDatacenterConfigMap.Name),
			}
			sc.Spec.Datacenters[idx].ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
				CustomConfigSecretRef: pointer.Ptr(scyllaDBManagerAgentDatacenterSecret.Name),
			}
			sc.Spec.Datacenters[idx].RackTemplate = &scyllav1alpha1.RackTemplate{
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr(scyllaDBDatacenterRackTemplateConfigMap.Name),
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr(scyllaDBManagerAgentDatacenterRackTemplateSecret.Name),
				},
			}
			sc.Spec.Datacenters[idx].Racks = []scyllav1alpha1.RackSpec{
				{
					Name: "a",
					RackTemplate: scyllav1alpha1.RackTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							CustomConfigMapRef: pointer.Ptr(scyllaDBDatacenterRackConfigMap.Name),
						},
						ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
							CustomConfigSecretRef: pointer.Ptr(scyllaDBManagerAgentDatacenterRackSecret.Name),
						},
					},
				},
			}
		}

		sc, err = metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(userNS.Name).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx2, metaCluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sc, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, dc := range sc.Spec.Datacenters {
			framework.By("Verifying if ConfigMaps and Secrets referenced by ScyllaDBCluster %q are mirrored in %q Datacenter", sc.Name, dc.Name)

			clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]

			dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
				return dc.Name == dcStatus.Name
			})
			o.Expect(ok).To(o.BeTrue())
			o.Expect(dcStatus.RemoteNamespaceName).ToNot(o.BeNil())

			for _, cm := range scyllaDBConfigMaps {
				mirroredCM, err := clusterClient.KubeAdminClient().CoreV1().ConfigMaps(*dcStatus.RemoteNamespaceName).Get(ctx, cm.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(mirroredCM.Data).To(o.Equal(cm.Data))
			}

			for _, secret := range scyllaDBManagerAgentSecrets {
				mirroredSecret, err := clusterClient.KubeAdminClient().CoreV1().Secrets(*dcStatus.RemoteNamespaceName).Get(ctx, secret.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				for k, v := range secret.Data {
					decodedValue, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(mirroredSecret.Data[k])))
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(mirroredSecret.Data).To(o.HaveKey(k))
					o.Expect(decodedValue).To(o.Equal(v))
				}
			}

			framework.By("Verifying if custom auth token set via configuration of ScyllaDB Manager Agent is being used by %q datacenter", dc.Name)
			sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			svc, err := clusterClient.KubeAdminClient().CoreV1().Services(*dcStatus.RemoteNamespaceName).Get(ctx, naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 0), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			pod, err := clusterClient.KubeAdminClient().CoreV1().Pods(*dcStatus.RemoteNamespaceName).Get(ctx, naming.PodNameFromService(svc), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
			o.Expect(err).NotTo(o.HaveOccurred())

			scyllaConfigClient := scyllaclient.NewConfigClient(host, scyllaDBManagerAgentRackAuthToken)

			framework.By("Verifying if configuration of ScyllaDB is being used by %q datacenter", dc.Name)
			gotReadRequestTimeoutInMs, err := scyllaConfigClient.ReadRequestTimeoutInMs(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(gotReadRequestTimeoutInMs).To(o.Equal(int64(scyllaDBRackReadRequestTimeoutInMs)))
		}
	})
})
