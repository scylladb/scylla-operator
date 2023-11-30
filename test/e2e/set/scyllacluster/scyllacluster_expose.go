// Copyright (c) 2022 ScyllaDB

package scyllacluster

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/gocql/gocql/scyllacloud"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/verification"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should connect to cluster via Ingresses", func() {
		if !utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
			g.Skip(fmt.Sprintf("Skipping because %q feature is disabled", features.AutomaticTLSCertificates))
		}

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()

		sc.Spec.DNSDomains = []string{fmt.Sprintf("%s.private.nodes.scylladb.com", f.Namespace()), fmt.Sprintf("%s.public.nodes.scylladb.com", f.Namespace())}
		if framework.TestContext.IngressController != nil {
			sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
				CQL: &scyllav1.CQLExposeOptions{
					Ingress: &scyllav1.IngressOptions{
						IngressClassName: framework.TestContext.IngressController.IngressClassName,
						Annotations:      framework.TestContext.IngressController.CustomAnnotations,
					},
				},
			}
		}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		// Wait until node picks up regenerated certificate.
		err = utils.WaitUntilServingCertificateIsLive(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Nodes reloaded the certificate")

		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(int(sc.Spec.Datacenter.Racks[0].Members)))

		connectionBundleDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("connection-bundle-%s-", f.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			err := os.RemoveAll(connectionBundleDir)
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		var groupVersioner runtime.GroupVersioner = schema.GroupVersions([]schema.GroupVersion{cqlclientv1alpha1.GroupVersion})
		decoder := scheme.Codecs.DecoderToVersion(scheme.Codecs.UniversalDeserializer(), groupVersioner)
		encoder := scheme.Codecs.EncoderForVersion(scheme.DefaultYamlSerializer, groupVersioner)

		// Connect to nodes via Ingresses
		for _, dnsDomain := range sc.Spec.DNSDomains {
			framework.By("Connecting via %s domain", dnsDomain)

			framework.By("Injecting ingress controller address into the CQL connection bundle")
			bundleSecret, err := f.KubeClient().CoreV1().Secrets(sc.Namespace).Get(ctx, naming.GetScyllaClusterLocalAdminCQLConnectionConfigsName(sc.Name), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			cqlConnectionConfig := &cqlclientv1alpha1.CQLConnectionConfig{}
			_, _, err = decoder.Decode(bundleSecret.Data[dnsDomain], nil, cqlConnectionConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			ingressAddress := f.GetIngressAddress(cqlConnectionConfig.Datacenters[sc.Spec.Datacenter.Name].Server)
			cqlConnectionConfig.Datacenters[sc.Spec.Datacenter.Name].Server = ingressAddress

			cqlConnectionConfigData, err := runtime.Encode(encoder, cqlConnectionConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			cqlConnectionConfigFilePath := path.Join(connectionBundleDir, fmt.Sprintf("%s.yaml", dnsDomain))
			framework.By("Saving CQL Connection Config in %s", cqlConnectionConfigFilePath)
			err = os.WriteFile(cqlConnectionConfigFilePath, cqlConnectionConfigData, 0600)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Connecting to cluster via Ingress")
			cluster, err := scyllacloud.NewCloudCluster(cqlConnectionConfigFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Increase default timeout, due to additional hop on the route to host.
			cluster.Timeout = 10 * time.Second

			di := insertAndVerifyCQLData(ctx, hosts, utils.WithClusterConfig(cluster))
			di.Close()
		}
	})

	type entry struct {
		exposeOptions                            *scyllav1.ExposeOptions
		memberServiceCreatedHook                 func(ctx context.Context, client corev1client.CoreV1Interface, svc *corev1.Service)
		validateService                          func(svc *corev1.Service)
		validateScyllaConfig                     func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod)
		expectedNumberOfIPAddressesInServingCert func(nodes int) int
	}

	g.DescribeTable("should be exposed", func(e *entry) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.ExposeOptions = e.exposeOptions

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for member Service to be created")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		svcName := naming.MemberServiceName(sc.Spec.Datacenter.Racks[0], sc, 0)
		svc, err := utils.WaitForServiceState(waitCtx, f.KubeClient().CoreV1().Services(sc.Namespace), svcName, utils.WaitForStateOptions{}, func(svc *corev1.Service) (bool, error) {
			return svc != nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		if e.memberServiceCreatedHook != nil {
			e.memberServiceCreatedHook(ctx, f.KubeAdminClient().CoreV1(), svc)
		}

		framework.By("Waiting for ScyllaCluster to deploy")
		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		svc, err = f.KubeClient().CoreV1().Services(sc.Namespace).Get(ctx, svcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		e.validateService(svc)

		pod, err := f.KubeClient().CoreV1().Pods(sc.Namespace).Get(ctx, svcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		configClient, err := utils.GetScyllaConfigClient(ctx, f.KubeClient().CoreV1(), sc, pod.Status.PodIP)
		o.Expect(err).NotTo(o.HaveOccurred())

		e.validateScyllaConfig(ctx, configClient, svc, pod)

		if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
			serviceAndPodIPs, err := utils.GetNodesServiceAndPodIPs(ctx, f.KubeClient().CoreV1(), sc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(serviceAndPodIPs).To(o.HaveLen(e.expectedNumberOfIPAddressesInServingCert(int(utils.GetMemberCount(sc)))))

			hostsIPs, err := helpers.ParseIPs(serviceAndPodIPs)
			o.Expect(err).NotTo(o.HaveOccurred())

			var servingDNSNames []string
			services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(naming.ClusterLabels(sc)).String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(services.Items).To(o.HaveLen(int(utils.GetMemberCount(sc)) + 1))
			for _, svc := range services.Items {
				servingDNSNames = append(servingDNSNames, fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace))
			}

			servingCertSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-serving-certs", sc.Name), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			servingCerts, _, _, _ := verification.VerifyAndParseTLSCert(servingCertSecret, verification.TLSCertOptions{
				IsCA:     pointer.Ptr(false),
				KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
			})
			o.Expect(servingCerts).To(o.HaveLen(1))
			o.Expect(servingCerts[0].Subject.CommonName).To(o.BeEmpty())
			o.Expect(helpers.NormalizeIPs(servingCerts[0].IPAddresses)).To(o.ConsistOf(hostsIPs))
			o.Expect(servingCerts[0].DNSNames).To(o.ConsistOf(servingDNSNames))
		}
	},
		g.Entry("using ClusterIP Service, and broadcast it to both client and nodes", &entry{
			exposeOptions: &scyllav1.ExposeOptions{
				NodeService: &scyllav1.NodeServiceTemplate{
					Type: scyllav1.NodeServiceTypeClusterIP,
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
					},
				},
			},
			validateService: func(svc *corev1.Service) {
				o.Expect(svc.Spec.Type).To(o.Equal(corev1.ServiceTypeClusterIP))
				o.Expect(svc.Spec.ClusterIP).NotTo(o.BeEmpty())
				parsedClusterIP := net.ParseIP(svc.Spec.ClusterIP)
				o.Expect(parsedClusterIP).ToNot(o.BeNil())
			},
			validateScyllaConfig: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				broadcastAddress, err := configClient.BroadcastAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastAddress).To(o.Equal(svc.Spec.ClusterIP))

				broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastRPCAddress).To(o.Equal(svc.Spec.ClusterIP))
			},
			expectedNumberOfIPAddressesInServingCert: func(nodes int) int {
				// Each node have 1 PodIP and 1 Service ClusterIP
				return 2 * nodes
			},
		}),
		g.Entry("using ClusterIP Service, and broadcast it to nodes, and PodIP from Pod Status to clients", &entry{
			exposeOptions: &scyllav1.ExposeOptions{
				NodeService: &scyllav1.NodeServiceTemplate{
					Type: scyllav1.NodeServiceTypeClusterIP,
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
						PodIP: &scyllav1.PodIPAddressOptions{
							Source: scyllav1.StatusPodIPSource,
						},
					},
				},
			},
			validateService: func(svc *corev1.Service) {
				o.Expect(svc.Spec.Type).To(o.Equal(corev1.ServiceTypeClusterIP))
				o.Expect(svc.Spec.ClusterIP).NotTo(o.BeEmpty())
				parsedClusterIP := net.ParseIP(svc.Spec.ClusterIP)
				o.Expect(parsedClusterIP).ToNot(o.BeNil())
			},
			validateScyllaConfig: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				broadcastAddress, err := configClient.BroadcastAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastAddress).To(o.Equal(svc.Spec.ClusterIP))

				broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastRPCAddress).To(o.Equal(pod.Status.PodIP))
			},
			expectedNumberOfIPAddressesInServingCert: func(nodes int) int {
				// Each node have 1 PodIP and 1 Service ClusterIP
				return 2 * nodes
			},
		}),
		g.Entry("using Headless Service, and broadcast PodIP from Pod Status to both client and nodes", &entry{
			exposeOptions: &scyllav1.ExposeOptions{
				NodeService: &scyllav1.NodeServiceTemplate{
					Type: scyllav1.NodeServiceTypeHeadless,
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
						PodIP: &scyllav1.PodIPAddressOptions{
							Source: scyllav1.StatusPodIPSource,
						},
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
						PodIP: &scyllav1.PodIPAddressOptions{
							Source: scyllav1.StatusPodIPSource,
						},
					},
				},
			},
			validateService: func(svc *corev1.Service) {
				o.Expect(svc.Spec.Type).To(o.Equal(corev1.ServiceTypeClusterIP))
				o.Expect(svc.Spec.ClusterIP).To(o.Equal(corev1.ClusterIPNone))
			},
			validateScyllaConfig: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				broadcastAddress, err := configClient.BroadcastAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastAddress).To(o.Equal(pod.Status.PodIP))

				broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastRPCAddress).To(o.Equal(pod.Status.PodIP))
			},
			expectedNumberOfIPAddressesInServingCert: func(nodes int) int {
				// Each node have only PodIP
				return nodes
			},
		}),
		g.Entry("using LoadBalancer Service, and broadcast external IP to client and PodIP to nodes", &entry{
			exposeOptions: &scyllav1.ExposeOptions{
				NodeService: &scyllav1.NodeServiceTemplate{
					Type: scyllav1.NodeServiceTypeLoadBalancer,
					// Change to non-default LB class to avoid conflicts on status when running in cloud
					LoadBalancerClass: pointer.Ptr(fmt.Sprintf("lb-class-%s", rand.String(6))),
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
					},
				},
			},
			memberServiceCreatedHook: func(ctx context.Context, client corev1client.CoreV1Interface, svc *corev1.Service) {
				framework.By("Patching member Service with dummy external IP")
				_, err := f.KubeAdminClient().CoreV1().Services(svc.Namespace).Patch(
					ctx,
					svc.Name,
					types.MergePatchType,
					[]byte(`{"status":{"loadBalancer":{"ingress":[{"ip":"123.123.123.123"}]}}}`),
					metav1.PatchOptions{},
					"status",
				)
				o.Expect(err).NotTo(o.HaveOccurred())
			},
			validateService: func(svc *corev1.Service) {
				o.Expect(svc.Spec.Type).To(o.Equal(corev1.ServiceTypeLoadBalancer))
			},
			validateScyllaConfig: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				broadcastAddress, err := configClient.BroadcastAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastAddress).To(o.Equal(pod.Status.PodIP))

				broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(broadcastRPCAddress).To(o.Equal("123.123.123.123"))
			},
			expectedNumberOfIPAddressesInServingCert: func(nodes int) int {
				// Each node have 1 PodIP, 1 Service ClusterIP and External IP
				return 3 * nodes
			},
		}),
	)

	g.It("should create ingress objects when ingress exposeOptions are provided", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.DNSDomains = []string{fmt.Sprintf("%s.private.nodes.scylladb.com", f.Namespace()), fmt.Sprintf("%s.public.nodes.scylladb.com", f.Namespace())}
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			CQL: &scyllav1.CQLExposeOptions{
				Ingress: &scyllav1.IngressOptions{
					IngressClassName: "my-cql-ingress-class",
					Annotations: map[string]string{
						"my-cql-annotation-key": "my-cql-annotation-value",
					},
				},
			},
		}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Verifying AnyNode Ingresses")
		services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		ingresses, err := f.KubeClient().NetworkingV1().Ingresses(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeAnyNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				fmt.Sprintf("cql.%s", sc.Spec.DNSDomains[0]),
				fmt.Sprintf("cql.%s", sc.Spec.DNSDomains[1]),
			},
			"cql-ssl",
		)

		framework.By("Verifying Node Ingresses")
		services, err = f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		var nodeHostIDs []string
		for _, svc := range services.Items {
			o.Expect(svc.Annotations).To(o.HaveKey(naming.HostIDAnnotation))
			nodeHostIDs = append(nodeHostIDs, svc.Annotations[naming.HostIDAnnotation])
		}

		ingresses, err = f.KubeClient().NetworkingV1().Ingresses(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				fmt.Sprintf("%s.cql.%s", nodeHostIDs[0], sc.Spec.DNSDomains[0]),
				fmt.Sprintf("%s.cql.%s", nodeHostIDs[0], sc.Spec.DNSDomains[1]),
			},
			"cql-ssl",
		)
	})
})

