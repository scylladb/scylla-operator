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
				StringData: map[string]string{
					"key": "value",
				},
			}
		}

		makeGenericConfigMap := func(name string) *corev1.ConfigMap {
			return &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: userNS.Name,
				},
				Data: map[string]string{
					"key": "value",
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

		makeVolumesWithConfigMapRef := func(cm *corev1.ConfigMap) []corev1.Volume {
			return []corev1.Volume{
				{
					Name: cm.Name,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cm.Name,
							},
						},
					},
				},
			}
		}

		framework.By("Creating all ConfigMaps to be referred from ScyllaDBCluster.Spec.DatacenterTemplate (to be mirrored to all DCs)")
		allDCsScyllaDBCustomConfigConfigMap := makeScyllaDBConfigMap("all-dcs-scylladb-customconfig", "1111")
		allDCsScyllaDBVolumesConfigMap := makeGenericConfigMap("all-dcs-scylladb-configmap-volumes")
		allDCsScyllaDBManagerAgentVolumesConfigMap := makeGenericConfigMap("all-dcs-scylladbmanageragent-configmap-volumes")

		allDCsRackTemplateScyllaDBCustomConfigConfigMap := makeScyllaDBConfigMap("all-dcs-racktemplate-scylladb-customconfig", "2222")
		allDCsRackTemplateScyllaDBVolumesConfigMap := makeGenericConfigMap("all-dcs-racktemplate-scylladb-configmap-volumes")
		allDCsRackTemplateScyllaDBManagerAgentVolumesConfigMap := makeGenericConfigMap("all-dcs-racktemplate-scylladbmanageragent-configmap-volumes")

		allDCsRacksScyllaDBCustomConfigConfigMap := makeScyllaDBConfigMap("all-dcs-racks-scylladb-customconfig", "3333")
		allDCsRacksScyllaDBManagerAgentVolumesConfigMap := makeGenericConfigMap("all-dcs-racks-scylladbmanageragent-configmap-volumes")
		allDCsRacksScyllaDBVolumesConfigMap := makeGenericConfigMap("all-dcs-racks-scylladb-configmap-volumes")

		userManagedConfigMapsToBeMirroredToAllDCs := []*corev1.ConfigMap{
			allDCsScyllaDBCustomConfigConfigMap,
			allDCsScyllaDBVolumesConfigMap,
			allDCsScyllaDBManagerAgentVolumesConfigMap,

			allDCsRackTemplateScyllaDBCustomConfigConfigMap,
			allDCsRackTemplateScyllaDBVolumesConfigMap,
			allDCsRackTemplateScyllaDBManagerAgentVolumesConfigMap,

			allDCsRacksScyllaDBCustomConfigConfigMap,
			allDCsRacksScyllaDBManagerAgentVolumesConfigMap,
			allDCsRacksScyllaDBVolumesConfigMap,
		}

		dcScyllaDBCustomConfigMap := makeScyllaDBConfigMap("dc-scylladb-customconfig", "4444")
		dcScyllaDBManagerAgentVolumesConfigMap := makeGenericConfigMap("dc-scylladbmanageragent-configmap-volumes")
		dcScyllaDBVolumesConfigMap := makeGenericConfigMap("dc-scylladb-configmap-volumes")

		dcRackTemplateScyllaDBCustomConfigMap := makeScyllaDBConfigMap("dc-racktemplate-scylladbmanageragent-customconfig", "3333")
		dcRackTemplateScyllaDBManagerAgentVolumesConfigMap := makeGenericConfigMap("dc-racktemplate-scylladbmanageragent-configmap-volumes")
		dcRackTemplateScyllaDBVolumesConfigMap := makeGenericConfigMap("dc-racktemplate-scylladb-configmap-volumes")

		const scyllaDBRackReadRequestTimeoutInMs = 5555
		dcRacksScyllaDBCustomConfigMap := makeScyllaDBConfigMap("dc-racks-scylladb-customconfig", fmt.Sprintf("%d", scyllaDBRackReadRequestTimeoutInMs))
		dcRacksScyllaDBManagerAgentVolumesConfigMap := makeGenericConfigMap("dc-racks-scylladbmanageragent-configmap-volumes")
		dcRacksScyllaDBVolumesConfigMap := makeGenericConfigMap("dc-racks-scylladb-configmap-volumes")

		userManagedConfigMapsToBeMirroredToSpecificDC := []*corev1.ConfigMap{
			dcScyllaDBCustomConfigMap,
			dcScyllaDBManagerAgentVolumesConfigMap,
			dcScyllaDBVolumesConfigMap,

			dcRackTemplateScyllaDBCustomConfigMap,
			dcRackTemplateScyllaDBManagerAgentVolumesConfigMap,
			dcRackTemplateScyllaDBVolumesConfigMap,

			dcRacksScyllaDBCustomConfigMap,
			dcRacksScyllaDBManagerAgentVolumesConfigMap,
			dcRacksScyllaDBVolumesConfigMap,
		}

		userManagedConfigMapsToBeMirrored := slices.Concat(
			userManagedConfigMapsToBeMirroredToAllDCs,
			userManagedConfigMapsToBeMirroredToSpecificDC,
		)

		for _, cm := range userManagedConfigMapsToBeMirrored {
			_, err := userClient.KubeClient().CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		framework.By("Creating all Secrets to be referred from ScyllaDBCluster.Spec.DatacenterTemplate (to be mirrored to all DCs)")
		allDCsScyllaDBManagerAgentCustomConfigSecret := makeScyllaDBManagerAgentSecret("all-dcs-scylladbmanageragent-customconfig", "aaaa")
		allDCsScyllaDBManagerAgentVolumesSecret := makeGenericSecret("all-dcs-scylladbmanageragent-secret-volumes")
		allDCsScyllaDBVolumesSecret := makeGenericSecret("all-dcs-scylladb-secret-volumes")

		allDCsRackTemplateScyllaDBManagerAgentCustomConfigSecret := makeScyllaDBManagerAgentSecret("all-dcs-racktemplate-scylladbmanageragent-customconfig", "bbbb")
		allDCsRackTemplateScyllaDBManagerAgentVolumesSecret := makeGenericSecret("all-dcs-racktemplate-scylladbmanageragent-secret-volumes")
		allDCsRackTemplateScyllaDBVolumesSecret := makeGenericSecret("all-dcs-racktemplate-scylladb-secret-volumes")

		const scyllaDBManagerAgentRackAuthToken = "eeee"
		allDCsRacksScyllaDBManagerAgentCustomConfigSecret := makeScyllaDBManagerAgentSecret("all-dcs-racks-scylladbmanageragent-customconfig", scyllaDBManagerAgentRackAuthToken)
		allDCsRacksScyllaDBManagerAgentVolumesSecret := makeGenericSecret("all-dcs-racks-scylladbmanageragent-secret-volumes")
		allDCsRacksScyllaDBVolumesSecret := makeGenericSecret("all-dcs-racks-scylladb-secret-volumes")

		userManagedSecretsToBeMirroredToAllDCs := []*corev1.Secret{
			allDCsScyllaDBManagerAgentCustomConfigSecret,
			allDCsScyllaDBManagerAgentVolumesSecret,
			allDCsScyllaDBVolumesSecret,
			allDCsRackTemplateScyllaDBManagerAgentCustomConfigSecret,
			allDCsRackTemplateScyllaDBManagerAgentVolumesSecret,
			allDCsRackTemplateScyllaDBVolumesSecret,
			allDCsRacksScyllaDBManagerAgentCustomConfigSecret,
			allDCsRacksScyllaDBManagerAgentVolumesSecret,
			allDCsRacksScyllaDBVolumesSecret,
		}

		framework.By("Creating all Secrets to be referred from ScyllaDBCluster.Spec.DatacenterTemplate.Datacenters (to be mirrored to a specific DC)")
		dcScyllaDBManagerAgentCustomConfigSecret := makeScyllaDBManagerAgentSecret("dc-scylladbmanageragent-customconfig", scyllaDBManagerAgentRackAuthToken)
		dcScyllaDBManagerAgentVolumesSecret := makeGenericSecret("dc-scylladbmanageragent-secret-volumes")
		dcScyllaDBVolumesSecret := makeGenericSecret("dc-scylladb-secret-volumes")

		dcRackTemplateScyllaDBManagerAgentCustomConfigSecret := makeScyllaDBManagerAgentSecret("dc-racktemplate-scylladbmanageragent-customconfig", scyllaDBManagerAgentRackAuthToken)
		dcRackTemplateScyllaDBManagerAgentVolumesSecret := makeGenericSecret("dc-racktemplate-scylladbmanageragent-secret-volumes")
		dcRackTemplateScyllaDBVolumesSecret := makeGenericSecret("dc-racktemplate-scylladb-secret-volumes")

		dcRacksScyllaDBManagerAgentCustomConfigSecret := makeScyllaDBManagerAgentSecret("dc-racks-scylladbmanageragent-customconfig", scyllaDBManagerAgentRackAuthToken)
		dcRacksScyllaDBManagerAgentVolumesSecret := makeGenericSecret("dc-racks-scylladbmanageragent-secret-volumes")
		dcRacksScyllaDBVolumesSecret := makeGenericSecret("dc-racks-scylladb-secret-volumes")

		userManagedSecretsToBeMirroredToSpecificDC := []*corev1.Secret{
			dcScyllaDBManagerAgentCustomConfigSecret,
			dcScyllaDBManagerAgentVolumesSecret,
			dcRackTemplateScyllaDBManagerAgentCustomConfigSecret,
			dcRackTemplateScyllaDBManagerAgentVolumesSecret,
			dcRackTemplateScyllaDBVolumesSecret,
			dcScyllaDBVolumesSecret,
			dcRacksScyllaDBManagerAgentCustomConfigSecret,
			dcRacksScyllaDBManagerAgentVolumesSecret,
			dcRacksScyllaDBVolumesSecret,
		}

		userManagedSecretsToBeMirrored := slices.Concat(
			userManagedSecretsToBeMirroredToAllDCs,
			userManagedSecretsToBeMirroredToSpecificDC,
		)

		for _, secret := range userManagedSecretsToBeMirrored {
			_, err := userClient.KubeClient().CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		framework.By("Creating ScyllaDBCluster")
		sc := f.GetDefaultScyllaDBCluster(rkcs)

		sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
			CustomConfigSecretRef: pointer.Ptr(allDCsScyllaDBManagerAgentCustomConfigSecret.Name),
			Volumes: slices.Concat(
				makeVolumesWithSecretRef(allDCsScyllaDBManagerAgentVolumesSecret),
				makeVolumesWithConfigMapRef(allDCsScyllaDBManagerAgentVolumesConfigMap),
			),
		}
		sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
			CustomConfigSecretRef: pointer.Ptr(allDCsRackTemplateScyllaDBManagerAgentCustomConfigSecret.Name),
			Volumes: slices.Concat(
				makeVolumesWithSecretRef(allDCsRackTemplateScyllaDBManagerAgentVolumesSecret),
				makeVolumesWithConfigMapRef(allDCsRackTemplateScyllaDBManagerAgentVolumesConfigMap),
			),
		}
		sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
			CustomConfigMapRef: pointer.Ptr(allDCsRackTemplateScyllaDBCustomConfigConfigMap.Name),
			Volumes: slices.Concat(
				makeVolumesWithSecretRef(allDCsRackTemplateScyllaDBVolumesSecret),
				makeVolumesWithConfigMapRef(allDCsRackTemplateScyllaDBVolumesConfigMap),
			),
		}
		sc.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef = pointer.Ptr(allDCsScyllaDBCustomConfigConfigMap.Name)
		sc.Spec.DatacenterTemplate.ScyllaDB.Volumes = slices.Concat(
			makeVolumesWithSecretRef(allDCsScyllaDBVolumesSecret),
			makeVolumesWithConfigMapRef(allDCsScyllaDBVolumesConfigMap),
		)

		sc.Spec.DatacenterTemplate.Racks = []scyllav1alpha1.RackSpec{
			{
				Name: "a",
				RackTemplate: scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						CustomConfigMapRef: pointer.Ptr(allDCsRacksScyllaDBCustomConfigConfigMap.Name),
						Volumes: slices.Concat(
							makeVolumesWithSecretRef(allDCsRacksScyllaDBVolumesSecret),
							makeVolumesWithConfigMapRef(allDCsRacksScyllaDBVolumesConfigMap),
						),
					},
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						CustomConfigSecretRef: pointer.Ptr(allDCsRacksScyllaDBManagerAgentCustomConfigSecret.Name),
						Volumes: slices.Concat(
							makeVolumesWithSecretRef(allDCsRacksScyllaDBManagerAgentVolumesSecret),
							makeVolumesWithConfigMapRef(allDCsRacksScyllaDBManagerAgentVolumesConfigMap),
						),
					},
				},
			},
		}

		o.Expect(sc.Spec.Datacenters).ToNot(o.BeEmpty())
		for idx := range sc.Spec.Datacenters {
			sc.Spec.Datacenters[idx].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
				CustomConfigMapRef: pointer.Ptr(dcScyllaDBCustomConfigMap.Name),
				Volumes: slices.Concat(
					makeVolumesWithSecretRef(dcScyllaDBVolumesSecret),
					makeVolumesWithConfigMapRef(dcScyllaDBVolumesConfigMap),
				),
			}
			sc.Spec.Datacenters[idx].ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
				CustomConfigSecretRef: pointer.Ptr(dcScyllaDBManagerAgentCustomConfigSecret.Name),
				Volumes: slices.Concat(
					makeVolumesWithSecretRef(dcScyllaDBManagerAgentVolumesSecret),
					makeVolumesWithConfigMapRef(dcScyllaDBManagerAgentVolumesConfigMap),
				),
			}
			sc.Spec.Datacenters[idx].RackTemplate = &scyllav1alpha1.RackTemplate{
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr(dcRackTemplateScyllaDBCustomConfigMap.Name),
					Volumes: slices.Concat(
						makeVolumesWithSecretRef(dcRackTemplateScyllaDBVolumesSecret),
						makeVolumesWithConfigMapRef(dcRackTemplateScyllaDBVolumesConfigMap),
					),
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr(dcRackTemplateScyllaDBManagerAgentCustomConfigSecret.Name),
					Volumes: slices.Concat(
						makeVolumesWithSecretRef(dcRackTemplateScyllaDBManagerAgentVolumesSecret),
						makeVolumesWithConfigMapRef(dcRackTemplateScyllaDBManagerAgentVolumesConfigMap),
					),
				},
			}

			sc.Spec.Datacenters[idx].Racks = []scyllav1alpha1.RackSpec{
				{
					Name: "a",
					RackTemplate: scyllav1alpha1.RackTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							CustomConfigMapRef: pointer.Ptr(dcRacksScyllaDBCustomConfigMap.Name),
							Volumes: slices.Concat(
								makeVolumesWithSecretRef(dcRacksScyllaDBVolumesSecret),
								makeVolumesWithConfigMapRef(dcRacksScyllaDBVolumesConfigMap),
							),
						},
						ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
							CustomConfigSecretRef: pointer.Ptr(dcRacksScyllaDBManagerAgentCustomConfigSecret.Name),
							Volumes: slices.Concat(
								makeVolumesWithSecretRef(dcRacksScyllaDBManagerAgentVolumesSecret),
								makeVolumesWithConfigMapRef(dcRacksScyllaDBManagerAgentVolumesConfigMap),
							),
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

		framework.By("Collecting operator-managed Secrets which should be mirrored into remote datacenters")
		var operatorManagedScyllaDBManagerAgentSecrets []*corev1.Secret

		scyllaDBManagerAgentAuthTokenSecretName, err := naming.ScyllaDBManagerAgentAuthTokenSecretNameForScyllaDBCluster(sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		operatorManagedScyllaDBManagerAgentSecrets = append(operatorManagedScyllaDBManagerAgentSecrets, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scyllaDBManagerAgentAuthTokenSecretName,
				Namespace: sc.Namespace,
			},
		})

		operatorManagedSecretsToBeMirrored := slices.Concat(
			operatorManagedScyllaDBManagerAgentSecrets,
		)

		secretsToBeMirrored := slices.Concat(
			userManagedSecretsToBeMirrored,
			operatorManagedSecretsToBeMirrored,
		)

		configMapsToBeMirrored := userManagedConfigMapsToBeMirrored

		for _, dc := range sc.Spec.Datacenters {
			framework.By("Verifying if ConfigMaps and Secrets referenced by ScyllaDBCluster %q are mirrored in %q Datacenter", sc.Name, dc.Name)

			clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]

			dcStatus, _, ok := oslices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
				return dc.Name == dcStatus.Name
			})
			o.Expect(ok).To(o.BeTrue())
			o.Expect(dcStatus.RemoteNamespaceName).ToNot(o.BeNil())

			for _, cm := range configMapsToBeMirrored {
				mirroredCM, err := clusterClient.KubeAdminClient().CoreV1().ConfigMaps(*dcStatus.RemoteNamespaceName).Get(ctx, cm.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(mirroredCM.Data).To(o.Equal(cm.Data))
			}

			for _, secret := range secretsToBeMirrored {
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
