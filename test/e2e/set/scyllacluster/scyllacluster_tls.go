// Copyright (C) 2022 ScyllaDB

package scyllacluster

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/verification"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should setup and maintain up to date TLS certificates", func() {
		if !utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
			g.Skip(fmt.Sprintf("Skipping because %q feature is disabled", features.AutomaticTLSCertificates))
		}

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating an initial ScyllaCluster with a single node")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(
			ctx,
			sc,
			metav1.CreateOptions{
				FieldManager: f.FieldManager(),
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		initialHosts, initialHostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialHosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, initialHosts)
		defer di.Close()

		for _, tc := range []struct {
			domains  []string
			replicas int
		}{
			{
				domains:  nil,
				replicas: 1,
			},
			{
				domains:  []string{"foo.scylladb.com", "bar.scylladb.com"},
				replicas: 1,
			},
			{
				domains:  []string{"foo.scylladb.com", "bar.scylladb.com"},
				replicas: 2,
			},
			{
				domains:  []string{"foo.scylladb.com", "bar.scylladb.com"},
				replicas: 1,
			},
			{
				domains:  nil,
				replicas: 1,
			},
		} {
			tc := tc
			func() {
				framework.By("Scaling the ScyllaCluster to %d replicas setting domains to %q", tc.replicas, tc.domains)
				sc.ManagedFields = nil
				sc.ResourceVersion = ""
				sc.Spec.Datacenter.Racks[0].Members = int32(tc.replicas)
				sc.Spec.DNSDomains = tc.domains
				scData, err := runtime.Encode(scheme.Codecs.LegacyCodec(scyllav1.GroupVersion), sc)
				o.Expect(err).NotTo(o.HaveOccurred())
				// TODO: Use generated Apply method when our clients have it.
				sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.ApplyPatchType, scData, metav1.PatchOptions{
					FieldManager: f.FieldManager(),
					Force:        pointer.Ptr(true),
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(tc.replicas))
				o.Expect(sc.Spec.DNSDomains).To(o.BeEquivalentTo(tc.domains))

				framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
				waitCtxL1, waitCtxL1Cancel := utils.ContextForRollout(ctx, sc)
				defer waitCtxL1Cancel()
				sc, err = utils.WaitForScyllaClusterState(waitCtxL1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				verifyScyllaCluster(ctx, f.KubeClient(), sc)
				waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

				hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(hosts).To(o.HaveLen(tc.replicas))
				o.Expect(hostIDs).To(o.HaveLen(tc.replicas))
				o.Expect(hostIDs).To(o.ContainElements(initialHostIDs))

				framework.By("Verifying TLS API objects")

				clientCASecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-client-ca", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				clientCACerts, _, _, _ := verification.VerifyAndParseTLSCert(clientCASecret, verification.TLSCertOptions{
					IsCA:     pointer.Ptr(true),
					KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign),
				})
				o.Expect(clientCACerts).To(o.HaveLen(1))

				servingCASecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-serving-ca", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, _, _, _ = verification.VerifyAndParseTLSCert(servingCASecret, verification.TLSCertOptions{
					IsCA:     pointer.Ptr(true),
					KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign),
				})

				servingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-serving-ca", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				servingCACerts, servingCACertBytes := verification.VerifyAndParseCABundle(servingCABundleConfigMap)
				o.Expect(servingCACerts).To(o.HaveLen(1))

				servingCertSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-serving-certs", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				servingCerts, _, _, _ := verification.VerifyAndParseTLSCert(servingCertSecret, verification.TLSCertOptions{
					IsCA:     pointer.Ptr(false),
					KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
				})

				adminClientSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-user-admin", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, adminClientCertBytes, _, adminClientKeyBytes := verification.VerifyAndParseTLSCert(adminClientSecret, verification.TLSCertOptions{
					IsCA:     pointer.Ptr(false),
					KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
				})

				adminClientConnectionConfigsSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-cql-connection-configs-admin", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_ = verifyAndParseCQLConnectionConfigs(adminClientConnectionConfigsSecret, verifyCQLConnectionConfigsOptions{
					domains:               sc.Spec.DNSDomains,
					datacenters:           []string{sc.Spec.Datacenter.Name},
					ServingCAData:         servingCACertBytes,
					ClientCertificateData: adminClientCertBytes,
					ClientKeyData:         adminClientKeyBytes,
				})

				framework.By("Verifying certificates")

				var sniHosts []string
				for _, domain := range sc.Spec.DNSDomains {
					sniHosts = append(sniHosts, fmt.Sprintf("cql.%s", domain))

					for _, hostID := range hostIDs {
						sniHosts = append(sniHosts, fmt.Sprintf("%s.cql.%s", hostID, domain))
					}
				}
				o.Expect(sniHosts).To(o.HaveLen(len(sc.Spec.DNSDomains) + len(sc.Spec.DNSDomains)*tc.replicas))

				var serviceServingDNSNames []string
				services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(naming.ClusterLabels(sc)).String(),
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				for _, svc := range services.Items {
					serviceServingDNSNames = append(serviceServingDNSNames, fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace))
				}

				serviceAndPodIPs, err := utils.GetNodesServiceAndPodIPs(ctx, f.KubeClient().CoreV1(), sc)
				o.Expect(err).NotTo(o.HaveOccurred())

				expectedServiceAndPodIPsNum := 2 * int(utils.GetMemberCount(sc))
				if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.NodeService != nil && sc.Spec.ExposeOptions.NodeService.Type == scyllav1.NodeServiceTypeHeadless {
					expectedServiceAndPodIPsNum = int(utils.GetMemberCount(sc))
				}
				o.Expect(serviceAndPodIPs).To(o.HaveLen(expectedServiceAndPodIPsNum))

				hostsIPs, err := helpers.ParseIPs(serviceAndPodIPs)
				o.Expect(err).NotTo(o.HaveOccurred())

				servingDNSNames := make([]string, 0, len(sniHosts)+len(serviceServingDNSNames))
				servingDNSNames = append(servingDNSNames, sniHosts...)
				servingDNSNames = append(servingDNSNames, serviceServingDNSNames...)

				// Check the serving cert content first to distinguish whether the cert was correctly reloaded by ScyllaDB or not.
				o.Expect(servingCerts[0].Subject.CommonName).To(o.BeEmpty())
				o.Expect(helpers.NormalizeIPs(servingCerts[0].IPAddresses)).To(o.ConsistOf(hostsIPs))
				o.Expect(servingCerts[0].DNSNames).To(o.ConsistOf(servingDNSNames))

				// Now check the cert used by ScyllaDB.
				servingCAPool := x509.NewCertPool()
				servingCAPool.AddCert(servingCACerts[0])
				adminTLSCert, err := tls.X509KeyPair(adminClientCertBytes, adminClientKeyBytes)
				o.Expect(err).NotTo(o.HaveOccurred())

				for _, nodeAddress := range hosts {
					framework.Infof("Starting to probe node %q for correct certs", nodeAddress)

					o.Eventually(func(eo o.Gomega) {
						serverCerts, err := utils.GetServerTLSCertificates(fmt.Sprintf("%s:9142", nodeAddress), &tls.Config{
							ServerName:         nodeAddress,
							InsecureSkipVerify: false,
							Certificates:       []tls.Certificate{adminTLSCert},
							RootCAs:            servingCAPool,
						})
						eo.Expect(err).NotTo(o.HaveOccurred())
						eo.Expect(serverCerts).NotTo(o.BeEmpty())

						eo.Expect(serverCerts[0].Subject.CommonName).To(o.BeEmpty())
						eo.Expect(helpers.NormalizeIPs(serverCerts[0].IPAddresses)).To(o.ConsistOf(hostsIPs))
						eo.Expect(serverCerts[0].DNSNames).To(o.ConsistOf(servingDNSNames))
					}).WithTimeout(5 * 60 * time.Second).WithPolling(1 * time.Second).Should(o.Succeed())

					framework.Infof("Node %q has correct certs", nodeAddress)
				}
			}()
		}
	})

	g.It("should rotate TLS certificates before they expire", func() {
		if !utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
			g.Skip(fmt.Sprintf("Skipping because %q feature is disabled", features.AutomaticTLSCertificates))
		}

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		initialHosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialHosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, initialHosts)
		defer di.Close()

		// This test rotates CAs which makes it dependent on the order.
		// Child certs need to be tested first before the issuer changes.
		items := []struct {
			secretName string
			issuerName string
			cmName     string
		}{
			{
				secretName: fmt.Sprintf("%s-local-serving-certs", sc.Name),
				issuerName: fmt.Sprintf("%s-local-serving-ca", sc.Name),
				cmName:     "",
			},
			{
				secretName: fmt.Sprintf("%s-local-serving-ca", sc.Name),
				issuerName: fmt.Sprintf("%s-local-serving-ca", sc.Name),
				cmName:     fmt.Sprintf("%s-local-serving-ca", sc.Name),
			},
			{
				secretName: fmt.Sprintf("%s-local-user-admin", sc.Name),
				issuerName: fmt.Sprintf("%s-local-user-admin", sc.Name),
				cmName:     "",
			},
			{
				secretName: fmt.Sprintf("%s-local-client-ca", sc.Name),
				issuerName: fmt.Sprintf("%s-local-client-ca", sc.Name),
				cmName:     fmt.Sprintf("%s-local-client-ca", sc.Name),
			},
		}

		for _, item := range items {
			func() {
				framework.By("Checkpointing secret %q", item.secretName)

				initialSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, item.secretName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(initialSecret.Data).To(o.HaveKey("tls.crt"))
				o.Expect(initialSecret.Data).To(o.HaveKey("tls.key"))
				o.Expect(initialSecret.Data["tls.crt"]).NotTo(o.BeEmpty())
				o.Expect(initialSecret.Data["tls.key"]).NotTo(o.BeEmpty())

				var configMap *corev1.ConfigMap
				var initialBundleCert *x509.Certificate
				if len(item.cmName) != 0 {
					framework.By("Checkpointing configmap %q", item.secretName)
					configMap, err = f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, item.cmName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					bundleCerts, err := kubecrypto.GetCABundleFromConfigMap(configMap)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(bundleCerts).To(o.HaveLen(1))
					initialBundleCert = bundleCerts[0]
				}

				framework.By("Replacing secret %q cert to be past its latest refresh time (90%%)", item.secretName)

				initialCert, err := kubecrypto.GetCertFromSecret(initialSecret.DeepCopy())
				o.Expect(err).NotTo(o.HaveOccurred())

				cert, key, err := kubecrypto.GetCertKeyFromSecret(initialSecret.DeepCopy())
				o.Expect(err).NotTo(o.HaveOccurred())

				issuerSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, item.issuerName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				issuerCert, issuerKey, err := kubecrypto.GetCertKeyFromSecret(issuerSecret)
				o.Expect(err).NotTo(o.HaveOccurred())

				now := time.Now()
				// We can pick any validity long enough for the test.
				validity := 10 * time.Hour
				cert.NotAfter = now.Add(validity / 10)
				cert.NotBefore = now.Add(-validity / 10 * 9)
				o.Expect(cert.NotAfter.Sub(cert.NotBefore)).To(o.Equal(validity))

				// For self-signed certs we can reuse the key.
				cert, err = crypto.SignCertificate(cert, key.Public().(*rsa.PublicKey), issuerCert, issuerKey)
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(cert.AuthorityKeyId).To(o.Equal(initialCert.AuthorityKeyId))

				pemBytes, err := crypto.EncodeCertificates(cert)
				o.Expect(err).NotTo(o.HaveOccurred())

				secret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Patch(
					ctx,
					initialSecret.Name,
					types.JSONPatchType,
					[]byte(fmt.Sprintf(`[{"op": "replace", "path": "/data/tls.crt", "value": %q}]`, base64.StdEncoding.EncodeToString(pemBytes))),
					metav1.PatchOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.By("Waiting for Secret %q to be updated", secret.Name)
				waitCtxL1, waitCtxL1Cancel := context.WithTimeout(ctx, utils.SyncTimeout)
				defer waitCtxL1Cancel()
				secret, err = utils.WaitForSecretState(waitCtxL1, f.KubeClient().CoreV1().Secrets(sc.Namespace), secret.Name, utils.WaitForStateOptions{}, func(s *corev1.Secret) (bool, error) {
					return !apiequality.Semantic.DeepEqual(s.Data, secret.Data), nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(secret.Annotations).To(o.HaveKeyWithValue(
					"certificates.internal.scylla-operator.scylladb.com/refresh-reason",
					o.MatchRegexp("^past its latest possible refresh time .*"),
				))
				o.Expect(secret.Data).To(o.HaveKey("tls.crt"))
				o.Expect(secret.Data).To(o.HaveKey("tls.key"))
				o.Expect(secret.Data["tls.crt"]).NotTo(o.Equal(initialSecret.Data["tls.crt"]))
				o.Expect(secret.Data["tls.key"]).NotTo(o.Equal(initialSecret.Data["tls.key"]))

				cert, _, err = kubecrypto.GetCertKeyFromSecret(secret.DeepCopy())
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Infof("New certificate issued from %v to %v", cert.NotBefore, cert.NotAfter)
				o.Expect(cert.NotBefore.After(now.Add(-2 * time.Second))).To(o.BeTrue())

				if len(item.cmName) != 0 {
					framework.By("Waiting for ConfigMap %q to be updated", item.cmName)

					waitCtxL2, waitCtxL2Cancel := context.WithTimeout(ctx, utils.SyncTimeout)
					defer waitCtxL2Cancel()
					configMap, err = utils.WaitForConfigMapState(waitCtxL2, f.KubeClient().CoreV1().ConfigMaps(sc.Namespace), item.cmName, utils.WaitForStateOptions{}, func(cm *corev1.ConfigMap) (bool, error) {
						return !apiequality.Semantic.DeepEqual(configMap.Data, cm.Data), nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())

					bundleCerts, err := kubecrypto.GetCABundleFromConfigMap(configMap)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(bundleCerts).To(o.HaveLen(2))
					o.Expect(bundleCerts).To(o.ContainElements(initialBundleCert))
				}
			}()
		}
	})
})
