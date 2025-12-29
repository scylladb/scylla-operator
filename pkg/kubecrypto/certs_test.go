package kubecrypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/crypto/testfiles"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type testSecretData struct {
	old     *corev1.Secret
	current *corev1.Secret
}

type verifySecretDataFuncType func(*testing.T, *testSecretData)

func now() time.Time {
	return time.Date(2021, 02, 01, 00, 00, 00, 00, time.UTC)
}

func isSelfSigned(cert *x509.Certificate) bool {
	if len(cert.AuthorityKeyId) == 0 {
		return true
	}

	return reflect.DeepEqual(cert.AuthorityKeyId, cert.SubjectKeyId)
}

func verifySelfSignedCert(t *testing.T, d *testSecretData) {
	t.Helper()

	cert, err := NewTLSSecret(d.current).GetCert()
	if err != nil {
		t.Fatal(err)
	}

	if !isSelfSigned(cert) {
		t.Errorf("certificate isn't self-signed")
	}
}

func verifyChildCert(t *testing.T, d *testSecretData) {
	t.Helper()

	cert, err := NewTLSSecret(d.current).GetCert()
	if err != nil {
		t.Fatal(err)
	}

	if isSelfSigned(cert) {
		t.Errorf("certificate is self-signed")
	}
}

func verifyTLSSecret(t *testing.T, d *testSecretData) {
	t.Helper()

	if len(d.current.Data["tls.crt"]) == 0 {
		t.Errorf("secret is missing TLS certificate")
	}

	if len(d.current.Data["tls.key"]) == 0 {
		t.Errorf("secret is missing TLS key")
	}

	if len(d.current.Data) != 2 {
		t.Errorf("secret has extra keys")
	}
}

func verifyCert(t *testing.T, d *testSecretData, expectedCA bool, expectedKeyUsage x509.KeyUsage) {
	t.Helper()

	verifyTLSSecret(t, d)

	cert, err := NewTLSSecret(d.current).GetCert()
	if err != nil {
		t.Fatal(err)
	}

	if !cert.BasicConstraintsValid {
		t.Errorf("certificate doesn't have basic constraints")
	}

	if cert.IsCA != expectedCA {
		t.Errorf("expected isCA to be %t, got %t", expectedCA, cert.IsCA)
	}

	if !reflect.DeepEqual(cert.KeyUsage, expectedKeyUsage) {
		t.Errorf("expected key usage %v, got %v", expectedKeyUsage, cert.KeyUsage)
	}
}

func verifyCA(t *testing.T, d *testSecretData) {
	t.Helper()
	verifyCert(t, d, true, x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign)
}

func verifyClientCert(t *testing.T, d *testSecretData) {
	t.Helper()
	verifyCert(t, d, false, x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature)
}

func isCertDataChanged(d *testSecretData) bool {
	if d.current == nil || d.old == nil {
		return d.current == d.old
	}

	return apiequality.Semantic.DeepEqual(d.current.Data, d.old.Data)
}

func verifyCertDataChanged(t *testing.T, d *testSecretData) {
	t.Helper()
	if isCertDataChanged(d) {
		t.Errorf("certificate stayed unchanged")
	}
}

func verifyCertDataUnchanged(t *testing.T, d *testSecretData) {
	t.Helper()
	if !isCertDataChanged(d) {
		t.Errorf("certificate changed: %s", cmp.Diff(d.old.Data, d.current.Data))
	}
}

