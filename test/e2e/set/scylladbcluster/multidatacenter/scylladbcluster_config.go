package multidatacenter

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
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

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcs, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating configs for ScyllaDB and ScyllaDB Manager Agent")

		userNS, userClient, ok := f.DefaultNamespaceIfAny()
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

		makeGenericSecret := func(name string) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: userNS.Name,
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			}
		}

		makeVolumesWithSecretRef := func(secret *corev1.Secret) []corev1.Volume {
			return []corev1.Volume{
				{
					Name: secret.Name,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
						},
					},
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

		scyllaDBVolumesSecret := makeGenericSecret("scylladb-volumes")
		scyllaDBManagerAgentVolumesSecret := makeGenericSecret("scylladb-manager-agent-volumes")
		rackTemplateScyllaDBVolumesSecret := makeGenericSecret("rack-template-scylladb-volumes")
		rackTemplateScyllaDBManagerAgentVolumesSecret := makeGenericSecret("rack-template-scylladb-manager-agent-volumes")
		racksScyllaDBVolumesSecret := makeGenericSecret("racks-scylladb-volumes")
		racksScyllaDBManagerAgentVolumesSecret := makeGenericSecret("racks-scylladb-manager-agent-volumes")

		userManagedVolumesSecrets := []*corev1.Secret{
			scyllaDBVolumesSecret,
			scyllaDBManagerAgentVolumesSecret,
			rackTemplateScyllaDBVolumesSecret,
			rackTemplateScyllaDBManagerAgentVolumesSecret,
			racksScyllaDBVolumesSecret,
			racksScyllaDBManagerAgentVolumesSecret,
		}

		userManagedSecrets := slices.Concat(
			userManagedVolumesSecrets,
			scyllaDBManagerAgentSecrets,
		)

		for _, cm := range scyllaDBConfigMaps {
			_, err := userClient.KubeClient().CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		for _, secret := range userManagedSecrets {
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
		sc.Spec.DatacenterTemplate.ScyllaDB.Volumes = makeVolumesWithSecretRef(scyllaDBVolumesSecret)
		sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.Volumes = makeVolumesWithSecretRef(scyllaDBManagerAgentVolumesSecret)
		sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDB.Volumes = makeVolumesWithSecretRef(rackTemplateScyllaDBVolumesSecret)
		sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDBManagerAgent.Volumes = makeVolumesWithSecretRef(rackTemplateScyllaDBManagerAgentVolumesSecret)

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
							Volumes:            makeVolumesWithSecretRef(racksScyllaDBVolumesSecret),
						},
						ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
							CustomConfigSecretRef: pointer.Ptr(scyllaDBManagerAgentDatacenterRackSecret.Name),
							Volumes:               makeVolumesWithSecretRef(racksScyllaDBManagerAgentVolumesSecret),
						},
					},
				},
			}
		}

		sc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(userNS.Name).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sc, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx2, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, dc := range sc.Spec.Datacenters {
			framework.By("Verifying if ConfigMaps and Secrets referenced by ScyllaDBCluster %q are mirrored in %q Datacenter", sc.Name, dc.Name)

			clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]

			dcStatus, _, ok := oslices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
				return dc.Name == dcStatus.Name
			})
			o.Expect(ok).To(o.BeTrue())
			o.Expect(dcStatus.RemoteNamespaceName).ToNot(o.BeNil())

			for _, cm := range scyllaDBConfigMaps {
				mirroredCM, err := clusterClient.KubeAdminClient().CoreV1().ConfigMaps(*dcStatus.RemoteNamespaceName).Get(ctx, cm.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(mirroredCM.Data).To(o.Equal(cm.Data))
			}

			for _, secret := range userManagedSecrets {
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
