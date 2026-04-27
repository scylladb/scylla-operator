package verification

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

type TLSCertOptions struct {
	IsCA     *bool
	KeyUsage *x509.KeyUsage
}

func VerifyAndParseTLSCert(secret *corev1.Secret, options TLSCertOptions) ([]*x509.Certificate, []byte, crypto.Signer, []byte) {
	o.Expect(secret.Type).To(o.Equal(corev1.SecretType("kubernetes.io/tls")))
	o.Expect(secret.Data).To(o.HaveKey("tls.crt"))
	o.Expect(secret.Data).To(o.HaveKey("tls.key"))

	certsBytes := secret.Data["tls.crt"]
	keyBytes := secret.Data["tls.key"]
	o.Expect(certsBytes).NotTo(o.BeEmpty())
	o.Expect(keyBytes).NotTo(o.BeEmpty())

	certs, key, err := ocrypto.GetTLSCertificatesFromBytes(certsBytes, keyBytes)
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(certs).NotTo(o.BeEmpty())
	o.Expect(certs[0].IsCA).To(o.Equal(*options.IsCA))
	o.Expect(certs[0].KeyUsage).To(o.Equal(*options.KeyUsage))

	o.Expect(key).NotTo(o.BeNil())
	o.Expect(key.Public()).NotTo(o.BeNil())
	if rsaKey, ok := key.(*rsa.PrivateKey); ok {
		o.Expect(rsaKey.Validate()).To(o.Succeed())
	} else if ecdsaKey, ok := key.(*ecdsa.PrivateKey); ok {
		_, err := ecdsaKey.ECDH()
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return certs, certsBytes, key, keyBytes
}

func VerifyAndParseCABundle(cm *corev1.ConfigMap) ([]*x509.Certificate, []byte) {
	o.Expect(cm.Data).To(o.HaveKey("ca-bundle.crt"))

	bundleBytes := cm.Data["ca-bundle.crt"]
	o.Expect(bundleBytes).NotTo(o.BeEmpty())

	certs, err := ocrypto.DecodeCertificates([]byte(bundleBytes))
	o.Expect(err).NotTo(o.HaveOccurred())

	return certs, []byte(bundleBytes)
}

// VerifyScyllaClusterTLSOptions configures how TLS certificates are verified.
type VerifyScyllaClusterTLSOptions struct {
	// CAKeyUsage is the expected KeyUsage for CA certificates.
	CAKeyUsage x509.KeyUsage
	// LeafKeyUsage is the expected KeyUsage for leaf (serving/client) certificates.
	LeafKeyUsage x509.KeyUsage
}

// VerifyScyllaClusterTLSResult holds the parsed TLS artifacts from verification
// so callers can perform additional checks (e.g., CQL connection config verification).
type VerifyScyllaClusterTLSResult struct {
	ServingCACertBytes   []byte
	AdminClientCertBytes []byte
	AdminClientKeyBytes  []byte
}

// VerifyScyllaClusterTLSCertificates verifies all TLS secrets and performs live TLS handshakes
// to each ScyllaDB node. It is algorithm-agnostic: the caller provides expected KeyUsage values
// appropriate for the key type (RSA or ECDSA).
func VerifyScyllaClusterTLSCertificates(
	ctx context.Context,
	coreClient corev1client.CoreV1Interface,
	sc *scyllav1.ScyllaCluster,
	hosts []string,
	hostIDs []string,
	options VerifyScyllaClusterTLSOptions,
) VerifyScyllaClusterTLSResult {
	namespace := sc.Namespace

	framework.By("Verifying TLS API objects")

	clientCASecret, err := coreClient.Secrets(namespace).Get(ctx, fmt.Sprintf("%s-local-client-ca", sc.Name), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	clientCACerts, _, _, _ := VerifyAndParseTLSCert(clientCASecret, TLSCertOptions{
		IsCA:     pointer.Ptr(true),
		KeyUsage: pointer.Ptr(options.CAKeyUsage),
	})
	o.Expect(clientCACerts).To(o.HaveLen(1))

	servingCASecret, err := coreClient.Secrets(namespace).Get(ctx, fmt.Sprintf("%s-local-serving-ca", sc.Name), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	_, _, _, _ = VerifyAndParseTLSCert(servingCASecret, TLSCertOptions{
		IsCA:     pointer.Ptr(true),
		KeyUsage: pointer.Ptr(options.CAKeyUsage),
	})

	servingCABundleConfigMap, err := coreClient.ConfigMaps(namespace).Get(ctx, fmt.Sprintf("%s-local-serving-ca", sc.Name), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	servingCACerts, servingCACertBytes := VerifyAndParseCABundle(servingCABundleConfigMap)
	o.Expect(servingCACerts).NotTo(o.BeEmpty())

	servingCertSecret, err := coreClient.Secrets(namespace).Get(ctx, fmt.Sprintf("%s-local-serving-certs", sc.Name), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	servingCerts, _, _, _ := VerifyAndParseTLSCert(servingCertSecret, TLSCertOptions{
		IsCA:     pointer.Ptr(false),
		KeyUsage: pointer.Ptr(options.LeafKeyUsage),
	})

	adminClientSecret, err := coreClient.Secrets(namespace).Get(ctx, fmt.Sprintf("%s-local-user-admin", sc.Name), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	_, adminClientCertBytes, _, adminClientKeyBytes := VerifyAndParseTLSCert(adminClientSecret, TLSCertOptions{
		IsCA:     pointer.Ptr(false),
		KeyUsage: pointer.Ptr(options.LeafKeyUsage),
	})

	framework.By("Verifying serving certificate SANs")

	var sniHosts []string
	for _, domain := range sc.Spec.DNSDomains {
		sniHosts = append(sniHosts, fmt.Sprintf("cql.%s", domain))

		for _, hostID := range hostIDs {
			sniHosts = append(sniHosts, fmt.Sprintf("%s.cql.%s", hostID, domain))
		}
	}

	var serviceServingDNSNames []string
	services, err := coreClient.Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, svc := range services.Items {
		serviceServingDNSNames = append(serviceServingDNSNames, fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace))
	}

	serviceAndPodIPs, err := utils.GetNodesServiceAndPodIPs(ctx, coreClient, sc)
	o.Expect(err).NotTo(o.HaveOccurred())

	expectedServiceAndPodIPsNum := 2 * int(utils.GetMemberCount(sc))
	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.NodeService != nil && sc.Spec.ExposeOptions.NodeService.Type == scyllav1.NodeServiceTypeHeadless {
		expectedServiceAndPodIPsNum = int(utils.GetMemberCount(sc))
	}
	o.Expect(serviceAndPodIPs).To(o.HaveLen(expectedServiceAndPodIPsNum))

	identityServiceIP, err := utils.GetIdentityServiceIP(ctx, coreClient, sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(identityServiceIP).NotTo(o.BeEmpty())

	allIPs := make([]string, 0, len(serviceAndPodIPs)+1)
	allIPs = append(allIPs, serviceAndPodIPs...)
	allIPs = append(allIPs, identityServiceIP)

	hostsIPs, err := helpers.ParseIPs(allIPs)
	o.Expect(err).NotTo(o.HaveOccurred())

	servingDNSNames := make([]string, 0, len(sniHosts)+len(serviceServingDNSNames))
	servingDNSNames = append(servingDNSNames, sniHosts...)
	servingDNSNames = append(servingDNSNames, serviceServingDNSNames...)

	// Check the serving cert content first to distinguish whether the cert was correctly reloaded by ScyllaDB or not.
	o.Expect(servingCerts[0].Subject.CommonName).To(o.BeEmpty())
	o.Expect(helpers.NormalizeIPs(servingCerts[0].IPAddresses)).To(o.ConsistOf(hostsIPs))
	o.Expect(servingCerts[0].DNSNames).To(o.ConsistOf(servingDNSNames))

	framework.By("Verifying live TLS connection to each ScyllaDB node")

	// Now check the cert used by ScyllaDB.
	servingCAPool := x509.NewCertPool()
	for _, caCert := range servingCACerts {
		servingCAPool.AddCert(caCert)
	}
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

	return VerifyScyllaClusterTLSResult{
		ServingCACertBytes:   servingCACertBytes,
		AdminClientCertBytes: adminClientCertBytes,
		AdminClientKeyBytes:  adminClientKeyBytes,
	}
}
