// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	monitoringversionedclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// defaultReconciliationTimeout bounds every Reconcile of a controller registered with the manager. It is a guardrail,
// not a budget: the longest sync the operator has is a ScyllaDBDatacenter one chaining its three 10s StatefulSet cache
// propagation sleeps (create, update, member replace), and a minute leaves that twice the room.
const defaultReconciliationTimeout = 1 * time.Minute

// setControllerRuntimeLogger guards ctrllog.SetLogger, which is neither idempotent nor safe to call concurrently.
var setControllerRuntimeLogger sync.Once

type Options struct {
	RestConfig *rest.Config
	Logger     logr.Logger

	KubeClient       kubernetes.Interface
	ScyllaClient     scyllaversionedclient.Interface
	MonitoringClient monitoringversionedclient.Interface

	ClusterDomainGetter scyllaoperatorconfig.GetClusterDomainFunc
	KeyGenerator        crypto.KeyGenerator

	OperatorImage                  string
	CQLSIngressPort                int
	ConcurrentSyncs                int
	ResyncPeriod                   time.Duration
	OperatorNamespace              string
	GlobalScyllaDBManagerNamespace string
}

// Manager owns the controller-runtime manager, and with it the single cache every controller of the operator
// reads from, and the controllers registered with it.
type Manager struct {
	options Options
	mgr     ctrlmanager.Manager
	client  ctrlclient.ReadYourWritesClient

	// starters are started right before the manager and stopped with it. The remote cluster informer factories
	// live here until the multi-DC controllers are migrated.
	starters []func(stopCh <-chan struct{})
	// runnables are the legacy controllers' Run functions.
	runnables []func(ctx context.Context)
}

// New creates the controller-runtime manager. Controllers are registered in Run because they take the context
// their informers' list and watch calls run under.
func New(options Options) (*Manager, error) {
	// controller-runtime's packages (the cache above all) log through the package-level logger, not the manager's;
	// route it to the same place, once per process, so those logs aren't dropped.
	setControllerRuntimeLogger.Do(func() {
		ctrllog.SetLogger(options.Logger)
	})

	mgr, err := ctrlmanager.New(options.RestConfig, ctrlmanager.Options{
		Scheme: scheme.Scheme,
		Logger: options.Logger,
		Cache: cache.Options{
			SyncPeriod: new(options.ResyncPeriod),
			ByObject: map[client.Object]cache.ByObject{
				// Every controller reads the ScyllaOperatorConfig singleton and nothing else of the kind.
				&scyllav1alpha1.ScyllaOperatorConfig{}: {
					Field: fields.OneTermEqualSelector("metadata.name", naming.SingletonName),
				},
			},
		},
		// The operator's cache watches every kind it manages cluster-wide, so a read after a write only waits for
		// the informer to catch up.
		Client: client.Options{
			Cache: &client.CacheOptions{
				EnableReadYourWritesConsistency: new(true),
			},
		},
		Controller: config.Controller{
			ReconciliationTimeout: defaultReconciliationTimeout,
		},
		// Metrics stay off until OPERATOR-414 (operator observability) settles how they are served:
		// https://scylladb.atlassian.net/browse/OPERATOR-414. Health probes were never served.
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
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
		// The client is read-your-writes by the cache option set above.
		client: ctrlclient.NewReadYourWritesClient(mgr.GetClient()),
	}, nil
}

// Client returns the manager's client, reading from the shared cache with read-your-writes consistency.
func (m *Manager) Client() ctrlclient.ReadYourWritesClient {
	return m.client
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
