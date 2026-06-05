package kubecrypto

import (
	"context"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"reflect"
	"testing"
	"time"

	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	testcrypto "github.com/scylladb/scylla-operator/pkg/test/crypto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func Test_ManageCertificates_staleCacheDoesNotRegenerateCA(t *testing.T) {
	t.Parallel()

	const (
		namespace        = "default"
		caName           = "test-ca"
		caBundleName     = "test-ca"
		servingCertName  = "test-serving-certs"
		testCertValidity = 30 * 24 * time.Hour
		testCertRefresh  = 15 * 24 * time.Hour
		testCAValidity   = 10 * 365 * 24 * time.Hour
		testCARefresh    = 8 * 365 * 24 * time.Hour
	)

	currentTime := time.Now()
	nowFunc := func() time.Time { return currentTime }

	keygen, err := ocrypto.NewECDSAKeyGenerator(1, 1, elliptic.P256(), 42*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	testcrypto.StartKeyGenerator(t, keygen)

	controller := &metav1.ObjectMeta{
		Namespace: namespace,
		Name:      "test-controller",
		UID:       "test-uid",
	}
	controllerGVK := schema.GroupVersionKind{
		Group:   "scylla.scylladb.com",
		Version: "v1alpha1",
		Kind:    "ScyllaDBMonitoring",
	}

	caConfig := &CAConfig{
		MetaConfig: MetaConfig{
			Name: caName,
		},
		Validity: testCAValidity,
		Refresh:  testCARefresh,
	}
	caBundleConfig := &CABundleConfig{
		MetaConfig: MetaConfig{
			Name: caBundleName,
		},
	}
	certConfigs := []*CertificateConfig{
		{
			MetaConfig: MetaConfig{
				Name: servingCertName,
			},
			Validity: testCertValidity,
			Refresh:  testCertRefresh,
			CertCreator: (&ocrypto.ServingCertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "",
				},
				DNSNames: []string{"test.example.com"},
			}).ToCreator(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fakeClient := fake.NewSimpleClientset()

	secretCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	secretLister := corev1listers.NewSecretLister(secretCache)
	configMapCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	configMapLister := corev1listers.NewConfigMapLister(configMapCache)
	recorder := record.NewFakeRecorder(10)

	cm := NewCertificateManager(
		keygen,
		fakeClient.CoreV1(),
		secretLister,
		fakeClient.CoreV1(),
		configMapLister,
		recorder,
	)

	// First call: no existing objects anywhere. Creates CA, bundle, and serving cert.
	err = cm.ManageCertificates(ctx, nowFunc, controller, controllerGVK, caConfig, caBundleConfig, certConfigs, map[string]*corev1.Secret{}, map[string]*corev1.ConfigMap{})
	if err != nil {
		t.Fatalf("first ManageCertificates call failed: %v", err)
	}

	// Record the state after the first call.
	caSecretAfterFirst, err := fakeClient.CoreV1().Secrets(namespace).Get(ctx, caName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("can't get CA secret after first call: %v", err)
	}
	servingSecretAfterFirst, err := fakeClient.CoreV1().Secrets(namespace).Get(ctx, servingCertName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("can't get serving secret after first call: %v", err)
	}
	bundleCMAfterFirst, err := fakeClient.CoreV1().ConfigMaps(namespace).Get(ctx, caBundleName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("can't get CA bundle ConfigMap after first call: %v", err)
	}

	firstBundleCerts, err := ocrypto.DecodeCertificates([]byte(bundleCMAfterFirst.Data[CABundleKey]))
	if err != nil {
		t.Fatalf("can't decode CA bundle after first call: %v", err)
	}
	if len(firstBundleCerts) != 1 {
		t.Fatalf("expected 1 cert in CA bundle after first call, got %d", len(firstBundleCerts))
	}

	// Populate the listers with the objects from the fake client, simulating the informer
	// having caught up by the time ApplySecret/ApplyConfigMap runs inside ManageCertificates.
	for _, s := range []*corev1.Secret{caSecretAfterFirst, servingSecretAfterFirst} {
		if err := secretCache.Add(s); err != nil {
			t.Fatalf("can't add secret %q to cache: %v", s.Name, err)
		}
	}
	if err := configMapCache.Add(bundleCMAfterFirst); err != nil {
		t.Fatalf("can't add ConfigMap %q to cache: %v", bundleCMAfterFirst.Name, err)
	}

	// Second call: empty existingSecrets/existingConfigMaps (simulating a stale snapshot taken
	// before the informer delivered the objects created in the first call), but the listers
	// are populated (the informer caught up between the snapshot and the ApplySecret calls).
	// Without the live-GET fallback this would mint a new CA, producing a 2-cert bundle.
	err = cm.ManageCertificates(ctx, nowFunc, controller, controllerGVK, caConfig, caBundleConfig, certConfigs, map[string]*corev1.Secret{}, map[string]*corev1.ConfigMap{})
	if err != nil {
		t.Fatalf("second ManageCertificates call failed: %v", err)
	}

	// Verify CA Secret data is unchanged.
	caSecretAfterSecond, err := fakeClient.CoreV1().Secrets(namespace).Get(ctx, caName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("can't get CA secret after second call: %v", err)
	}
	if !reflect.DeepEqual(caSecretAfterFirst.Data, caSecretAfterSecond.Data) {
		t.Errorf("CA secret was regenerated on the second call (stale cache should not cause regeneration)")
	}

	// Verify serving cert Secret data is unchanged.
	servingSecretAfterSecond, err := fakeClient.CoreV1().Secrets(namespace).Get(ctx, servingCertName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("can't get serving secret after second call: %v", err)
	}
	if !reflect.DeepEqual(servingSecretAfterFirst.Data, servingSecretAfterSecond.Data) {
		t.Errorf("serving cert secret was regenerated on the second call (stale cache should not cause regeneration)")
	}

	// Verify CA bundle still has exactly 1 cert.
	bundleCMAfterSecond, err := fakeClient.CoreV1().ConfigMaps(namespace).Get(ctx, caBundleName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("can't get CA bundle ConfigMap after second call: %v", err)
	}
	secondBundleCerts, err := ocrypto.DecodeCertificates([]byte(bundleCMAfterSecond.Data[CABundleKey]))
	if err != nil {
		t.Fatalf("can't decode CA bundle after second call: %v", err)
	}
	if len(secondBundleCerts) != 1 {
		t.Fatalf("expected 1 cert in CA bundle after second call, got %d", len(secondBundleCerts))
	}

	// Verify the single cert in the bundle is the same CA cert, not a new one.
	if !certEqual(firstBundleCerts[0], secondBundleCerts[0]) {
		t.Errorf("CA cert in bundle changed between calls")
	}
}

func certEqual(a, b *x509.Certificate) bool {
	return reflect.DeepEqual(a.Raw, b.Raw)
}
