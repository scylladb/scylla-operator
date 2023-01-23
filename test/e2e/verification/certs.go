package verification

import (
	"crypto"
	"crypto/x509"

	o "github.com/onsi/gomega"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	corev1 "k8s.io/api/core/v1"
)

type TLSCertOptions struct {
	IsCA     *bool
	KeyUsage *x509.KeyUsage
}

func VerifyAndParseTLSCert(secret *corev1.Secret, options TLSCertOptions) ([]*x509.Certificate, []byte, crypto.PrivateKey, []byte) {
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

	o.Expect(key.Validate()).To(o.Succeed())

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
