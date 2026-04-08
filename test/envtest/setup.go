//go:build envtest

package envtest

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	"github.com/scylladb/scylla-operator/pkg/admissionreview"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Environment struct {
	kubeClient       *kubernetes.Clientset
	scyllaClient     *scyllaversionedclient.Clientset
	namespace        string
	config           *rest.Config
	client           client.Client
	monitoringClient *monitoringclient.Clientset
}

// Setup sets up an envtest environment with the ScyllaDB CRDs installed. It will be cleaned up automatically when the test ends.
// It returns an Environment struct that provides access to the Kubernetes and ScyllaDB clients, as well as the test namespace
// for convenience.
func Setup(ctx context.Context) *Environment {
	g.GinkgoHelper()

	log.SetLogger(g.GinkgoLogr)

	testEnv := &envtest.Environment{
		ControlPlaneStartTimeout:    time.Minute,
		ControlPlaneStopTimeout:     time.Minute,
		DownloadBinaryAssets:        true,
		DownloadBinaryAssetsVersion: configassets.Project.OperatorTests.EnvTestKubernetesVersion,
		BinaryAssetsDirectory:       os.TempDir(),
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

	monitoringClient, err := monitoringclient.NewForConfig(testEnv.Config)
	o.Expect(err).NotTo(o.HaveOccurred())

	baseScyllaCRDDir := filepath.Join("..", "..", "..", "pkg", "api", "scylla")
	prometheusOperatorCRDsPath := filepath.Join("..", "..", "..", "examples", "third-party", "prometheus-operator.yaml")
	installCRDOptions := envtest.CRDInstallOptions{
		Paths: []string{
			filepath.Join(baseScyllaCRDDir, "v1alpha1"),
			filepath.Join(baseScyllaCRDDir, "v1"),
			prometheusOperatorCRDsPath,
		},
		ErrorIfPathMissing: true,
		// The default MaxTime of 10 seconds is not enough for the API server to process
		// all CRDs and register them in discovery under CI load.
		MaxTime: time.Minute,
	}
	_, err = envtest.InstallCRDs(testEnv.Config, installCRDOptions)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to install CRDs")

	ns, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create test namespace")

	return &Environment{
		kubeClient:       kubeClient,
		scyllaClient:     scyllaClient,
		monitoringClient: monitoringClient,
		client:           cl,
		namespace:        ns.Name,
		config:           testEnv.Config,
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

func (e *Environment) MonitoringClient() *monitoringclient.Clientset {
	return e.monitoringClient
}

func (e *Environment) Namespace() string {
	return e.namespace
}

func (e *Environment) Config() *rest.Config {
	return e.config
}

// SetupMockValidatingWebhook installs a ValidatingWebhookConfiguration that intercepts all operations on all resources
// under the scylla.scylladb.com API group (v1 and v1alpha1) and dispatches them to handleFunc.
// The webhook server is started automatically and cleaned up when the test ends.
// NOTE: this function starts a test-only mock webhook server, not the validating webhook shipped with the Operator.
func SetupMockValidatingWebhook(ctx context.Context, e *Environment, handleFunc admissionreview.HandleFunc) {
	g.GinkgoHelper()

	webhookPath := "/validate"
	failurePolicy := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone
	scope := admissionregistrationv1.AllScopes

	webhookOpts := envtest.WebhookInstallOptions{
		ValidatingWebhooks: []*admissionregistrationv1.ValidatingWebhookConfiguration{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "envtest-scylladb",
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name:                    "envtest.scylladb.scylla.scylladb.com",
						AdmissionReviewVersions: []string{"v1"},
						SideEffects:             &sideEffects,
						FailurePolicy:           &failurePolicy,
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Operations: []admissionregistrationv1.OperationType{
									admissionregistrationv1.OperationAll,
								},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{"scylla.scylladb.com"},
									APIVersions: []string{"v1", "v1alpha1"},
									Resources:   []string{"*"},
									Scope:       &scope,
								},
							},
						},
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								// Namespace and Name are replaced by a direct URL by envtest;
								// only Path is used.
								Namespace: "default",
								Name:      "unused",
								Path:      ptr.To(webhookPath),
							},
						},
					},
				},
			},
		},
	}

	err := webhookOpts.Install(e.Config())
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to install validating webhook")

	g.DeferCleanup(func() {
		err := webhookOpts.Cleanup()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to clean up webhook install options")
	})

	cert, err := tls.LoadX509KeyPair(
		filepath.Join(webhookOpts.LocalServingCertDir, "tls.crt"),
		filepath.Join(webhookOpts.LocalServingCertDir, "tls.key"),
	)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to load webhook TLS keypair")

	mux := http.NewServeMux()
	mux.Handle(webhookPath, admissionreview.NewHandler(handleFunc))

	listenAddr := fmt.Sprintf("%s:%d", webhookOpts.LocalServingHost, webhookOpts.LocalServingPort)
	listener, err := net.Listen("tcp", listenAddr)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create webhook listener")

	server := &http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	go func() {
		if err := server.ServeTLS(listener, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			g.GinkgoWriter.Printf("Webhook server error: %v\n", err)
		}
	}()

	g.DeferCleanup(func(ctx g.SpecContext) {
		err := server.Shutdown(ctx)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to shut down webhook server")
	})
}