func Test_makeCertificate(t *testing.T) {
	tt := []struct {
		name                    string
		caNamespace             string
		caName                  string
		certCreator             ocrypto.CertCreator
		signer                  ocrypto.Signer
		validity                time.Duration
		refresh                 time.Duration
		controller              metav1.Object
		controllerGVK           schema.GroupVersionKind
		existingSecret          *corev1.Secret
		expectedError           error
		expectedSecret          *corev1.Secret
		expectedSecretDataFuncs []verifySecretDataFuncType
	}{
		{
			name:   "generates new self-signed CA when none exists",
			caName: "ca",
			certCreator: (&ocrypto.CACertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "My CA certificate",
				},
			}).ToCreator(),
			signer:   ocrypto.NewSelfSignedSigner(now),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: nil,
			expectedError:  nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=My CA certificate",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
					},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataChanged,
				verifyCA,
				verifySelfSignedCert,
			},
		},
		{
			name:   "generates new client cert signed by this CA",
			caName: "ca",
			certCreator: (&ocrypto.ClientCertCreatorConfig{
				DNSNames: []string{"my.client.certificate.org"},
			}).ToCreator(),
			signer: helpers.Must(ocrypto.NewCertificateAuthority(
				helpers.Must(ocrypto.DecodeCertificates(testfiles.AlphaCACertBytes))[0],
				helpers.Must(ocrypto.DecodePrivateKey(testfiles.AlphaCAKeyBytes)),
				now,
			)),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: nil,
			expectedError:  nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "false",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
					},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataChanged,
				verifyClientCert,
				verifyChildCert,
			},
		},
		{
			name:   "reuses existing self-signed CA when valid",
			caName: "ca",
			certCreator: (&ocrypto.CACertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "test.ca-name",
				},
			}).ToCreator(),
			signer: ocrypto.NewSelfSignedSigner(func() time.Time {
				return now().Add(1 * time.Second)
			}),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
						"custom": "foo",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					UID: "uid-that-should-never-make-it-to-the-desired-object",
					// Random creation timestamp will make sure it won't make it over to the desired secret.
					CreationTimestamp: metav1.NewTime(time.Date(2022, 01, 01, 00, 00, rand.Intn(60), 00, time.UTC)),
				},
				Data: map[string][]byte{
					"tls.crt": testfiles.AlphaCACertBytes,
					"tls.key": testfiles.AlphaCAKeyBytes,
				},
				Type: "kubernetes.io/tls",
			},
			expectedError: nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
						"custom": "foo",
					},
					UID:               "",
					CreationTimestamp: metav1.Time{},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataUnchanged,
				verifyCA,
				verifySelfSignedCert,
			},
		},
		{
			name:   "reuses existing serving cert when valid",
			caName: "ca",
			certCreator: (&ocrypto.ClientCertCreatorConfig{
				DNSNames: []string{"my.client.certificate.org"},
			}).ToCreator(),
			signer: helpers.Must(ocrypto.NewCertificateAuthority(
				helpers.Must(ocrypto.DecodeCertificates(testfiles.AlphaCACertBytes))[0],
				helpers.Must(ocrypto.DecodePrivateKey(testfiles.AlphaCAKeyBytes)),
				now,
			)),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
						"custom": "foo",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					UID: "uid-that-should-never-make-it-to-the-desired-object",
					// Random creation timestamp will make sure it won't make it over to the desired secret.
					CreationTimestamp: metav1.NewTime(time.Date(2022, 01, 01, 00, 00, rand.Intn(60), 00, time.UTC)),
				},
				Data: map[string][]byte{
					"tls.crt": testfiles.AlphaServingCertBytes,
					"tls.key": testfiles.AlphaServingKeyBytes,
				},
				Type: "kubernetes.io/tls",
			},
			expectedError: nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "false",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
						"custom": "foo",
					},
					UID:               "",
					CreationTimestamp: metav1.Time{},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataUnchanged,
				verifyClientCert,
				verifyChildCert,
			},
		},
		{
			name:   "reuses existing CA and fixes annotations",
			caName: "ca",
			certCreator: (&ocrypto.CACertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "test.ca-name",
				},
			}).ToCreator(),
			signer:   ocrypto.NewSelfSignedSigner(now),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					Annotations: map[string]string{
						"custom": "foo",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					UID: "uid-that-should-never-make-it-to-the-desired-object",
					// Random creation timestamp will make sure it won't make it over to the desired secret.
					CreationTimestamp: metav1.NewTime(time.Date(2022, 01, 01, 00, 00, rand.Intn(60), 00, time.UTC)),
				},
				Data: map[string][]byte{
					"tls.crt": testfiles.AlphaCACertBytes,
					"tls.key": testfiles.AlphaCAKeyBytes,
				},
				Type: "kubernetes.io/tls",
			},
			expectedError: nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":         "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":    "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":     "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":         "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":        "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits": "4096",
						"custom": "foo",
					},
					UID:               "",
					CreationTimestamp: metav1.Time{},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataUnchanged,
				verifyCA,
				verifySelfSignedCert,
			},
		},
		{
			name:   "rotates existing self-signed CA when expired",
			caName: "ca",
			certCreator: (&ocrypto.CACertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "My CA",
				},
			}).ToCreator(),
			signer: ocrypto.NewSelfSignedSigner(func() time.Time {
				return now().Add(42 * 365 * 24 * time.Hour)
			}),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2021-01-31T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2021-02-01T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=test.ca-name",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
						"custom": "foo",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					UID: "uid-that-should-never-make-it-to-the-desired-object",
					// Random creation timestamp will make sure it won't make it over to the desired secret.
					CreationTimestamp: metav1.NewTime(time.Date(2022, 01, 01, 00, 00, rand.Intn(60), 00, time.UTC)),
				},
				Data: map[string][]byte{
					"tls.crt": testfiles.AlphaCACertBytes,
					"tls.key": testfiles.AlphaCAKeyBytes,
				},
				Type: "kubernetes.io/tls",
			},
			expectedError: nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2063-01-21T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2063-01-22T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=My CA",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "already expired",
						"custom": "foo",
					},
					UID:               "",
					CreationTimestamp: metav1.Time{},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataChanged,
				verifyCA,
				verifySelfSignedCert,
			},
		},
		{
			name:   "refreshes existing self-signed CA with broken annotations when actually expired",
			caName: "ca",
			certCreator: (&ocrypto.CACertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "My CA",
				},
			}).ToCreator(),
			signer: ocrypto.NewSelfSignedSigner(func() time.Time {
				return now().Add(42 * 365 * 24 * time.Hour)
			}),
			validity: 1 * time.Hour,
			refresh:  50 * time.Minute,
			controller: &metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "sc",
				UID:       "42",
			},
			controllerGVK: schema.GroupVersionKind{
				Group:   "scylla.scylladb.com",
				Version: "v1",
				Kind:    "ScyllaCluster",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "42",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "broken",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "broken",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "won't make it through",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=CA na that gets properly filled",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "needs new cert",
						"custom": "foo",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					UID: "uid-that-should-never-make-it-to-the-desired-object",
					// Random creation timestamp will make sure it won't make it over to the desired secret.
					CreationTimestamp: metav1.NewTime(time.Date(2022, 01, 01, 00, 00, rand.Intn(60), 00, time.UTC)),
				},
				Data: map[string][]byte{
					"tls.crt": testfiles.AlphaCACertBytes,
					"tls.key": testfiles.AlphaCAKeyBytes,
				},
				Type: "kubernetes.io/tls",
			},
			expectedError: nil,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "ca",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "sc",
							UID:                "42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
					Annotations: map[string]string{
						"certificates.internal.scylla-operator.scylladb.com/count":          "1",
						"certificates.internal.scylla-operator.scylladb.com/not-before":     "2063-01-21T23:59:59Z",
						"certificates.internal.scylla-operator.scylladb.com/not-after":      "2063-01-22T01:00:00Z",
						"certificates.internal.scylla-operator.scylladb.com/is-ca":          "true",
						"certificates.internal.scylla-operator.scylladb.com/issuer":         "CN=My CA",
						"certificates.internal.scylla-operator.scylladb.com/key-size-bits":  "4096",
						"certificates.internal.scylla-operator.scylladb.com/refresh-reason": "already expired",
						"custom": "foo",
					},
					UID:               "",
					CreationTimestamp: metav1.Time{},
				},
				Type: "kubernetes.io/tls",
			},
			expectedSecretDataFuncs: []verifySecretDataFuncType{
				verifyCertDataChanged,
				verifyCA,
				verifySelfSignedCert,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			keygen, err := ocrypto.NewRSAKeyGenerator(1, 1, 4096, 42*time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			defer keygen.Close()

			var wg sync.WaitGroup
			defer wg.Wait()

			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			wg.Add(1)
			go func() {
				defer wg.Done()
				keygen.Run(ctx)
			}()

			got, err := makeCertificate(
				ctx,
				tc.caName,
				tc.certCreator,
				keygen,
				tc.signer,
				tc.validity,
				tc.refresh,
				tc.controller,
				tc.controllerGVK,
				tc.existingSecret,
			)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected error %v, got %v", tc.expectedError, err)
			}

			if tc.expectedError != nil {
				if tc.signer.VerifyCertificate(helpers.Must(got.GetCert())) != nil {
					t.Errorf("certificate isn't signed by this signer: %v", err)
				}
			}

			secret := got.GetSecret()
			for _, f := range tc.expectedSecretDataFuncs {
				f(t, &testSecretData{
					old:     tc.existingSecret,
					current: secret,
				},
				)
			}
			secret.Data = nil
			if !apiequality.Semantic.DeepEqual(secret, tc.expectedSecret) {
				t.Errorf("expected and got differ: %s", cmp.Diff(tc.expectedSecret, secret))
			}
		})
	}
}

