// Copyright (c) 2021 ScyllaDB

package operator

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

const (
	pollInterval = 1 * time.Second
	pollTimeout  = 3 * time.Second
)

func TestMain(m *testing.M) {
	dynamiccertificates.FileRefreshDuration = 1 * time.Second
	os.Exit(m.Run())
}

func TestWebhookOptionsRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tt := []struct {
		Name           string
		WebhookOptions *WebhookOptions
		ExpectedError  error
	}{
		{
			Name: "valid webhook options",
			WebhookOptions: func() *WebhookOptions {
				wo := NewWebhookOptions(genericclioptions.IOStreams{}, DefaultValidators)
				wo.Port = 65535
				wo.InsecureGenerateLocalhostCerts = true

				return wo
			}(),
			ExpectedError: nil,
		},
		{
			Name: "invalid port",
			WebhookOptions: func() *WebhookOptions {
				wo := NewWebhookOptions(genericclioptions.IOStreams{}, DefaultValidators)
				wo.Port = 65536
				wo.InsecureGenerateLocalhostCerts = true

				return wo
			}(),
			ExpectedError: fmt.Errorf("can't create listener: %w", &net.OpError{
				Op:  "listen",
				Net: "tcp",
				Err: &net.AddrError{
					Err:  "invalid port",
					Addr: "65536",
				},
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			err := tc.WebhookOptions.Complete()
			if err != nil {
				t.Fatalf("can't complete WebhookOptions: %v", err)
			}

			err = tc.WebhookOptions.run(ctx, genericclioptions.IOStreams{
				Out:    os.Stdout,
				ErrOut: os.Stderr,
			})
			if !reflect.DeepEqual(tc.ExpectedError, err) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.ExpectedError, err))
			}
		})
	}
}

func TestWebhookOptionsRunWithReload(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	tlsCertFile := dir + "/tls.crt"
	tlsKeyFile := dir + "/tls.key"

	cc, err := newCertificateCreator()
	if err != nil {
		t.Fatalf("can't create certificateCreator: %v", err)
	}

	initialCertSerialNumber := big.NewInt(1)
	encodedInitialCert, err := cc.generateEncodedCertificate(initialCertSerialNumber)
	if err != nil {
		t.Fatalf("can't create new encoded certificate: %v", err)
	}

	err = createFileWithContent(tlsCertFile, encodedInitialCert.cert)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	err = createFileWithContent(tlsKeyFile, encodedInitialCert.privateKey)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	wo := NewWebhookOptions(genericclioptions.IOStreams{}, DefaultValidators)
	wo.TLSCertFile = tlsCertFile
	wo.TLSKeyFile = tlsKeyFile
	wo.Port = 0
	wo.InsecureGenerateLocalhostCerts = false

	err = wo.Complete()
	if err != nil {
		t.Fatalf("can't complete WebhookOptions: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := wo.run(ctx, genericclioptions.IOStreams{
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		})
		if err != nil {
			t.Errorf("can't run webhook server: %v", err)
		}
	}()

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cc.encodedCA)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certpool,
			},
		},
	}

	// Wait for the listen address to be established.
	<-wo.resolvedListenAddrCh
	addr := fmt.Sprintf("https://%s/readyz", wo.resolvedListenAddr)

	hasExpectedCertificate := func(expectedCertificateSerialNumber *big.Int) func() (bool, error) {
		return func() (bool, error) {
			resp, err := client.Get(addr)
			if err != nil {
				t.Logf("can't make a GET request: %v", err)
				return false, nil
			}

			if !reflect.DeepEqual(expectedCertificateSerialNumber, resp.TLS.PeerCertificates[0].SerialNumber) {
				t.Logf("serial numbers differ: expected %s, actual %s", expectedCertificateSerialNumber, resp.TLS.PeerCertificates[0].SerialNumber)
				return false, nil
			}

			return true, nil
		}
	}

	err = apimachineryutilwait.PollImmediate(pollInterval, pollTimeout, hasExpectedCertificate(initialCertSerialNumber))
	if err != nil {
		t.Fatalf("can't observe the initial certificate: %v", err)
	}

	updatedCertSerialNumber := big.NewInt(2)
	encodedUpdatedCert, err := cc.generateEncodedCertificate(updatedCertSerialNumber)
	if err != nil {
		t.Fatalf("can't create new encoded certificate: %v", err)
	}

	err = createFileWithContent(wo.TLSCertFile, encodedUpdatedCert.cert)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	err = createFileWithContent(wo.TLSKeyFile, encodedUpdatedCert.privateKey)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	err = apimachineryutilwait.PollImmediate(pollInterval, pollTimeout, hasExpectedCertificate(updatedCertSerialNumber))
	if err != nil {
		t.Errorf("certificate wasn't updated: %v", err)
	}
}

