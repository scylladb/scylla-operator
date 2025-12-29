package kubecrypto

import (
	"context"
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
	certs, err := s.GetCerts()
	if err != nil {
		return nil, fmt.Errorf("can't get issuer certs from secret %q: %v", naming.ObjRef(s.GetSecret()), err)
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates in secret %q", naming.ObjRef(s.GetSecret()))
	}

	signerCert := certs[0]

	// Get the key - use cached key if available (supports RSA and ECDSA)
	var signerKey any
	if s.key != nil {
		signerKey = s.key
	} else {
		// Load from secret - defaults to RSA for backward compatibility
		rsaKey, err := s.GetKey()
		if err != nil {
			return nil, fmt.Errorf("can't get issuer key from secret %q: %v", naming.ObjRef(s.GetSecret()), err)
		}
		signerKey = rsaKey
	}

	ca, err := ocrypto.NewCertificateAuthorityWithAnyKey(signerCert, signerKey, s.nowFunc)
	if err != nil {
		return nil, fmt.Errorf("can't create certificate authority from secret %q: %v", naming.ObjRef(s.GetSecret()), err)
	}

	return ca, nil
}

func (s *SigningTLSSecret) MakeCertificate(ctx context.Context, name string, certCreator ocrypto.CertCreator, keyGetter ocrypto.KeyGetter, controller *metav1.ObjectMeta, controllerGVK schema.GroupVersionKind, existingSecret *corev1.Secret, validity, refresh time.Duration) (*TLSSecret, error) {
	ca, err := s.AsCertificateAuthority()
	if err != nil {
		return nil, err
	}

	cert, err := makeCertificate(ctx, name, certCreator, keyGetter, ca, validity, refresh, controller, controllerGVK, existingSecret)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
