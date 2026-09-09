package sidecar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "SidecarController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scyllasidecar"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	maxSyncDuration          = 30 * time.Second
	scyllaAPIPollingInterval = 30 * time.Second
)

type hostID struct {
	v string
	sync.RWMutex
}

// Controller keeps the member Service of the ScyllaDB node the sidecar runs next to in sync with the node: it projects
// the node's identity from the ScyllaDB API into the Service's annotations and carries out the decommission the
// Service asks for. It is a single-key controller for its own Service.
type Controller struct {
	namespace        string
	serviceName      string
	localhostAddress string

	client client.Client

	newScyllaClient func() (*scyllaclient.Client, error)

	// trigger enqueues the Service outside its watch: periodically, to re-project the ScyllaDB API values.
	trigger *controllertools.Trigger

	hostID hostID
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	namespace,
	serviceName string,
	localhostAddress string,
	c client.Client,
	newScyllaClient func() (*scyllaclient.Client, error),
) (*Controller, error) {
	// Sanity check.
	if len(namespace) == 0 {
		return nil, fmt.Errorf("service namespace can't be empty")
	}
	if len(serviceName) == 0 {
		return nil, fmt.Errorf("service name can't be empty")
	}

	if len(localhostAddress) == 0 {
		return nil, fmt.Errorf("localhost address can't be empty")
	}

	return &Controller{
		namespace:        namespace,
		serviceName:      serviceName,
		localhostAddress: localhostAddress,

		client: c,

		newScyllaClient: newScyllaClient,

		trigger: controllertools.NewTrigger(),
	}, nil
}

// CacheOptions restricts the manager's cache to the member's Service and Pod, which share the name, in namespace.
func CacheOptions(namespace, serviceName string) cache.Options {
	identity := fields.OneTermEqualSelector("metadata.name", serviceName)

	return cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Service{}: {
				Field: identity,
			},
			&corev1.Pod{}: {
				Field: identity,
			},
		},
	}
}

// ControllerOptions returns the controller options the sidecar runs with: a single worker with a tight retry bound,
// and a bounded sync duration.
func ControllerOptions() controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: 1,
		RateLimiter: workqueue.NewTypedMaxOfRateLimiter[reconcile.Request](
			workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
				5*time.Millisecond,
				// This is a single key controller just for its node, the upper bound should be fairly low.
				10*time.Second,
			),
			&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
		// Enforces preemption. Do not raise the value! Controllers shouldn't actively wait, but rather requeue.
		ReconciliationTimeout: maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager, and the periodic sync that keeps the values projected
// from the ScyllaDB API up to date.
func (c *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	err := ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		Watches(&corev1.Service{}, controllertools.EnqueueSingleton(controllerRuntimeName)).
		WatchesRawSource(c.trigger.Source(controllerRuntimeName)).
		WithOptions(options).
		Complete(c)
	if err != nil {
		return fmt.Errorf("can't build controller: %w", err)
	}

	// Periodically reconcile Member Service to make sure values projected from Scylla API are up-to-date.
	err = mgr.Add(controllertools.PeriodicTrigger(c.trigger, scyllaAPIPollingInterval))
	if err != nil {
		return fmt.Errorf("can't add periodic trigger: %w", err)
	}

	return nil
}

// Enqueue requests a sync outside the Service's watch.
func (c *Controller) Enqueue() {
	c.trigger.Enqueue()
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	rq := &controllertools.Requeue{}
	err := c.sync(ctx, rq)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		return rq.Result(), nil

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", req.NamespacedName, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", req.NamespacedName, "Error", err)
	}

	return reconcile.Result{}, fmt.Errorf("syncing key '%v' failed: %w", req.NamespacedName, err)
}

func (c *Controller) getHostID(ctx context.Context, scyllaClient *scyllaclient.Client, localhostAddr string) (string, error) {
	var v string
	c.hostID.RLock()
	v = c.hostID.v
	c.hostID.RUnlock()

	if len(v) > 0 {
		return v, nil
	}

	c.hostID.Lock()
	defer c.hostID.Unlock()

	v = c.hostID.v
	if len(v) > 0 {
		return v, nil
	}

	v, err := scyllaClient.GetLocalHostId(ctx, localhostAddr, false)
	if err != nil {
		return "", fmt.Errorf("can't get local HostID: %w", err)
	}

	if len(v) == 0 {
		return "", fmt.Errorf("can't get local HostID: HostID can't be empty")
	}

	c.hostID.v = v

	return v, nil
}