func Test_getAuthorityKeyIDFromSignerKey(t *testing.T) {
	tt := []struct {
		name       string
		key        *rsa.PublicKey
		expectedID []byte
	}{
		{
			name:       "nil key return empty id",
			key:        nil,
			expectedID: nil,
		},
		{
			name:       "real self-signed cert and key",
			key:        &helpers.Must(ocrypto.DecodePrivateKey(testfiles.AlphaCAKeyBytes)).PublicKey,
			expectedID: helpers.Must(ocrypto.DecodeCertificates(testfiles.AlphaCACertBytes))[0].SubjectKeyId,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := getAuthorityKeyIDFromSignerKey(tc.key)
			if !reflect.DeepEqual(got, tc.expectedID) {
				t.Errorf("expected %q, got %q", tc.expectedID, got)
			}
		})
	}
}

func TestMakeCertificate_ECDSA(t *testing.T) {
t.Parallel()

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Create ECDSA key generator
ecdsaKeyGen, err := ocrypto.NewECDSAKeyGenerator(1, 1, elliptic.P384(), 1*time.Second)
if err != nil {
t.Fatal(err)
}
defer ecdsaKeyGen.Close()

wg := sync.WaitGroup{}
wg.Add(1)
go func() {
defer wg.Done()
ecdsaKeyGen.Run(ctx)
}()
defer wg.Wait()

// Create certificate creator
certCreator := &ocrypto.X509CertCreator{
Subject: pkix.Name{
CommonName: "test-ecdsa-cert",
},
IsCA:     false,
KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
}

// Create self-signed signer
signer := ocrypto.NewSelfSignedSigner(now)

// Make certificate with ECDSA key
cert, key, err := certCreator.MakeCertificateAny(ctx, ecdsaKeyGen, signer, 24*time.Hour)
if err != nil {
t.Fatalf("failed to create ECDSA certificate: %v", err)
}

// Verify certificate was created
if cert == nil {
t.Fatal("expected certificate, got nil")
}

// Verify key is ECDSA
_, ok := key.(*ecdsa.PrivateKey)
if !ok {
t.Fatalf("expected ECDSA key, got %T", key)
}

// Verify certificate uses ECDSA signature algorithm
if cert.SignatureAlgorithm != x509.ECDSAWithSHA384 {
t.Errorf("expected ECDSAWithSHA384 signature algorithm, got %v", cert.SignatureAlgorithm)
}

// Verify certificate subject
if cert.Subject.CommonName != "test-ecdsa-cert" {
t.Errorf("expected common name 'test-ecdsa-cert', got %q", cert.Subject.CommonName)
}
}

