// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	remoteclient "github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// MetricsDisabledBindAddress is the metrics bind address that disables the metrics server.
const MetricsDisabledBindAddress = "0"

type Options struct {
	RestConfig *rest.Config
	Logger     logr.Logger

	KubeClient       kubernetes.Interface
	ScyllaClient     scyllaversionedclient.Interface
	MonitoringClient monitoringversionedclient.Interface
	// MonitoringCRDsInstalled tells whether the Prometheus Operator CRDs are available in the cluster.
	// The ScyllaDBMonitoring controller is only registered when they are.
	MonitoringCRDsInstalled bool

	ClusterKubeClient   *remoteclient.ClusterClient[kubernetes.Interface]
	ClusterScyllaClient *remoteclient.ClusterClient[scyllaversionedclient.Interface]

	ClusterDomainGetter scyllaoperatorconfig.GetClusterDomainFunc
	KeyGenerator        crypto.KeyGenerator

	OperatorImage   string
	CQLSIngressPort int
	ConcurrentSyncs int
	ResyncPeriod    time.Duration

	// MetricsBindAddress is the address the controller-runtime metrics server binds to.
	// MetricsDisabledBindAddress disables the server.
	MetricsBindAddress string
}

// Manager owns the controller-runtime manager, and with it the single cache every controller of the operator
// reads from, and the controllers registered with it.
type Manager struct {
	options Options
	mgr     ctrlmanager.Manager

	// starters are started right before the manager and stopped with it. The remote cluster informer factories
	// live here until the multi-DC controllers are migrated.
	starters []func(stopCh <-chan struct{})
	// runnables are the legacy controllers' Run functions.
	runnables []func(ctx context.Context)
}

// New creates the controller-runtime manager. Controllers are registered in Run because they take the context
// their informers' list and watch calls run under.
func New(options Options) (*Manager, error) {
	mgr, err := ctrlmanager.New(options.RestConfig, ctrlmanager.Options{
		Scheme: scheme.Scheme,
		Logger: options.Logger,
		Cache: cache.Options{
			SyncPeriod: ptr.To(options.ResyncPeriod),
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				EnableReadYourWritesConsistency: ptr.To(true),
			},
		},
		Metrics: metricsserver.Options{
			BindAddress: options.MetricsBindAddress,
		},
		// The operator has never served health probes, and swapping the binary must not change what a running
		// deployment can observe.
		HealthProbeBindAddress: "0",
		// Leader election stays with pkg/leaderelection around Run, so that the standby replicas don't start
		// the cache and the lease name and identity are unchanged.
		LeaderElection: false,
	})
	if err != nil {
		return nil, fmt.Errorf("can't create controller-runtime manager: %w", err)
	}

	return &Manager{
		options: options,
		mgr:     mgr,
	}, nil
}

// Run registers the controllers, starts the cache, the remote informer factories and the controllers,
// and blocks until ctx is done.
func (m *Manager) Run(ctx context.Context) error {
	err := m.registerControllers(ctx)
	if err != nil {
		return fmt.Errorf("can't register controllers: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, start := range m.starters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start(ctx.Done())
		}()
	}

	mgrErrCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		mgrErrCh <- m.mgr.Start(ctx)
	}()

	for _, run := range m.runnables {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run(ctx)
		}()
	}

	<-ctx.Done()

	err = <-mgrErrCh
	if err != nil {
		klog.ErrorS(err, "Controller-runtime manager failed")
		return fmt.Errorf("controller-runtime manager failed: %w", err)
	}

	return nil
}
