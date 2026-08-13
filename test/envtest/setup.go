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
	operatorcmd "github.com/scylladb/scylla-operator/pkg/cmd/operator"
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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var repoRoot = filepath.Join("..", "..", "..")

type Environment struct {
	kubeClient       *kubernetes.Clientset
	scyllaClient     *scyllaversionedclient.Clientset
	namespace        string
	config           *rest.Config
	client           client.Client
	monitoringClient *monitoringclient.Clientset
}

// SetupOptions configures the envtest environment.
type SetupOptions struct {
	InstallMonitoringCRDs  bool
	InstallMutatingWebhook bool
}

func defaultSetupOptions() SetupOptions {
	return SetupOptions{
		InstallMonitoringCRDs:  true,
		InstallMutatingWebhook: true,
	}
}

// SetupOption is a functional option for configuring the envtest environment.
type SetupOption func(*SetupOptions)

// WithoutMonitoringCRDs configures the envtest environment to not install
// Prometheus Operator CRDs (monitoring.coreos.com).
func WithoutMonitoringCRDs() SetupOption {
	return func(o *SetupOptions) {
		o.InstallMonitoringCRDs = false
	}
}

// WithoutMutatingWebhook configures the envtest environment to not install the mutating admission webhook shipped
// in deploy/operator/. Use it only for specs that need to observe objects as they were submitted, or that install
// the webhook themselves, e.g. to exercise the behavior from before it was installed.
func WithoutMutatingWebhook() SetupOption {
	return func(o *SetupOptions) {
		o.InstallMutatingWebhook = false
	}
}

// Setup sets up an envtest environment with the ScyllaDB CRDs installed. It will be cleaned up automatically when the test ends.
// It returns an Environment struct that provides access to the Kubernetes and ScyllaDB clients, as well as the test namespace
// for convenience.
// The mutating and validating admission webhooks shipped in deploy/operator/ are installed and served, so that objects
// created by any spec go through the same defaulting and validation as in a real deployment. The mutating one can be
// opted out of with WithoutMutatingWebhook.
func Setup(ctx context.Context, opts ...SetupOption) *Environment {
	g.GinkgoHelper()

	options := defaultSetupOptions()
	for _, opt := range opts {
		opt(&options)
	}

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

	var monitoringClient *monitoringclient.Clientset
	if options.InstallMonitoringCRDs {
		monitoringClient, err = monitoringclient.NewForConfig(testEnv.Config)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	baseScyllaCRDDir := filepath.Join(repoRoot, "pkg", "api", "scylla")
	crdPaths := []string{
		filepath.Join(baseScyllaCRDDir, "v1alpha1"),
		filepath.Join(baseScyllaCRDDir, "v1"),
	}
	if options.InstallMonitoringCRDs {
		prometheusOperatorCRDsPath := filepath.Join(repoRoot, "examples", "third-party", "prometheus-operator.yaml")
		crdPaths = append(crdPaths, prometheusOperatorCRDsPath)
	}
	installCRDOptions := envtest.CRDInstallOptions{
		Paths:              crdPaths,
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

	env := &Environment{
		kubeClient:       kubeClient,
		scyllaClient:     scyllaClient,
		monitoringClient: monitoringClient,
		client:           cl,
		namespace:        ns.Name,
		config:           testEnv.Config,
	}

	if options.InstallMutatingWebhook {
		SetupOperatorMutatingWebhook(ctx, env, operatorcmd.NewMutatingWebhookHandler(operatorcmd.DefaultDefaulters))
	}

	setupOperatorValidatingWebhook(ctx, env, operatorcmd.NewValidatingWebhookHandleFunc(operatorcmd.DefaultValidators))

	return env
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
// The shipped one is installed by Setup and decides on admission as well, on its own server, so an object has to
// satisfy the Operator's validation on top of what handleFunc admits.
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

	setupMockWebhook(ctx, e, webhookOpts, webhookPath, admissionreview.NewHandler(handleFunc))
}

// SetupOperatorMutatingWebhook installs the MutatingWebhookConfiguration shipped in deploy/operator/ and dispatches
// the intercepted admission requests to handler, so the shipped rules and path are exercised.
// The webhook server is started automatically and cleaned up when the test ends.
// NOTE: envtest rewrites the manifest's client config to point at a test-local webhook server, not the webhook
// server shipped with the Operator.
func SetupOperatorMutatingWebhook(ctx context.Context, e *Environment, handler admission.Handler) {
	g.GinkgoHelper()

	webhookOpts := envtest.WebhookInstallOptions{
		Paths: []string{filepath.Join(repoRoot, "deploy", "operator", "10_mutatingwebhook.yaml")},
	}

	setupMockWebhook(ctx, e, webhookOpts, "/mutate", &admission.Webhook{
		Handler: handler,
	})
}

// setupOperatorValidatingWebhook installs the ValidatingWebhookConfiguration shipped in deploy/operator/ and dispatches
// the intercepted admission requests to handleFunc, so the shipped rules and path are exercised.
// The webhook server is started automatically and cleaned up when the test ends.
// NOTE: envtest rewrites the manifest's client config to point at a test-local webhook server, not the webhook
// server shipped with the Operator.
func setupOperatorValidatingWebhook(ctx context.Context, e *Environment, handleFunc admissionreview.HandleFunc) {
	g.GinkgoHelper()

	webhookOpts := envtest.WebhookInstallOptions{
		Paths: []string{filepath.Join(repoRoot, "deploy", "operator", "10_validatingwebhook.yaml")},
	}

	setupMockWebhook(ctx, e, webhookOpts, "/validate", admissionreview.NewHandler(handleFunc))
}

func setupMockWebhook(ctx context.Context, e *Environment, webhookOpts envtest.WebhookInstallOptions, webhookPath string, handler http.Handler) {
	g.GinkgoHelper()

	// Allocate the serving certificates and the address the webhook definitions are rewritten to point at, without
	// registering them in the API server yet.
	err := webhookOpts.PrepWithoutInstalling()
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to prepare webhook install options")

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
	mux.Handle(webhookPath, handler)

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

	// The webhook configurations are installed with failurePolicy: Fail, so anything their rules match is rejected
	// while they are registered but not served yet. Wait for the server, then register them, so that no operation
	// can hit a registered but dead webhook.
	o.Eventually(func(eo o.Gomega) {
		conn, err := tls.Dial("tcp", listenAddr, &tls.Config{
			InsecureSkipVerify: true,
		})
		eo.Expect(err).NotTo(o.HaveOccurred())

		eo.Expect(conn.Close()).NotTo(o.HaveOccurred())
	}).WithTimeout(30*time.Second).WithPolling(100*time.Millisecond).Should(o.Succeed(), "Webhook server didn't start serving")

	// Install doesn't repeat the preparation above, as it's already been done.
	err = webhookOpts.Install(e.Config())
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to install webhook configuration")
}
