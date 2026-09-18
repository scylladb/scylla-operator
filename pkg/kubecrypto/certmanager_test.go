package kubecrypto

import (
	"context"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	testcrypto "github.com/scylladb/scylla-operator/pkg/test/crypto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

var _ ObjectControl[*corev1.Secret] = ctrlclient.ObjectControl[corev1.Secret, *corev1.Secret]{}

// managedObjects is how a test reaches the objects a CertificateManager manages, whichever way the manager was built.
type managedObjects struct {
	getSecret    func(name string) (*corev1.Secret, error)
	getConfigMap func(name string) (*corev1.ConfigMap, error)
	// syncCaches makes the manager's cached reads see what it has written so far. A no-op where the cached reads and
	// the writes go through the same client.
	syncCaches func()
	// writes counts the creates and updates the manager has issued so far.
	writes func() int
}

// newTypedCertificateManager builds the manager the way the legacy controllers do: typed clients for the writes and
// listers over informer caches for the reads.
func newTypedCertificateManager(t *testing.T, ctx context.Context, keygen ocrypto.KeyGenerator, namespace string) (*CertificateManager, managedObjects) {
	t.Helper()

	kubeClient := fake.NewSimpleClientset()
	secretCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	configMapCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	cm := NewCertificateManager(
		keygen,
		kubeClient.CoreV1(),
		corev1listers.NewSecretLister(secretCache),
		kubeClient.CoreV1(),
		corev1listers.NewConfigMapLister(configMapCache),
		record.NewFakeRecorder(10),
	)

	return cm, managedObjects{
		getSecret: func(name string) (*corev1.Secret, error) {
			return kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		},
		getConfigMap: func(name string) (*corev1.ConfigMap, error) {
			return kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		},
		syncCaches: func() {
			secrets, err := kubeClient.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for i := range secrets.Items {
				if err := secretCache.Add(&secrets.Items[i]); err != nil {
					t.Fatal(err)
				}
			}
			configMaps, err := kubeClient.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for i := range configMaps.Items {
				if err := configMapCache.Add(&configMaps.Items[i]); err != nil {
					t.Fatal(err)
				}
			}
		},
		writes: func() int {
			n := 0
			for _, action := range kubeClient.Actions() {
				if action.GetVerb() == "create" || action.GetVerb() == "update" {
					n++
				}
			}
			return n
		},
	}
}

// newControlCertificateManager builds the manager the way a controller-runtime reconciler does: one client for the
// reads and the writes, wrapped in ctrlclient's object controls; the fake client stands in for the live reader too.
func newControlCertificateManager(t *testing.T, ctx context.Context, keygen ocrypto.KeyGenerator, namespace string) (*CertificateManager, managedObjects) {
	t.Helper()

	writes := 0
	c := ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			writes++
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			writes++
			return c.Update(ctx, obj, opts...)
		},
	}).Build()

	cm := NewCertificateManagerWithControl(
		keygen,
		ctrlclient.NewObjectControl[corev1.Secret](ctx, c, c),
		ctrlclient.NewObjectControl[corev1.ConfigMap](ctx, c, c),
		record.NewFakeRecorder(10),
	)

	return cm, managedObjects{
		getSecret: func(name string) (*corev1.Secret, error) {
			return ctrlclient.Get[corev1.Secret](ctx, c, namespace, name)
		},
		getConfigMap: func(name string) (*corev1.ConfigMap, error) {
			return ctrlclient.Get[corev1.ConfigMap](ctx, c, namespace, name)
		},
		syncCaches: func() {},
		writes:     func() int { return writes },
	}
}

// Whichever way it is built, the manager has to create a CA, its bundle and a serving certificate, and on the next
// run see what it wrote and leave it alone.
func TestCertificateManager_ManageCertificates(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name       string
		newManager func(t *testing.T, ctx context.Context, keygen ocrypto.KeyGenerator, namespace string) (*CertificateManager, managedObjects)
	}{
		{
			// The typed construction lives as long as a controller builds the manager from typed clients and listers;
			// after OPERATOR-418 ports the ScyllaDBDatacenter controller, that is the ScyllaDBMonitoring controller.
			name:       "typed clients and listers",
			newManager: newTypedCertificateManager,
		},
		{
			name:       "controller-runtime client",
			newManager: newControlCertificateManager,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			const namespace = "ns"

			keygen, err := ocrypto.NewECDSAKeyGenerator(1, 1, elliptic.P256(), 42*time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			testcrypto.StartKeyGenerator(t, keygen)

			ctx := t.Context()
			cm, objects := tc.newManager(t, ctx, keygen, namespace)

			controller := &metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "controller",
				UID:       "controller-uid",
			}
			controllerGVK := schema.GroupVersionKind{Group: "scylla.scylladb.com", Version: "v1alpha1", Kind: "ScyllaDBDatacenter"}
			caConfig := &CAConfig{
				MetaConfig: MetaConfig{Name: "ca"},
				Validity:   10 * 365 * 24 * time.Hour,
				Refresh:    8 * 365 * 24 * time.Hour,
			}
			caBundleConfig := &CABundleConfig{
				MetaConfig: MetaConfig{Name: "ca-bundle"},
			}
			certConfigs := []*CertificateConfig{
				{
					MetaConfig: MetaConfig{Name: "serving-certs"},
					Validity:   30 * 24 * time.Hour,
					Refresh:    15 * 24 * time.Hour,
					CertCreator: (&ocrypto.ServingCertCreatorConfig{
						Subject:  pkix.Name{},
						DNSNames: []string{"test.example.com"},
					}).ToCreator(),
				},
			}
			now := time.Now()
			nowFunc := func() time.Time { return now }

			manage := func() {
				t.Helper()
				err := cm.ManageCertificates(ctx, nowFunc, controller, controllerGVK, caConfig, caBundleConfig, certConfigs, map[string]*corev1.Secret{}, map[string]*corev1.ConfigMap{})
				if err != nil {
					t.Fatal(err)
				}
			}
			snapshot := func() map[string]metav1.Object {
				t.Helper()
				res := map[string]metav1.Object{}
				for _, name := range []string{"ca", "serving-certs"} {
					secret, err := objects.getSecret(name)
					if err != nil {
						t.Fatal(err)
					}
					if ref := metav1.GetControllerOf(secret); ref == nil || ref.UID != controller.UID {
						t.Errorf("expected Secret %q to be controlled by the controller, got %v", name, secret.OwnerReferences)
					}
					res["Secret/"+name] = secret
				}
				configMap, err := objects.getConfigMap("ca-bundle")
				if err != nil {
					t.Fatal(err)
				}
				certs, err := ocrypto.DecodeCertificates([]byte(configMap.Data[CABundleKey]))
				if err != nil {
					t.Fatal(err)
				}
				if len(certs) != 1 {
					t.Errorf("expected 1 certificate in the CA bundle, got %d", len(certs))
				}
				res["ConfigMap/ca-bundle"] = configMap
				return res
			}

			manage()
			first := snapshot()
			if writes := objects.writes(); writes != len(first) {
				t.Errorf("expected the first run to issue %d writes, got %d", len(first), writes)
			}

			objects.syncCaches()
			manage()
			second := snapshot()
			if writes := objects.writes(); writes != len(first) {
				t.Errorf("expected the second run to issue no writes, got %d in total", writes)
			}
			if diff := cmp.Diff(first, second); diff != "" {
				t.Errorf("expected the second run to leave the objects alone (-first +second):\n%s", diff)
			}
		})
	}
}