func TestPortFlag(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		portFlag      string
		expectedValue int
		expectedError error
	}{
		{
			name:          "port number",
			portFlag:      "1234",
			expectedValue: 1234,
			expectedError: nil,
		},
		{
			name:          "string with tcp://host:port format",
			portFlag:      "tcp://host:1234",
			expectedValue: 1234,
			expectedError: nil,
		},
		{
			name:          "string with tcp://ip:port format",
			portFlag:      "tcp://127.0.0.1:4321",
			expectedValue: 4321,
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pf := portFlag(0)
			gotErr := pf.Set(tc.portFlag)
			if !equality.Semantic.DeepEqual(gotErr, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, gotErr)
			}
			if int(pf) != tc.expectedValue {
				t.Errorf("expected value %v, got %v", tc.expectedValue, pf)
			}
		})
	}
}

type certificateCreator struct {
	caCert       *x509.Certificate
	caPrivateKey *rsa.PrivateKey
	encodedCA    []byte
}

type encodedCertificate struct {
	cert       []byte
	privateKey []byte
}

func newCertificateCreator() (*certificateCreator, error) {
	now := time.Now()
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		NotBefore:    now,
		NotAfter:     now.Add(1 * time.Hour),

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,

		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")},
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("can't generate key: %w", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, caPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("can't create a certificate: %w", err)
	}

	encodedCA, err := pemEncode(caBytes, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("can't encode a certificate: %w", err)
	}

	return &certificateCreator{
		caCert:       caCert,
		caPrivateKey: caPrivateKey,
		encodedCA:    encodedCA,
	}, nil
}

func (cc *certificateCreator) generateEncodedCertificate(serialNumber *big.Int) (*encodedCertificate, error) {
	now := time.Now()
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    now,
		NotAfter:     now.Add(1 * time.Hour),

		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},

		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("can't generate key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cc.caCert, privateKey.Public(), cc.caPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("can't create a certificate: %w", err)
	}

	certEncoded, err := pemEncode(certBytes, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("can't encode a certificate: %w", err)
	}

	privateKeyEncoded, err := pemEncode(x509.MarshalPKCS1PrivateKey(privateKey), "RSA PRIVATE KEY")
	if err != nil {
		return nil, fmt.Errorf("can't encode a private key: %w", err)
	}

	return &encodedCertificate{
		cert:       certEncoded,
		privateKey: privateKeyEncoded,
	}, nil
}

func pemEncode(cert []byte, certType string) ([]byte, error) {
	buff := &bytes.Buffer{}
	err := pem.Encode(buff, &pem.Block{
		Type:  certType,
		Bytes: cert,
	})
	if err != nil {
		return nil, fmt.Errorf("can't PEM encode: %w", err)
	}

	return buff.Bytes(), nil
}

func createFileWithContent(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("can't create file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("can't write to file: %w", err)
	}

	return nil
}

