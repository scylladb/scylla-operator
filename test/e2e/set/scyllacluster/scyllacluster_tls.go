// Copyright (C) 2022 ScyllaDB

package scyllacluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	verificationutils "github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/verification"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

var (
	deploymentResourceInfo = collect.ResourceInfo{
		Resource: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Scope: meta.RESTScopeNamespace,
	}
)

const (
	operatorNamespace      = "scylla-operator"
	operatorDeploymentName = "scylla-operator"
)

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

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

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		initialHosts, initialHostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialHosts).To(o.HaveLen(1))
		di := verificationutils.InsertAndVerifyCQLData(ctx, initialHosts)
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

				framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
				waitCtxL1, waitCtxL1Cancel := utils.ContextForRollout(ctx, sc)
				defer waitCtxL1Cancel()
				sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtxL1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
				o.Expect(err).NotTo(o.HaveOccurred())

				scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
				scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

				hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(hosts).To(o.HaveLen(tc.replicas))
				o.Expect(hostIDs).To(o.HaveLen(tc.replicas))
				o.Expect(hostIDs).To(o.ContainElements(initialHostIDs))

				framework.By("Verifying TLS certificates and live TLS connections")

				ecdsaCAKeyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
				ecdsaLeafKeyUsage := x509.KeyUsageDigitalSignature

				tlsResult := verification.VerifyScyllaClusterTLSCertificates(ctx, f.KubeClient().CoreV1(), sc, hosts, hostIDs, verification.VerifyScyllaClusterTLSOptions{
					CAKeyUsage:   ecdsaCAKeyUsage,
					LeafKeyUsage: ecdsaLeafKeyUsage,
				})

				adminClientConnectionConfigsSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-local-cql-connection-configs-admin", sc.Name), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_ = scyllaclusterverification.VerifyAndParseCQLConnectionConfigs(adminClientConnectionConfigsSecret, scyllaclusterverification.VerifyCQLConnectionConfigsOptions{
					Domains:               sc.Spec.DNSDomains,
					Datacenters:           []string{sc.Spec.Datacenter.Name},
					ServingCAData:         tlsResult.ServingCACertBytes,
					ClientCertificateData: tlsResult.AdminClientCertBytes,
					ClientKeyData:         tlsResult.AdminClientKeyBytes,
				})
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

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		initialHosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialHosts).To(o.HaveLen(1))
		di := verificationutils.InsertAndVerifyCQLData(ctx, initialHosts)
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
				cert, err = crypto.SignCertificate(cert, key.Public(), issuerCert, issuerKey)
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
				secret, err = controllerhelpers.WaitForSecretState(waitCtxL1, f.KubeClient().CoreV1().Secrets(sc.Namespace), secret.Name, controllerhelpers.WaitForStateOptions{}, func(s *corev1.Secret) (bool, error) {
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
					configMap, err = controllerhelpers.WaitForConfigMapState(waitCtxL2, f.KubeClient().CoreV1().ConfigMaps(sc.Namespace), item.cmName, controllerhelpers.WaitForStateOptions{}, func(cm *corev1.ConfigMap) (bool, error) {
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

// setOperatorCryptoKeyType sets --crypto-key-type on the operator Deployment, or removes it when keyType is empty,
// and waits for the operator to roll out.
func setOperatorCryptoKeyType(ctx context.Context, f *framework.Framework, keyType crypto.KeyType) {
	g.GinkgoHelper()

	const (
		cryptoKeyTypeFlagPrefix    = "--crypto-key-type="
		cryptoRSAKeySizeFlagPrefix = "--crypto-rsa-key-size="
	)

	operatorDeploy, err := f.KubeAdminClient().AppsV1().Deployments(operatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	containerIdx := slices.IndexFunc(operatorDeploy.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == "scylla-operator"
	})
	o.Expect(containerIdx).NotTo(o.Equal(-1), "operator container not found in Deployment")

	args := slices.DeleteFunc(slices.Clone(operatorDeploy.Spec.Template.Spec.Containers[containerIdx].Args), func(arg string) bool {
		return strings.HasPrefix(arg, cryptoKeyTypeFlagPrefix) || strings.HasPrefix(arg, cryptoRSAKeySizeFlagPrefix)
	})
	if len(keyType) != 0 {
		args = append(args, cryptoKeyTypeFlagPrefix+string(keyType))
	}
	if keyType == crypto.RSAKeyType {
		// Use the smallest supported RSA key size, so that the operator generates the keys quickly.
		args = append(args, cryptoRSAKeySizeFlagPrefix+"2048")
	}

	patchBytes, err := json.Marshal([]map[string]any{
		{
			"op":    "replace",
			"path":  fmt.Sprintf("/spec/template/spec/containers/%d/args", containerIdx),
			"value": args,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	operatorDeploy, err = f.KubeAdminClient().AppsV1().Deployments(operatorNamespace).Patch(
		ctx,
		operatorDeploymentName,
		types.JSONPatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the operator Deployment to roll out (RV=%s)", operatorDeploy.ResourceVersion)
	waitCtx, waitCtxCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer waitCtxCancel()
	_, err = controllerhelpers.WaitForDeploymentState(
		waitCtx,
		f.KubeAdminClient().AppsV1().Deployments(operatorNamespace),
		operatorDeploymentName,
		controllerhelpers.WaitForStateOptions{},
		controllerhelpers.IsDeploymentRolledOut,
		// Make sure no operator replica with the previous configuration is left to reconcile.
		func(deploy *appsv1.Deployment) (bool, error) {
			return deploy.Status.Replicas == *deploy.Spec.Replicas, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// createRolledOutScyllaCluster creates a single-node ScyllaCluster and waits for it to roll out.
func createRolledOutScyllaCluster(ctx context.Context, f *framework.Framework) *scyllav1.ScyllaCluster {
	g.GinkgoHelper()

	framework.By("Creating a ScyllaCluster")
	sc := f.GetDefaultScyllaCluster()
	o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
	sc.Spec.Datacenter.Racks[0].Members = 1

	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
	scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

	return sc
}

// waitForCertificatesToUseKeyAlgorithm waits until the certificates of the ScyllaCluster's serving chain (CA and serving
// certificate) and client chain (CA and admin client certificate) have keys of the given algorithm.
func waitForCertificatesToUseKeyAlgorithm(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster, algorithm x509.PublicKeyAlgorithm) {
	g.GinkgoHelper()

	secretNames := []string{
		naming.GetScyllaClusterLocalServingCAName(sc.Name),
		naming.GetScyllaClusterLocalClientCAName(sc.Name),
		naming.GetScyllaClusterLocalServingCertName(sc.Name),
		naming.GetScyllaClusterLocalUserAdminCertName(sc.Name),
	}

	o.Eventually(func(eo o.Gomega) {
		for _, name := range secretNames {
			secret, err := f.KubeClient().CoreV1().Secrets(sc.Namespace).Get(ctx, name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			certs, err := crypto.DecodeCertificates(secret.Data[corev1.TLSCertKey])
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(certs).NotTo(o.BeEmpty())
			eo.Expect(certs[0].PublicKeyAlgorithm).To(o.Equal(algorithm), "certificate in Secret %q", name)
		}
	}).WithContext(ctx).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(o.Succeed())
}

// verifyAdminCanQueryOverTLS runs a CQL query over TLS with the admin client certificate, verifying ScyllaDB with the
// serving CA bundle. A TLS handshake alone doesn't show that ScyllaDB accepts the client certificate.
func verifyAdminCanQueryOverTLS(ctx context.Context, hosts []string, certificates verification.VerifyScyllaClusterTLSResult) {
	g.GinkgoHelper()

	servingCAPool := x509.NewCertPool()
	o.Expect(servingCAPool.AppendCertsFromPEM(certificates.ServingCACertBytes)).To(o.BeTrue())

	adminCert, err := tls.X509KeyPair(certificates.AdminClientCertBytes, certificates.AdminClientKeyBytes)
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Eventually(func(eo o.Gomega) {
		cluster := gocql.NewCluster(hosts...)
		cluster.Port = scylla.DefaultNativeTransportPortSSL
		cluster.SslOpts = &gocql.SslOptions{
			Config: &tls.Config{
				RootCAs:      servingCAPool,
				Certificates: []tls.Certificate{adminCert},
			},
			EnableHostVerification: true,
		}
		session, err := cluster.CreateSession()
		eo.Expect(err).NotTo(o.HaveOccurred())
		defer session.Close()

		var key string
		eo.Expect(session.Query("SELECT key FROM system.local").WithContext(ctx).Scan(&key)).To(o.Succeed())
	}).WithContext(ctx).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(o.Succeed())
}

var _ = g.Describe("ScyllaCluster certificate key type", framework.SuiteSerial, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should rotate RSA certificates to ECDSA keys when --crypto-key-type=RSA is removed", func(ctx g.SpecContext) {
		if !utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
			g.Skip(fmt.Sprintf("Skipping because %q feature is disabled", features.AutomaticTLSCertificates))
		}

		framework.By("Snapshotting the operator Deployment for restoration")
		f.AddCleaners(framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			deploymentResourceInfo,
			operatorNamespace,
			operatorDeploymentName,
			framework.RestoreStrategyUpdate,
		))

		framework.By("Configuring the operator to use RSA keys")
		setOperatorCryptoKeyType(ctx, f, crypto.RSAKeyType)

		sc := createRolledOutScyllaCluster(ctx, f)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying the serving and client certificates use RSA keys")
		expectedRSACertificates := verification.VerifyScyllaClusterTLSOptions{
			CAKeyUsage:         x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			LeafKeyUsage:       x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			PublicKeyAlgorithm: pointer.Ptr(x509.RSA),
		}
		verification.VerifyScyllaClusterTLSCertificates(ctx, f.KubeClient().CoreV1(), sc, hosts, hostIDs, expectedRSACertificates)

		framework.By("Configuring the operator with the default key type")
		setOperatorCryptoKeyType(ctx, f, "")

		framework.By("Waiting for the serving and client certificates to be reissued with ECDSA keys")
		waitForCertificatesToUseKeyAlgorithm(ctx, f, sc, x509.ECDSA)

		framework.By("Verifying the serving and client certificates use ECDSA keys")
		expectedECDSACertificates := verification.VerifyScyllaClusterTLSOptions{
			CAKeyUsage:         x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			LeafKeyUsage:       x509.KeyUsageDigitalSignature,
			PublicKeyAlgorithm: pointer.Ptr(x509.ECDSA),
		}
		ecdsaCertificates := verification.VerifyScyllaClusterTLSCertificates(ctx, f.KubeClient().CoreV1(), sc, hosts, hostIDs, expectedECDSACertificates)

		framework.By("Verifying ScyllaDB accepts CQL queries with the ECDSA admin client certificate")
		verifyAdminCanQueryOverTLS(ctx, hosts, ecdsaCertificates)
	}, g.NodeTimeout(testTimeout))
})
