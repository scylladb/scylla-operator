package scyllaoperatorconfig

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/client-go/tools/record"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "ScyllaOperatorConfigController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scyllaoperatorconfig"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	maxSyncDuration           = 30 * time.Second
	clusterDomainPollInterval = 5 * time.Minute
)

type GetClusterDomainFunc func(context.Context) (string, error)

// Controller keeps the singleton ScyllaOperatorConfig present and its status current, re-resolving the cluster
// domain periodically. It is a single-key controller for the singleton.
type Controller struct {
	client client.Client

	getClusterDomainFunc GetClusterDomainFunc

	eventRecorder record.EventRecorder

	// trigger enqueues the singleton outside its watch: once at start, so the object is created when missing, and
	// periodically, to re-resolve the cluster domain.
	trigger *controllertools.Trigger
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	eventRecorder record.EventRecorder,
	getClusterDomain GetClusterDomainFunc,
) *Controller {
	return &Controller{
		client: c,

		getClusterDomainFunc: getClusterDomain,

		eventRecorder: eventRecorder,

		trigger: controllertools.NewTrigger(),
	}
}

// ControllerOptions returns the controller options the controller runs with: a bounded sync duration.
func ControllerOptions() controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: 1,
		ReconciliationTimeout:   maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager: every event of the singleton ScyllaOperatorConfig
// re-runs the sync, and so does the trigger, once at start and every clusterDomainPollInterval.
func (opc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	if options.ReconciliationTimeout == 0 {
		options.ReconciliationTimeout = maxSyncDuration
	}

	err := ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		Watches(
			&scyllav1alpha1.ScyllaOperatorConfig{},
			controllertools.EnqueueSingleton(controllerRuntimeName),
			ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetName() == naming.SingletonName
			})),
		).
		WatchesRawSource(opc.trigger.Source(controllerRuntimeName)).
		WithOptions(options).
		Complete(opc)
	if err != nil {
		return fmt.Errorf("can't build controller: %w", err)
	}

	err = mgr.Add(controllertools.PeriodicTrigger(opc.trigger, clusterDomainPollInterval))
	if err != nil {
		return fmt.Errorf("can't add periodic trigger: %w", err)
	}

	// Sync right away: the singleton is created by the sync when it is missing, so there may be no event to wait for.
	opc.trigger.Enqueue()

	return nil
}

// Make sure we always have an aggregate to process and all nested errors are flattened.

// The quiet errors still retry with the rate limiter, like they did in the client-go controller.
