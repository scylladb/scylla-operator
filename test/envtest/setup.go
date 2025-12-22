package envtest

import (
	"path/filepath"
	"testing"
	"time"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type Environment struct {
	kubeClient   *kubernetes.Clientset
	scyllaClient *scyllaversionedclient.Clientset
	namespace    string
	config       *rest.Config
	client       client.Client
}

// Setup sets up an envtest environment with the ScyllaDB CRDs installed. It will be cleaned up automatically when the test ends.
// It returns an Environment struct that provides access to the Kubernetes and ScyllaDB clients, as well as the test namespace
// for convenience.
func Setup(t *testing.T) *Environment {
	t.Helper()

	testEnv := &envtest.Environment{
		ControlPlaneStartTimeout:    time.Minute,
		ControlPlaneStopTimeout:     time.Minute,
		DownloadBinaryAssets:        true,
		DownloadBinaryAssetsVersion: "v1.35.0", // TODO: move to assets
	}

	_, err := testEnv.Start()
	if err != nil {
		t.Fatalf("Failed to start test environment: %v", err)
	}
	t.Cleanup(func() {
		t.Logf("Stopping test environment")
		if err := testEnv.Stop(); err != nil {
			t.Fatalf("Failed to stop test environment: %v", err)
		}
	})

	kubeClient, err := kubernetes.NewForConfig(testEnv.Config)
	if err != nil {
		t.Fatalf("Failed to create kubeClient: %v", err)
	}
	scyllaClient, err := scyllaversionedclient.NewForConfig(testEnv.Config)
	if err != nil {
		t.Fatalf("Failed to create scyllaClient: %v", err)
	}
	cl, err := client.New(testEnv.Config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatalf("Failed to create controller-runtime client: %v", err)
	}

	baseCRDDir := filepath.Join("..", "..", "..", "pkg", "api", "scylla")
	installCRDOptions := envtest.CRDInstallOptions{
		Paths: []string{
			filepath.Join(baseCRDDir, "v1alpha1"),
			filepath.Join(baseCRDDir, "v1"),
		},
		ErrorIfPathMissing: true,
	}
	crds, err := envtest.InstallCRDs(testEnv.Config, installCRDOptions)
	if err != nil {
		t.Fatalf("Failed to install CRDs: %v", err)
	}
	if err := envtest.WaitForCRDs(testEnv.Config, crds, installCRDOptions); err != nil {
		t.Fatalf("Failed to wait for CRDs: %v", err)
	}

	ns, err := kubeClient.CoreV1().Namespaces().Create(t.Context(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test namespace: %v", err)
	}

	return &Environment{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,
		client:       cl,
		namespace:    ns.Name,
		config:       testEnv.Config,
	}
}

func (e *Environment) TypedKubeClient() *kubernetes.Clientset {
	return e.kubeClient
}

func (e *Environment) KubeClient() client.Client {
	return e.client
}

func (e *Environment) ScyllaClient() *scyllaversionedclient.Clientset {
	return e.scyllaClient
}

func (e *Environment) Namespace() string {
	return e.namespace
}

func (e *Environment) Config() *rest.Config {
	return e.config
}