func verifyIngress(ingress *networkingv1.Ingress, name string, annotations map[string]string, className string, service *corev1.Service, hosts []string, servicePort string) {
	o.Expect(ingress.Name).To(o.Equal(name))
	for k, v := range annotations {
		o.Expect(ingress.Annotations).To(o.HaveKeyWithValue(k, v))
	}

	o.Expect(ingress.Spec.IngressClassName).ToNot(o.BeNil())
	o.Expect(*ingress.Spec.IngressClassName).To(o.Equal(className))
	o.Expect(ingress.Spec.Rules).To(o.HaveLen(len(hosts)))
	for i := range hosts {
		o.Expect(ingress.Spec.Rules[i].Host).To(o.Equal(hosts[i]))
	}
	for _, rule := range ingress.Spec.Rules {
		o.Expect(rule.HTTP.Paths).To(o.HaveLen(1))
		o.Expect(rule.HTTP.Paths[0].Path).To(o.Equal("/"))
		o.Expect(rule.HTTP.Paths[0].PathType).ToNot(o.BeNil())
		o.Expect(*rule.HTTP.Paths[0].PathType).To(o.Equal(networkingv1.PathTypePrefix))
		o.Expect(rule.HTTP.Paths[0].Backend.Service).ToNot(o.BeNil())
		o.Expect(rule.HTTP.Paths[0].Backend.Service.Name).To(o.Equal(service.Name))
		o.Expect(rule.HTTP.Paths[0].Backend.Service.Port.Name).To(o.Equal(servicePort))
	}
}
