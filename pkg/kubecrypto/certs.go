package kubecrypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"reflect"
	"time"

	"github.com/google/go-cmp/cmp"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

func needsRefresh(existingCert *x509.Certificate, now time.Time, refresh time.Duration, desiredCert *x509.Certificate, issuerPublicKey any, secretRef klog.ObjectRef) string {
	// Don't check notBefore to avoid issues on time skew.
	// notAfter is fine as the cert should never be close to it.
	if now.After(existingCert.NotAfter) {
		return "already expired"
	}

	validity := existingCert.NotAfter.Sub(existingCert.NotBefore)
	at80Percent := existingCert.NotAfter.Add(-validity / 5)
	if now.After(at80Percent) {
		return fmt.Sprintf("past its latest possible refresh time %v", at80Percent)
	}

	refreshDate := existingCert.NotBefore.Add(refresh)
	if now.After(refreshDate) {
		return fmt.Sprintf("past its refresh time %v", refreshDate)
	}

	existingCertTemplate := ocrypto.ExtractDesiredFieldsFromTemplate(existingCert)
	desiredCertTemplate := ocrypto.ExtractDesiredFieldsFromTemplate(desiredCert)
	if !reflect.DeepEqual(desiredCertTemplate, existingCertTemplate) {
		klog.V(2).InfoS(
			"Existing certificate template differs from the desired one",
			"Secret", secretRef,
			"Diff", cmp.Diff(existingCertTemplate, desiredCertTemplate),
		)
		return "certificate needs an update"
	}

	existingIssuerHash := existingCert.AuthorityKeyId
	desiredIssuerHash := getAuthorityKeyIDFromSignerKey(issuerPublicKey)

	if !reflect.DeepEqual(desiredIssuerHash, existingIssuerHash) {
		klog.V(2).InfoS(
			"Issuers key hashes differ",
			"Secret", secretRef,
			"Existing64", base64.StdEncoding.EncodeToString(existingIssuerHash),
			"Desired64", base64.StdEncoding.EncodeToString(desiredIssuerHash),
		)
		return "issuer changed, new cert needs to be signed"
	}

	return ""
}

// extractExistingSecret extracts certificates and the key along with a refresh reason, if any.
func extractExistingSecretCertData(secret *corev1.Secret) ([]byte, []byte, string) {
	if secret.Data == nil {
		return nil, nil, "missing data"
	}

	certBytes := secret.Data[corev1.TLSCertKey]
	if len(certBytes) == 0 {
		return nil, nil, "missing cert data"
	}

	keyBytes := secret.Data[corev1.TLSPrivateKeyKey]
	if len(keyBytes) == 0 {
		return nil, nil, "missing key data"
	}

	return certBytes, keyBytes, ""
}

func extractExistingSecret(
	secret *corev1.Secret,
	now time.Time,
	refresh time.Duration,
	desiredCert *x509.Certificate,
	desiredIssuerKey any,
) ([]byte, []byte, *x509.Certificate, string) {
	if secret.Data == nil {
		return nil, nil, nil, "missing data"
	}

	certBytes, privateKeyBytes, refreshReason := extractExistingSecretCertData(secret)
	if len(refreshReason) != 0 {
		return nil, nil, nil, refreshReason
	}

	certs, err := ocrypto.DecodeCertificates(certBytes)
	if err != nil {
		return nil, nil, nil, fmt.Sprintf("can't decode TLS certificates from secret %q: %v", naming.ObjRef(secret), err)
	}

	if len(certs) == 0 {
		return nil, nil, nil, "no cert present"
	}

	cert := certs[0]

	// Validate cert dates and force refresh if needed.
	refreshReason = needsRefresh(cert, now, refresh, desiredCert, desiredIssuerKey, klog.KObj(secret))
	if len(refreshReason) != 0 {
		return nil, nil, nil, refreshReason
	}

	return certBytes, privateKeyBytes, cert, ""
}

func getAuthorityKeyIDFromSignerKey(key any) []byte {
	// Virtual signers, like SelfSignedSigner will have an empty key.
	if key == nil {
		return nil
	}

	var keyBytes []byte
	var err error
	
	switch k := key.(type) {
	case *rsa.PublicKey:
		if k == nil {
			return nil
		}
		keyBytes = x509.MarshalPKCS1PublicKey(k)
	case *ecdsa.PublicKey:
		if k == nil {
			return nil
		}
		keyBytes, err = x509.MarshalPKIXPublicKey(k)
		if err != nil {
			klog.ErrorS(err, "Failed to marshal ECDSA public key")
			return nil
		}
	default:
		klog.Warningf("Unsupported public key type: %T", k)
		return nil
	}

	h := sha1.Sum(keyBytes)
	return h[:]
}

