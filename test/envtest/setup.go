//go:build envtest

package envtest

import (
	"context"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
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
func Setup(ctx context.Context) *Environment {
	testEnv := &envtest.Environment{
		ControlPlaneStartTimeout:    time.Minute,
		ControlPlaneStopTimeout:     time.Minute,
		DownloadBinaryAssets:        true,
		DownloadBinaryAssetsVersion: configassets.Project.OperatorTests.EnvTestKubernetesVersion,
	}

	_, err := testEnv.Start()
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to start test environment")

	g.DeferCleanup(func() {
		g.GinkgoWriter.Println("Stopping test environment")
		err := testEnv.Stop()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to stop test environment")
	})

	kubeClient, err := kubernetes.NewForConfig(testEnv.Config)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create kubeClient")

	scyllaClient, err := scyllaversionedclient.NewForConfig(testEnv.Config)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create scyllaClient")

	cl, err := client.New(testEnv.Config, client.Options{Scheme: scheme.Scheme})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create controller-runtime client")

	baseCRDDir := filepath.Join("..", "..", "..", "pkg", "api", "scylla")
	installCRDOptions := envtest.CRDInstallOptions{
		Paths: []string{
			filepath.Join(baseCRDDir, "v1alpha1"),
			filepath.Join(baseCRDDir, "v1"),
		},
		ErrorIfPathMissing: true,
	}
	crds, err := envtest.InstallCRDs(testEnv.Config, installCRDOptions)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to install CRDs")

	err = envtest.WaitForCRDs(testEnv.Config, crds, installCRDOptions)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to wait for CRDs")

	ns, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create test namespace")

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