func Test_validate(t *testing.T) {
	t.Parallel()

	testScheme := runtime.NewScheme()

	err := corev1.AddToScheme(testScheme)
	if err != nil {
		t.Fatal(err)
	}

	codecs := serializer.NewCodecFactory(testScheme)
	encoder := unstructured.NewJSONFallbackEncoder(codecs.LegacyCodec(testScheme.PrioritizedVersionsAllGroups()...))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: "test",
		},
	}
	podRaw, err := runtime.Encode(encoder, pod)
	if err != nil {
		t.Fatal(err)
	}

	tt := []struct {
		name             string
		ar               *admissionv1.AdmissionReview
		validators       map[schema.GroupVersionResource]Validator
		expectedWarnings []string
		expectedError    error
	}{
		{
			name: "create, no errors, no warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "CREATE",
						Object:    runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						return nil
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateUpdateFunc")
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						return nil
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnUpdateFunc")
					},
				},
			},
			expectedWarnings: nil,
			expectedError:    nil,
		},
		{
			name: "create, error, no warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "CREATE",
						Object:    runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						return field.ErrorList{
							field.Invalid(field.NewPath("spec"), "value", "error"),
						}
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateUpdateFunc")
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						return nil
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnUpdateFunc")
					},
				},
			},
			expectedWarnings: nil,
			expectedError: apierrors.NewInvalid(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(), "pod", field.ErrorList{
				field.Invalid(field.NewPath("spec"), "value", "error"),
			}),
		},
		{
			name: "create, no errors, warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "CREATE",
						Object:    runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						return nil
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateUpdateFunc")
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						return []string{"warning"}
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnUpdateFunc")
					},
				},
			},
			expectedWarnings: []string{"warning"},
			expectedError:    nil,
		},
		{
			name: "create, errors, warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "CREATE",
						Object:    runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						return field.ErrorList{
							field.Invalid(field.NewPath("spec"), "value", "error"),
						}
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateUpdateFunc")
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						return []string{"warning"}
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnUpdateFunc")
					},
				},
			},
			expectedWarnings: []string{"warning"},
			expectedError: apierrors.NewInvalid(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(), "pod", field.ErrorList{
				field.Invalid(field.NewPath("spec"), "value", "error"),
			}),
		},
		{
			name: "update, no errors, no warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "UPDATE",
						Object:    runtime.RawExtension{Raw: podRaw},
						OldObject: runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateCreateFunc")
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						return nil
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnCreateFunc")
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						return nil
					},
				},
			},
			expectedWarnings: nil,
			expectedError:    nil,
		},
		{
			name: "update, error, no warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "UPDATE",
						Object:    runtime.RawExtension{Raw: podRaw},
						OldObject: runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateCreateFunc")
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						return field.ErrorList{
							field.Invalid(field.NewPath("spec"), "value", "error"),
						}
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnCreateFunc")
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						return nil
					},
				},
			},
			expectedWarnings: nil,
			expectedError: apierrors.NewInvalid(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(), "pod", field.ErrorList{
				field.Invalid(field.NewPath("spec"), "value", "error"),
			}),
		},
		{
			name: "update, no errors, warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "UPDATE",
						Object:    runtime.RawExtension{Raw: podRaw},
						OldObject: runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateCreateFunc")
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						return nil
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnCreateFunc")
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						return []string{"warning"}
					},
				},
			},
			expectedWarnings: []string{"warning"},
			expectedError:    nil,
		},
		{
			name: "update, errors, warnings",
			ar: func() *admissionv1.AdmissionReview {
				return &admissionv1.AdmissionReview{
					Request: &admissionv1.AdmissionRequest{
						UID: "uid",
						Resource: metav1.GroupVersionResource{
							Group:    "",
							Version:  "v1",
							Resource: "pods",
						},
						Operation: "UPDATE",
						Object:    runtime.RawExtension{Raw: podRaw},
						OldObject: runtime.RawExtension{Raw: podRaw},
					},
				}
			}(),
			validators: map[schema.GroupVersionResource]Validator{
				corev1.SchemeGroupVersion.WithResource("pods"): &GenericValidator[*corev1.Pod]{
					ValidateCreateFunc: func(_ *corev1.Pod) field.ErrorList {
						panic("unexpected call to ValidateCreateFunc")
					},
					ValidateUpdateFunc: func(_, _ *corev1.Pod) field.ErrorList {
						return field.ErrorList{
							field.Invalid(field.NewPath("spec"), "value", "error"),
						}
					},
					GetWarningsOnCreateFunc: func(_ *corev1.Pod) []string {
						panic("unexpected call to GetWarningsOnCreateFunc")
					},
					GetWarningsOnUpdateFunc: func(_, _ *corev1.Pod) []string {
						return []string{"warning"}
					},
				},
			},
			expectedWarnings: []string{"warning"},
			expectedError: apierrors.NewInvalid(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(), "pod", field.ErrorList{
				field.Invalid(field.NewPath("spec"), "value", "error"),
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := validate(tc.ar, tc.validators)
			if !reflect.DeepEqual(tc.expectedError, err) {
				t.Fatalf("expected and actual errors differ: %s", cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(tc.expectedWarnings, warnings) {
				t.Errorf("expected and actual warnings differ: %s", cmp.Diff(tc.expectedWarnings, warnings))
			}
		})
	}
}
