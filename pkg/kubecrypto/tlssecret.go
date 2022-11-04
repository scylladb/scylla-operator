package kubecrypto

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"strconv"
	"time"

	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

const (
	CABundleKey           = "ca-bundle.crt"
	certsCountKey         = "certificates.internal.scylla-operator.scylladb.com/count"
	certsNotBeforeKey     = "certificates.internal.scylla-operator.scylladb.com/not-before"
	certsNotAfterKey      = "certificates.internal.scylla-operator.scylladb.com/not-after"
	certsIsCAKey          = "certificates.internal.scylla-operator.scylladb.com/is-ca"
	certsIssuerKey        = "certificates.internal.scylla-operator.scylladb.com/issuer"
	certsRefreshReasonKey = "certificates.internal.scylla-operator.scylladb.com/refresh-reason"
	certsKeySizeBitsKey   = "certificates.internal.scylla-operator.scylladb.com/key-size-bits"
)

var (
	CertProjectedAnnotations = map[string]func([]*x509.Certificate) (string, error){
		certsCountKey: func(certs []*x509.Certificate) (string, error) {
			return strconv.Itoa(len(certs)), nil
		},
		certsNotBeforeKey: func(certs []*x509.Certificate) (string, error) {
			return certs[0].NotBefore.Format(time.RFC3339), nil
		},
		certsNotAfterKey: func(certs []*x509.Certificate) (string, error) {
			return certs[0].NotAfter.Format(time.RFC3339), nil
		},
		certsIsCAKey: func(certs []*x509.Certificate) (string, error) {
			return strconv.FormatBool(certs[0].IsCA), nil
		},
		certsIssuerKey: func(certs []*x509.Certificate) (string, error) {
			return certs[0].Issuer.String(), nil
		},
	}
	CertKeyProjectedAnnotations = helpers.MergeMaps(
		wrapCertProjectionsForCertKey(CertProjectedAnnotations),
		map[string]func([]*x509.Certificate, *rsa.PrivateKey) (string, error){
			certsKeySizeBitsKey: func(certs []*x509.Certificate, key *rsa.PrivateKey) (string, error) {
				return strconv.Itoa(key.Size() * 8), nil
			},
		},
	)
)

func wrapCertProjectionsForCertKey(
	certProjections map[string]func([]*x509.Certificate) (string, error),
) map[string]func([]*x509.Certificate, *rsa.PrivateKey) (string, error) {
	res := make(map[string]func([]*x509.Certificate, *rsa.PrivateKey) (string, error), len(certProjections))

	for k, fi := range CertProjectedAnnotations {
		f := fi
		res[k] = func(certificates []*x509.Certificate, key *rsa.PrivateKey) (string, error) {
			return f(certificates)
		}
	}

	return res
}

func GetCABundleDataFromConfigMap(cm *corev1.ConfigMap) ([]byte, error) {
	if cm.Data == nil {
		return nil, fmt.Errorf("configMap %q doesn't contain any data", naming.ObjRef(cm))
	}

	caBundle, found := cm.Data[CABundleKey]
	if !found {
		return nil, fmt.Errorf("configMap %q is missing data for key %q", naming.ObjRef(cm), CABundleKey)
	}

	if len(caBundle) == 0 {
		return nil, fmt.Errorf("configMap %q is missing ca-bundle content", naming.ObjRef(cm))
	}

	return []byte(caBundle), nil
}

func GetCABundleFromConfigMap(cm *corev1.ConfigMap) ([]*x509.Certificate, error) {
	caBundleData, err := GetCABundleDataFromConfigMap(cm)
	if err != nil {
		return nil, err
	}

	return ocrypto.DecodeCertificates(caBundleData)
}

func GetCertDataFromSecret(secret *corev1.Secret) ([]byte, error) {
	if secret.Data == nil {
		return nil, fmt.Errorf("secret %q doesn't contain any data", naming.ObjRef(secret))
	}

	certBytes := secret.Data[corev1.TLSCertKey]
	if len(certBytes) == 0 {
		return nil, fmt.Errorf("secret %q is missing certificate data", naming.ObjRef(secret))
	}

	return certBytes, nil
}

func GetCertsFromSecret(secret *corev1.Secret) ([]*x509.Certificate, error) {
	certBytes, err := GetCertDataFromSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("can't get certificate bytes from secret %q: %w", naming.ObjRef(secret), err)
	}

	certificates, err := ocrypto.DecodeCertificates(certBytes)
	if err != nil {
		return nil, fmt.Errorf("can't decode TLS certificates from secret %q: %w", naming.ObjRef(secret), err)
	}

	return certificates, nil
}

func GetCertFromSecret(secret *corev1.Secret) (*x509.Certificate, error) {
	certs, err := GetCertsFromSecret(secret)
	if err != nil {
		return nil, err
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("secret %q is missing a certificate", naming.ObjRef(secret))
	}

	return certs[0], nil
}

func GetKeyDataFromSecret(secret *corev1.Secret) ([]byte, error) {
	if secret.Data == nil {
		return nil, fmt.Errorf("secret %q doesn't contain any data", naming.ObjRef(secret))
	}

	keyBytes := secret.Data[corev1.TLSPrivateKeyKey]
	if len(keyBytes) == 0 {
		return nil, fmt.Errorf("secret %q is missing certificate key", naming.ObjRef(secret))
	}

	return keyBytes, nil
}

