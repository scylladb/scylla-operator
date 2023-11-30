// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

func addQuantity(lhs resource.Quantity, rhs resource.Quantity) *resource.Quantity {
	res := lhs.DeepCopy()
	res.Add(rhs)

	// Pre-cache the string so DeepEqual works.
	_ = res.String()

	return &res
}

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.FContext("TODO", func() {
		type entry struct {
			repository           string
			version              string
			minReadySeconds      int32
			nodeServiceType      scyllav1.NodeServiceType
			nodesBroadcastType   scyllav1.BroadcastAddressType
			clientsBroadcastType scyllav1.BroadcastAddressType
			makeClusterConfig    func(ctx context.Context, sc *scyllav1.ScyllaCluster, client corev1client.CoreV1Interface) *gocql.ClusterConfig
		}

		makeNonTLSClusterConfig := func(ctx context.Context, sc *scyllav1.ScyllaCluster, client corev1client.CoreV1Interface) *gocql.ClusterConfig {
			hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
			o.Expect(hosts).NotTo(o.HaveLen(0))
			clusterConfig := gocql.NewCluster(hosts...)
			clusterConfig.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
			clusterConfig.Timeout = 3 * time.Second
			clusterConfig.ConnectTimeout = 3 * time.Second
			clusterConfig.Port = 9042
			clusterConfig.Consistency = gocql.Quorum

			return clusterConfig
		}

		// makeTLSClusterConfig := func(ctx context.Context, sc *scyllav1.ScyllaCluster, client corev1client.CoreV1Interface) *gocql.ClusterConfig {
		// 	servingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-serving-ca", sc.Name), metav1.GetOptions{})
		// 	o.Expect(err).NotTo(o.HaveOccurred())
		// 	servingCACerts, _ := verification.VerifyAndParseCABundle(servingCABundleConfigMap)
		// 	o.Expect(servingCACerts).To(o.HaveLen(1))
		//
		// 	adminClientSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-user-admin", sc.Name), metav1.GetOptions{})
		// 	o.Expect(err).NotTo(o.HaveOccurred())
		// 	_, adminClientCertBytes, _, adminClientKeyBytes := verification.VerifyAndParseTLSCert(adminClientSecret, verification.TLSCertOptions{
		// 		IsCA:     pointer.Ptr(false),
		// 		KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
		// 	})
		//
		// 	servingCAPool := x509.NewCertPool()
		// 	servingCAPool.AddCert(servingCACerts[0])
		// 	adminTLSCert, err := tls.X509KeyPair(adminClientCertBytes, adminClientKeyBytes)
		// 	o.Expect(err).NotTo(o.HaveOccurred())
		//
		// 	hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		// 	o.Expect(hosts).NotTo(o.HaveLen(0))
		// 	host := hosts[0]
		// 	clusterConfig := gocql.NewCluster(host)
		// 	clusterConfig.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
		// 	clusterConfig.Timeout = 3 * time.Second
		// 	clusterConfig.ConnectTimeout = 3 * time.Second
		// 	clusterConfig.Port = 9142
		// 	clusterConfig.Consistency = gocql.Quorum
		// 	clusterConfig.SslOpts = &gocql.SslOptions{
		// 		Config: &tls.Config{
		// 			ServerName:         host,
		// 			InsecureSkipVerify: false,
		// 			Certificates:       []tls.Certificate{adminTLSCert},
		// 			RootCAs:            servingCAPool,
		// 		},
		// 	}
		//
		// 	return clusterConfig
		// }

		g.DescribeTable("shouldn't disrupt ongoing traffic on rollouts", func(e entry) {
			const (
				trafficWarmupDuration          = 60 * time.Second
				trafficAfterCertUpdateDuration = 60 * time.Second
			)

			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
			o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
			sc.Spec.Datacenter.Racks[0].Members = 3
			sc.Spec.Repository = e.repository
			sc.Spec.Version = e.version
			sc.Spec.MinReadySeconds = e.minReadySeconds
			sc.Spec.Sysctls = []string{
				"net.ipv4.tcp_syn_retries=3",
			}
			sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
				NodeService: &scyllav1.NodeServiceTemplate{
					Type: e.nodeServiceType,
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Clients: scyllav1.BroadcastOptions{
						Type: e.clientsBroadcastType,
					},
					Nodes: scyllav1.BroadcastOptions{
						Type: e.nodesBroadcastType,
					},
				},
			}

			framework.By("Creating a ScyllaCluster")
			sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			renderArgs := map[string]any{
				"name":              sc.Name,
				"namespace":         sc.Namespace,
				"scyllaClusterName": sc.Name,
			}
			if framework.TestContext.IngressController != nil {
				renderArgs["ingressClassName"] = framework.TestContext.IngressController.IngressClassName
				renderArgs["ingressCustomAnnotations"] = framework.TestContext.IngressController.CustomAnnotations
			}

			sm, _, err := scyllafixture.ScyllaDBMonitoringTemplate.RenderObject(renderArgs)
			o.Expect(err).NotTo(o.HaveOccurred())

			sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace).Create(
				ctx,
				sm,
				metav1.CreateOptions{
					FieldManager: f.FieldManager(),
					// Disable strict validation until https://github.com/scylladb/scylla-operator/issues/1251 is fixed
					FieldValidation: metav1.FieldValidationWarn,
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			go func() {
				client := f.KubeClient().CoreV1().Pods(f.Namespace())
				labelSelector := labels.SelectorFromSet(naming.ClusterLabels(sc)).String()
				lw := &cache.ListWatch{
					ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
						options.LabelSelector = labelSelector
						return client.List(ctx, options)
					}),
					WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
						options.LabelSelector = labelSelector
						return client.Watch(ctx, options)
					},
				}

				_, _ = watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
					switch event.Type {
					case watch.Added, watch.Modified:
					case watch.Deleted:
						pod := event.Object.(*corev1.Pod)
						framework.Infof("Pod %q deleted", naming.ObjRef(pod))
					default:
						return true, fmt.Errorf("unexpected event: %#v", event)
					}
					return false, nil
				})
			}()

			framework.By("Waiting for the ScyllaDBMonitoring to rollout (RV=%s)", sm.ResourceVersion)
			waitCtx2, waitCtx2Cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer waitCtx2Cancel()
			sm, err = utils.WaitForScyllaDBMonitoringState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace), sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaDBMonitoringRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
			waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
			defer waitCtx1Cancel()
			sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			scClient, hosts, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range hosts {
				err = scClient.SetLogLevel(ctx, "cql_server", "trace", host)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// verifyScyllaCluster(ctx, f.KubeClient(), sc)
			hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
			// o.Expect(hosts).To(o.HaveLen(3))

			const loaders = 1
			stopLoaderFuncs := make([]func() *utils.DataInserterReport, 0, loaders)

			for i := 0; i < loaders; i++ {
				clusterConfig := e.makeClusterConfig(ctx, sc, f.KubeClient().CoreV1())

				di := insertAndVerifyCQLData(ctx, hosts, utils.WithClusterConfig(clusterConfig))
				defer di.Close()

				trafficCtx, trafficCtxCancel := context.WithCancel(ctx)
				defer trafficCtxCancel()

				stopLoader, err := di.StartContinuousReads(trafficCtx)
				o.Expect(err).NotTo(o.HaveOccurred())

				stopLoaderFuncs = append(stopLoaderFuncs, stopLoader)
			}

			framework.By("Waiting %s before initiating rolling restart", trafficWarmupDuration)
			select {
			case <-ctx.Done():
				g.Fail("Test ended prematurely")
			case <-time.After(trafficWarmupDuration):
			}

			framework.By("Triggering rolling restart")
			sc.ManagedFields = nil
			sc.ResourceVersion = ""
			sc.Spec.ForceRedeploymentReason = rand.String(8)
			scData, err := runtime.Encode(scheme.Codecs.LegacyCodec(scyllav1.GroupVersion), sc)
			o.Expect(err).NotTo(o.HaveOccurred())
			// TODO: Use generated Apply method when our clients have it.
			//       Ref: https://github.com/scylladb/scylla-operator/issues/1474
			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.ApplyPatchType, scData, metav1.PatchOptions{
				FieldManager: f.FieldManager(),
				Force:        pointer.Ptr(true),
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
			waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
			defer waitCtx3Cancel()
			sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting %s before killing loaders", time.Duration(2*e.minReadySeconds)*time.Second)
			select {
			case <-ctx.Done():
				g.Fail("Test ended prematurely")
			case <-time.After(time.Duration(2*e.minReadySeconds) * time.Second):
			}

			framework.By("Killing loaders")

			var totalRequests, totalFailures int
			for _, stopLoader := range stopLoaderFuncs {
				report := stopLoader()
				for err, count := range report.Errors {
					framework.Infof("Error(%d): %s", count, err)
				}

				totalRequests += report.TotalRequests
				totalFailures += report.Failures
			}

			ratio := float64(totalRequests-totalFailures) / float64(totalRequests)
			framework.Infof("Read requests: %d", totalRequests)
			framework.Infof("Read failures: %d", totalFailures)
			framework.Infof("Read success ratio: %.2f", ratio)
			o.Expect(ratio).To(o.BeNumerically(">=", 0.99))
		},
			g.Entry("0s ClusterIP cluster with non-TLS traffic 5.0.12", entry{
				repository:           "docker.io/scylladb/scylla",
				version:              "5.0.12",
				minReadySeconds:      0,
				nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
				nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
				clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
				makeClusterConfig:    makeNonTLSClusterConfig,
			}),
			// g.Entry("0s PodIP cluster with non-TLS traffic 5.0.12", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.0.12",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s ClusterIP cluster with non-TLS traffic 5.1.18", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.1.18",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s PodIP cluster with non-TLS traffic 5.1.18", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.1.18",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s ClusterIP cluster with non-TLS traffic 5.2.9", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.2.9",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s PodIP cluster with non-TLS traffic 5.2.9", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.2.9",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s ClusterIP cluster with non-TLS traffic 5.3.0-rc0", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.3.0-rc0",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s PodIP cluster with non-TLS traffic 5.3.0-rc0", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.3.0-rc0",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s ClusterIP cluster with non-TLS traffic 5.4.0-rc0", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.4.0-rc0",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("0s PodIP cluster with non-TLS traffic 5.4.0-rc0", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.4.0-rc0",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("10s PodIP cluster with non-TLS traffic 5.0.12", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.0.12",
			// 	minReadySeconds:      10,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("30s PodIP cluster with non-TLS traffic 5.0.12", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.0.12",
			// 	minReadySeconds:      30,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds ClusterIP cluster with non-TLS traffic 5.0.12", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.0.12",
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds PodIP cluster with non-TLS traffic 5.0.12", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.0.12",
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			//
			// g.Entry("ClusterIP cluster with non-TLS traffic 5.2.9", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.2.9",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("PodIP cluster with non-TLS traffic 5.2.9", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.2.9",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds ClusterIP cluster with non-TLS traffic 5.2.9", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.2.9",
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds PodIP cluster with non-TLS traffic 5.2.9", entry{
			// 	repository:           "docker.io/scylladb/scylla",
			// 	version:              "5.2.9",
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			//
			// g.Entry("ClusterIP cluster with non-TLS traffic 5.1.18", entry{
			// 	repository:           "docker.io/scylladb/scylla-nightly",
			// 	version:              "5.1.18",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("PodIP cluster with non-TLS traffic 5.1.18", entry{
			// 	repository:           "docker.io/scylladb/scylla-nightly",
			// 	version:              "5.1.18",
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds ClusterIP cluster with non-TLS traffic 5.1.18", entry{
			// 	repository:           "docker.io/scylladb/scylla-nightly",
			// 	version:              "5.1.18",
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds PodIP cluster with non-TLS traffic 5.1.18", entry{
			// 	repository:           "docker.io/scylladb/scylla-nightly",
			// 	version:              "5.1.18",
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("ClusterIP cluster with TLS traffic", entry{
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeTLSClusterConfig,
			// }),
			// g.Entry("PodIP cluster with TLS traffic", entry{
			// 	minReadySeconds:      0,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeTLSClusterConfig,
			// }),
			// g.Entry("60s minReadySeconds ClusterIP cluster with TLS traffic", entry{
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeTLSClusterConfig,
			// }),

			// g.Entry("60s minReadySeconds PodIP cluster with TLS traffic", entry{
			// 	minReadySeconds:      60,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeTLSClusterConfig,
			// }),
			// g.Entry("120s minReadySeconds ClusterIP cluster with non-TLS traffic", entry{
			// 	minReadySeconds:      120,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("120s minReadySeconds ClusterIP cluster with TLS traffic", entry{
			// 	minReadySeconds:      120,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeClusterIP,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypeServiceClusterIP,
			// 	makeClusterConfig:    makeTLSClusterConfig,
			// }),
			// g.Entry("120s minReadySeconds PodIP cluster with non-TLS traffic", entry{
			// 	minReadySeconds:      120,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeNonTLSClusterConfig,
			// }),
			// g.Entry("120s minReadySeconds PodIP cluster with TLS traffic", entry{
			// 	minReadySeconds:      120,
			// 	nodeServiceType:      scyllav1.NodeServiceTypeHeadless,
			// 	nodesBroadcastType:   scyllav1.BroadcastAddressTypePodIP,
			// 	clientsBroadcastType: scyllav1.BroadcastAddressTypePodIP,
			// 	makeClusterConfig:    makeTLSClusterConfig,
			// }),
		)
	})

	g.It("should reconcile resource changes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Changing pod resources")
		oldResources := *sc.Spec.Datacenter.Racks[0].Resources.DeepCopy()
		newResources := *oldResources.DeepCopy()
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceMemory))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceMemory))

		newResources.Requests[corev1.ResourceCPU] = *addQuantity(newResources.Requests[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Requests[corev1.ResourceMemory] = *addQuantity(newResources.Requests[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Requests).NotTo(o.BeEquivalentTo(oldResources.Requests))

		newResources.Limits[corev1.ResourceCPU] = *addQuantity(newResources.Limits[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Limits[corev1.ResourceMemory] = *addQuantity(newResources.Limits[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Limits).NotTo(o.BeEquivalentTo(oldResources.Limits))

		o.Expect(newResources).NotTo(o.BeEquivalentTo(oldResources))

		newResourcesJSON, err := json.Marshal(newResources)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources", "value": %s}]`,
				newResourcesJSON,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Resources).To(o.BeEquivalentTo(newResources))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		o.Expect(hosts).To(o.ConsistOf(getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)))
		verifyCQLData(ctx, di)

		framework.By("Scaling the ScyllaCluster up to create a new replica")
		oldMembers := sc.Spec.Datacenter.Racks[0].Members
		newMebmers := oldMembers + 1
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`,
				newMebmers,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.Equal(newMebmers))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(int(oldMembers)))
		o.Expect(hosts).To(o.HaveLen(int(newMebmers)))
		o.Expect(hosts).To(o.ContainElements(oldHosts))
		verifyCQLData(ctx, di)
	})
})
