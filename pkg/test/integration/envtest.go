// Copyright (C) 2017 ScyllaDB

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	utilyaml "github.com/scylladb/scylla-operator/pkg/util/yaml"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	env *envtest.Environment
)

// Get the root of the current file to use in CRD paths.
func rootPath() string {
	_, filename, _, _ := goruntime.Caller(0)
	return path.Join(path.Dir(filename), "..", "..", "..")
}

func init() {
	// Register the scheme.
	utilruntime.Must(scyllav1.AddToScheme(scheme.Scheme))

	root := rootPath()
	env = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join(root, "config", "operator", "crd", "bases"),
		},
	}
}

// TestEnvironment encapsulates a Kubernetes local test environment.
type TestEnvironment struct {
	manager.Manager
	Client
	KubeClient kubernetes.Interface
	Config     *rest.Config

	logger log.Logger
	cancel context.CancelFunc
}

type option struct {
	pollRetryInterval time.Duration
	pollTimeout       time.Duration
}

type EnvOption func(*option)

func WithPollRetryInterval(interval time.Duration) func(*option) {
	return func(o *option) {
		o.pollRetryInterval = interval
	}
}

func WithPollTimeout(timeout time.Duration) func(*option) {
	return func(o *option) {
		o.pollTimeout = timeout
	}
}

// NewTestEnvironment creates a new environment spinning up a local api-server.
func NewTestEnvironment(logger log.Logger, options ...EnvOption) (*TestEnvironment, error) {

	envOpts := &option{
		pollRetryInterval: 200 * time.Millisecond,
		pollTimeout:       60 * time.Second,
	}

	for _, opt := range options {
		opt(envOpts)
	}

	// initialize webhook here to be able to test the envtest install via webhookOptions
	// This should set LocalServingCertDir and LocalServingPort that are used below.
	if err := initializeWebhookInEnvironment(); err != nil {
		return nil, err
	}

	if _, err := env.Start(); err != nil {
		return nil, err
	}

	mgrOptions := manager.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
		CertDir:            env.WebhookInstallOptions.LocalServingCertDir,
		Port:               env.WebhookInstallOptions.LocalServingPort,
	}

	mgr, err := ctrl.NewManager(env.Config, mgrOptions)
	if err != nil {
		return nil, errors.Wrap(err, "start testenv manager")
	}

	if err := (&scyllav1.ScyllaCluster{}).SetupWebhookWithManager(mgr); err != nil {
		return nil, errors.Wrap(err, "create webhook")
	}

	return &TestEnvironment{
		Manager: mgr,
		Client: Client{
			Client:        mgr.GetClient(),
			RetryInterval: envOpts.pollRetryInterval,
			Timeout:       envOpts.pollTimeout,
		},
		KubeClient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		Config:     mgr.GetConfig(),
		logger:     logger,
	}, nil
}

const (
	mutatingWebhookKind   = "MutatingWebhookConfiguration"
	validatingWebhookKind = "ValidatingWebhookConfiguration"
)

func appendWebhookConfiguration(mutatingWebhooks []runtime.Object, validatingWebhooks []runtime.Object, configyamlFile io.Reader, tag string) ([]runtime.Object, []runtime.Object, error) {
	objs, err := utilyaml.ToUnstructured(configyamlFile)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse yaml")
	}

	for i := range objs {
		o := objs[i]
		if o.GetKind() == mutatingWebhookKind {
			mutatingWebhooks = append(mutatingWebhooks, &o)
		}
		if o.GetKind() == validatingWebhookKind {
			validatingWebhooks = append(validatingWebhooks, &o)
		}
	}
	return mutatingWebhooks, validatingWebhooks, err
}

func initializeWebhookInEnvironment() error {
	var validatingWebhooks, mutatingWebhooks []runtime.Object

	root := rootPath()
	configyamlFile, err := ioutil.ReadFile(filepath.Join(root, "config", "operator", "webhook", "manifests.yaml"))
	if err != nil {
		return errors.Wrap(err, "read core webhook configuration file")
	}

	mutatingWebhooks, validatingWebhooks, err = appendWebhookConfiguration(mutatingWebhooks, validatingWebhooks, bytes.NewReader(configyamlFile), "config")
	if err != nil {
		return errors.Wrap(err, "controller webhook config")
	}

	env.WebhookInstallOptions = envtest.WebhookInstallOptions{
		MaxTime:            20 * time.Second,
		PollInterval:       time.Second,
		ValidatingWebhooks: validatingWebhooks,
		MutatingWebhooks:   mutatingWebhooks,
	}

	return nil
}

func (t *TestEnvironment) StartManager(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	return t.Manager.Start(ctx.Done())
}

func (t *TestEnvironment) WaitForWebhooks() {
	port := env.WebhookInstallOptions.LocalServingPort

	timeout := 1 * time.Second
	for {
		time.Sleep(1 * time.Second)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), timeout)
		if err != nil {
			continue
		}
		_ = conn.Close()
		return
	}
}

func (t *TestEnvironment) Stop() error {
	t.cancel()
	return env.Stop()
}

func (t *TestEnvironment) CreateNamespace(ctx context.Context, generateName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
			Labels: map[string]string{
				"testenv/original-name": generateName,
			},
		},
	}
	if err := t.Client.Create(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}