func GetCertKeyDataFromSecret(secret *corev1.Secret) ([]byte, []byte, error) {
	certBytes, err := GetCertDataFromSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	keyBytes, err := GetKeyDataFromSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	return certBytes, keyBytes, nil
}

func GetKeyFromSecret(secret *corev1.Secret) (*rsa.PrivateKey, error) {
	keyBytes, err := GetKeyDataFromSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("can't get key bytes from secret %q: %w", naming.ObjRef(secret), err)
	}

	privateKey, err := ocrypto.DecodePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("can't decode TLS private key from secret %q: %w", naming.ObjRef(secret), err)
	}

	return privateKey, nil
}

func GetCertsKeyFromSecret(secret *corev1.Secret) ([]*x509.Certificate, *rsa.PrivateKey, error) {
	certs, err := GetCertsFromSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	key, err := GetKeyFromSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	return certs, key, nil
}

func GetCertKeyFromSecret(secret *corev1.Secret) (*x509.Certificate, *rsa.PrivateKey, error) {
	certs, key, err := GetCertsKeyFromSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("secret %q is missing a certificate", naming.ObjRef(secret))
	}

	return certs[0], key, nil
}

type TLSSecret struct {
	secret *corev1.Secret
	certs  []*x509.Certificate
	key    *rsa.PrivateKey
}

func NewTLSSecret(secret *corev1.Secret) *TLSSecret {
	return &TLSSecret{
		secret: secret,
	}
}

func (s *TLSSecret) Refresh(updated *corev1.Secret) {
	*s = *NewTLSSecret(updated)
}

func (s *TLSSecret) GetCerts() ([]*x509.Certificate, error) {
	if s.certs != nil {
		return s.certs, nil
	}

	var err error
	s.certs, err = GetCertsFromSecret(s.secret)
	if err != nil {
		return nil, fmt.Errorf("can't get certs from secret %q: %w", naming.ObjRef(s.secret), err)
	}

	return s.certs, nil
}

func (s *TLSSecret) GetCert() (*x509.Certificate, error) {
	certs, err := s.GetCerts()
	if err != nil {
		return nil, fmt.Errorf("can't get certs from secret %q: %w", naming.ObjRef(s.secret), err)
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("secret %q doesn't contain any certificate", naming.ObjRef(s.secret))
	}

	return s.certs[0], nil
}

func (s *TLSSecret) GetKey() (*rsa.PrivateKey, error) {
	if s.key != nil {
		return s.key, nil
	}

	var err error
	s.key, err = GetKeyFromSecret(s.secret)
	if err != nil {
		return nil, fmt.Errorf("can't get key from secret %q: %w", naming.ObjRef(s.secret), err)
	}

	return s.key, nil
}

func (s *TLSSecret) GetCertsKey() ([]*x509.Certificate, *rsa.PrivateKey, error) {
	certs, err := s.GetCerts()
	if err != nil {
		return nil, nil, err
	}

	key, err := s.GetKey()
	if err != nil {
		return nil, nil, err
	}

	return certs, key, err
}

func (s *TLSSecret) GetCertKey() (*x509.Certificate, *rsa.PrivateKey, error) {
	cert, err := s.GetCert()
	if err != nil {
		return nil, nil, err
	}

	key, err := s.GetKey()
	if err != nil {
		return nil, nil, err
	}

	return cert, key, err
}

func (s *TLSSecret) SetCertsCache(certs []*x509.Certificate) {
	s.certs = certs
}

func (s *TLSSecret) SetKeyCache(key *rsa.PrivateKey) {
	s.key = key
}

func (s *TLSSecret) SetCache(certs []*x509.Certificate, key *rsa.PrivateKey) {
	s.SetCertsCache(certs)
	s.SetKeyCache(key)
}

func (s *TLSSecret) GetSecret() *corev1.Secret {
	return s.secret
}

func (s *TLSSecret) MakeCABundle(name string, controller metav1.Object, controllerGVK schema.GroupVersionKind, existingCM *corev1.ConfigMap, now time.Time) (*corev1.ConfigMap, error) {
	var err error
	var existingCertificates []*x509.Certificate

	if existingCM != nil {
		existingCertificates, err = GetCABundleFromConfigMap(existingCM)
		if err != nil {
			// If the ConfigMap is empty or the data couldn't be decoded we'll act like there are
			// no valid certificates and use only the current certificate as the desired data.
			// This will make sure the controller can always repair the state and valid certs make it though.
			klog.V(2).InfoS(
				"Couldn't extract existing certificates from CABundle. Using only the current certificate",
				"CABundle", klog.KObj(existingCM),
				"Error", err,
			)
		}
	}

	currentCertificate, err := s.GetCert()
	if err != nil {
		return nil, fmt.Errorf("can't get current certificate: %w", err)
	}

	certificates := ocrypto.MakeCABundle(currentCertificate, existingCertificates, now)

	caBundleBytes, err := ocrypto.EncodeCertificates(certificates...)
	if err != nil {
		return nil, fmt.Errorf("can't encode ca bundle bytes: %w", err)
	}

	res := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   s.secret.Namespace,
			Name:        name,
			Annotations: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(controller, controllerGVK),
			},
		},
		Data: map[string]string{
			CABundleKey: string(caBundleBytes),
		},
	}

	return res, nil
}
