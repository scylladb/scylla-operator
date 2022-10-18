package kubecrypto

import (
	"fmt"
	"time"

	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SigningTLSSecret struct {
	TLSSecret
	nowFunc func() time.Time
}

func NewSigningTLSSecret(tlsSecret *TLSSecret, nowFunc func() time.Time) *SigningTLSSecret {
	return &SigningTLSSecret{
		TLSSecret: *tlsSecret,
		nowFunc:   nowFunc,
	}
}

func (s *SigningTLSSecret) AsCertificateAuthority() (*ocrypto.CertificateAuthority, error) {
	signerCert, signerKey, err := s.GetCertKey()
	if err != nil {
		return nil, fmt.Errorf("can't get issuer cert key from secret %q: %v", naming.ObjRef(s.GetSecret()), err)
	}

	ca, err := ocrypto.NewCertificateAuthority(signerCert, signerKey, s.nowFunc)
	if err != nil {
		return nil, fmt.Errorf("can't create certificate authority from secret %q: %v", naming.ObjRef(s.GetSecret()), err)
	}

	return ca, nil
}

func (s *SigningTLSSecret) MakeCertificate(name string, certCreator ocrypto.CertCreator, controller *metav1.ObjectMeta, controllerGVK schema.GroupVersionKind, existingSecret *corev1.Secret, validity, refresh time.Duration) (*TLSSecret, error) {
	ca, err := s.AsCertificateAuthority()
	if err != nil {
		return nil, err
	}

	cert, err := makeCertificate(name, certCreator, ca, validity, refresh, controller, controllerGVK, existingSecret)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