func makeCertificate(ctx context.Context, name string, certCreator ocrypto.CertCreator, keyGetter ocrypto.KeyGetter, signer ocrypto.Signer, validity, refresh time.Duration, controller metav1.Object, controllerGVK schema.GroupVersionKind, existingSecret *corev1.Secret) (*TLSSecret, error) {
	if certCreator == nil {
		return nil, fmt.Errorf("missing cert creator")
	}

	if signer == nil {
		return nil, fmt.Errorf("missing signer")
	}

	if controller == nil {
		return nil, fmt.Errorf("missing controller object")
	}

	if controllerGVK.Empty() {
		return nil, fmt.Errorf("controller GVK can't be empty")
	}

	if validity == 0 {
		return nil, fmt.Errorf("validity can't be zero")
	}

	if refresh == 0 {
		return nil, fmt.Errorf("validity can't be zero")
	}

	if refresh > validity {
		return nil, fmt.Errorf("refresh can't be greater then validity")
	}

	existingSecret = existingSecret.DeepCopy()
	now := signer.Now()

	tlsSecret := NewTLSSecret(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   controller.GetNamespace(),
			Name:        name,
			Annotations: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(controller, controllerGVK),
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{},
	})

	refreshReason := "needs new cert"
	if existingSecret != nil {
		// Copy all annotations over, so even custom annotations make it through.
		tlsSecret.GetSecret().Annotations = helpers.MergeMaps(tlsSecret.GetSecret().Annotations, existingSecret.Annotations)

		var certBytes, keyBytes []byte
		var existingCertificate *x509.Certificate
		certBytes, keyBytes, existingCertificate, refreshReason = extractExistingSecret(
			existingSecret,
			now,
			refresh,
			certCreator.MakeCertificateTemplate(now, validity),
			signer.GetPublicKey(),
		)
		if refreshReason == "" {
			tlsSecret.GetSecret().Data[corev1.TLSCertKey] = certBytes
			tlsSecret.GetSecret().Data[corev1.TLSPrivateKeyKey] = keyBytes
			tlsSecret.SetCertsCache([]*x509.Certificate{existingCertificate})
		}
	}

	if len(refreshReason) != 0 {
		startTime := time.Now()
		klog.V(2).InfoS("Creating certificate", "Secret", naming.ObjRef(tlsSecret.GetSecret()), "Reason", refreshReason)
		
		// Try to use the generic interface that supports both RSA and ECDSA
		var cert *x509.Certificate
		var key any
		var err error
		
		// Check if certCreator supports MakeCertificateAny (for both RSA and ECDSA)
		if x509Creator, ok := certCreator.(*ocrypto.X509CertCreator); ok {
			cert, key, err = x509Creator.MakeCertificateAny(ctx, keyGetter, signer, validity)
		} else {
			// Fallback to RSA-only path for backward compatibility
			rsaKeyGetter, ok := keyGetter.(ocrypto.RSAKeyGetter)
			if !ok {
				return nil, fmt.Errorf("keyGetter does not implement RSAKeyGetter and cert creator is not X509CertCreator")
			}
			cert, key, err = certCreator.MakeCertificate(ctx, rsaKeyGetter, signer, validity)
		}
		
		if err != nil {
			return nil, fmt.Errorf("can't create certificate: %w", err)
		}
		klog.V(2).InfoS("Certificate created", "Secret", naming.ObjRef(tlsSecret.GetSecret()), "ElapsedTime", time.Now().Sub(startTime))

		certBytes, err := ocrypto.EncodeCertificates(cert)
		if err != nil {
			return nil, fmt.Errorf("can't encode certificates: %w", err)
		}

		keyBytes, err := ocrypto.EncodePrivateKeyAny(key)
		if err != nil {
			return nil, fmt.Errorf("can't encode key: %w", err)
		}

		tlsSecret.GetSecret().Data[corev1.TLSCertKey] = certBytes
		tlsSecret.GetSecret().Data[corev1.TLSPrivateKeyKey] = keyBytes
		tlsSecret.SetCache([]*x509.Certificate{cert}, key)

		tlsSecret.GetSecret().Annotations[certsRefreshReasonKey] = refreshReason
	}

	certs, key, err := tlsSecret.GetCertsKey()
	if err != nil {
		return nil, err
	}

	for annotationKey, annotationFunc := range CertKeyProjectedAnnotations {
		tlsSecret.GetSecret().Annotations[annotationKey], err = annotationFunc(certs, key)
		if err != nil {
			return nil, fmt.Errorf("can't project annotation %q: %w", annotationKey, err)
		}
	}

	return tlsSecret, nil
}

func MakeSelfSignedCA(ctx context.Context, name string, certCreator ocrypto.CertCreator, keyGetter ocrypto.RSAKeyGetter, nowFunc func() time.Time, validity, refresh time.Duration, controller metav1.Object, controllerGVK schema.GroupVersionKind, existingSecret *corev1.Secret) (*SigningTLSSecret, error) {
	signer := ocrypto.NewSelfSignedSigner(nowFunc)
	tlsSecret, err := makeCertificate(ctx, name, certCreator, keyGetter, signer, validity, refresh, controller, controllerGVK, existingSecret)
	if err != nil {
		return nil, err
	}

	return NewSigningTLSSecret(tlsSecret, nowFunc), nil
}