func TestMakeSelfSignedCA_ECDSA(t *testing.T) {
t.Parallel()

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Create ECDSA key generator
ecdsaKeyGen, err := ocrypto.NewECDSAKeyGenerator(1, 1, elliptic.P256(), 1*time.Second)
if err != nil {
t.Fatal(err)
}
defer ecdsaKeyGen.Close()

wg := sync.WaitGroup{}
wg.Add(1)
go func() {
defer wg.Done()
ecdsaKeyGen.Run(ctx)
}()
defer wg.Wait()

// Create CA certificate creator
caCertCreator := &ocrypto.X509CertCreator{
Subject: pkix.Name{
CommonName: "test-ecdsa-ca",
},
IsCA:     true,
KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
}

controller := &metav1.ObjectMeta{
Namespace: "test-namespace",
Name:      "test-controller",
UID:       "test-uid",
}
controllerGVK := schema.GroupVersionKind{
Group:   "test.scylladb.com",
Version: "v1",
Kind:    "TestController",
}

// Create self-signed CA with ECDSA key
caTLSSecret, err := MakeSelfSignedCA(
ctx,
"test-ecdsa-ca",
caCertCreator,
ecdsaKeyGen,
now,
365*24*time.Hour, // validity
180*24*time.Hour, // refresh
controller,
controllerGVK,
nil, // no existing secret
)
if err != nil {
t.Fatalf("failed to create self-signed CA with ECDSA: %v", err)
}

// Verify CA certificate was created
if caTLSSecret == nil {
t.Fatal("expected CA TLS secret, got nil")
}

caCert, err := caTLSSecret.GetCert()
if err != nil {
t.Fatalf("failed to get CA certificate: %v", err)
}

// Verify certificate is a CA
if !caCert.IsCA {
t.Error("expected certificate to be a CA")
}

// Verify certificate uses ECDSA signature algorithm
if caCert.SignatureAlgorithm != x509.ECDSAWithSHA384 {
t.Errorf("expected ECDSAWithSHA384 signature algorithm, got %v", caCert.SignatureAlgorithm)
}

// Verify certificate subject
if caCert.Subject.CommonName != "test-ecdsa-ca" {
t.Errorf("expected common name 'test-ecdsa-ca', got %q", caCert.Subject.CommonName)
}

// Verify certificate is self-signed
if !isSelfSigned(caCert) {
t.Error("expected self-signed certificate")
}
}
