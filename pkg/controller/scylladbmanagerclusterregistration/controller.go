// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "ScyllaDBManagerClusterRegistrationController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scylladbmanagerclusterregistration"

	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather requeue.
	// Unfortunately, Scylla Manager calls are synchronous, internally retried and can take ages.
	// Contrary to what it should be, this needs to be quite high.
	// FIXME: https://github.com/scylladb/scylla-operator/issues/2686
	maxSyncDuration = 2 * time.Minute
)

// Controller registers ScyllaDB clusters with the ScyllaDB Manager instance their ScyllaDBManagerClusterRegistration
// points at. It reads through a controller-runtime client with read-your-writes consistency.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache.
	apiReader client.Reader

	eventRecorder record.EventRecorder
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
) *Controller {
	return &Controller{
		client:    c,
		apiReader: apiReader,

		eventRecorder: eventRecorder,
	}
}

// ControllerOptions returns the controller options the controller runs with, including the bounded sync duration.
func ControllerOptions(maxConcurrentReconciles int) controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		ReconciliationTimeout:   maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager. The event handlers resolve the registrations through
// the manager's cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (smcrc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBManagerClusterRegistration{}).
		Watches(&scyllav1alpha1.ScyllaDBDatacenter{}, handler.EnqueueRequestsFromMapFunc(mapScyllaDBDatacenterToRegistration(cache))).
		Watches(&scyllav1alpha1.ScyllaDBCluster{}, handler.EnqueueRequestsFromMapFunc(mapScyllaDBClusterToRegistration(cache))).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapSecretToRegistrationThroughOwner(cache))).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(mapGlobalScyllaDBManagerNamespaceToRegistrations(cache))).
		WithOptions(options).
		Complete(smcrc)
}

func (smcrc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	rq := &controllertools.Requeue{}
	err := smcrc.sync(ctx, req.NamespacedName, rq)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		return rq.Result(), nil

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", req.NamespacedName, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", req.NamespacedName, "Error", err)

	case controllertools.IsNonRetriable(err):
		klog.InfoS("Hit non-retriable error. Dropping the item from the queue.", "Key", req.NamespacedName, "Error", err)
		return reconcile.Result{}, reconcile.TerminalError(err)
	}

	return reconcile.Result{}, fmt.Errorf("syncing key '%v' failed: %w", req.NamespacedName, err)
}

func requestFor(namespace, name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// requestForRegistrationOfScyllaDBDatacenter returns the request for the registration of sdc, if it exists in r.
func requestForRegistrationOfScyllaDBDatacenter(ctx context.Context, r client.Reader, sdc *scyllav1alpha1.ScyllaDBDatacenter) []reconcile.Request {
	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return nil
	}

	smcr, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBManagerClusterRegistration](ctx, r, sdc.Namespace, smcrName)
	if err != nil {
		return nil
	}

	klog.V(4).InfoS("Enqueuing ScyllaDBManagerClusterRegistration for ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KObj(sdc), "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))
	return []reconcile.Request{requestFor(smcr.Namespace, smcr.Name)}
}

// requestForRegistrationOfScyllaDBCluster returns the request for the registration of sc, if it exists in r.
func requestForRegistrationOfScyllaDBCluster(ctx context.Context, r client.Reader, sc *scyllav1alpha1.ScyllaDBCluster) []reconcile.Request {
	smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBCluster(sc)
	if err != nil {
		apimachineryutilruntime.HandleError(err)
		return nil
	}

	smcr, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBManagerClusterRegistration](ctx, r, sc.Namespace, smcrName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			apimachineryutilruntime.HandleError(err)
		}
		return nil
	}

	klog.V(4).InfoS("Enqueuing ScyllaDBManagerClusterRegistration for ScyllaDBCluster", "ScyllaDBCluster", klog.KObj(sc), "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))
	return []reconcile.Request{requestFor(smcr.Namespace, smcr.Name)}
}

func mapScyllaDBDatacenterToRegistration(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sdc, ok := obj.(*scyllav1alpha1.ScyllaDBDatacenter)
		if !ok {
			return nil
		}

		return requestForRegistrationOfScyllaDBDatacenter(ctx, cache, sdc)
	}
}

func mapScyllaDBClusterToRegistration(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sc, ok := obj.(*scyllav1alpha1.ScyllaDBCluster)
		if !ok {
			return nil
		}

		return requestForRegistrationOfScyllaDBCluster(ctx, cache, sc)
	}
}

// mapSecretToRegistrationThroughOwner enqueues the registration of the ScyllaDBDatacenter or ScyllaDBCluster that
// controls the Secret.
func mapSecretToRegistrationThroughOwner(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		controllerRef := metav1.GetControllerOf(obj)
		if controllerRef == nil {
			return nil
		}

		switch controllerRef.Kind {
		case scyllav1alpha1.ScyllaDBDatacenterGVK.Kind:
			sdc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBDatacenter](ctx, cache, obj.GetNamespace(), controllerRef.Name)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				return nil
			}

			return requestForRegistrationOfScyllaDBDatacenter(ctx, cache, sdc)

		case scyllav1alpha1.ScyllaDBClusterGVK.Kind:
			sc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBCluster](ctx, cache, obj.GetNamespace(), controllerRef.Name)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				return nil
			}

			return requestForRegistrationOfScyllaDBCluster(ctx, cache, sc)

		default:
			// Nothing to do.
			return nil
		}
	}
}

// mapGlobalScyllaDBManagerNamespaceToRegistrations enqueues every registration of the global ScyllaDB Manager
// instance when its Namespace changes.
func mapGlobalScyllaDBManagerNamespaceToRegistrations(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetName() != naming.ScyllaManagerNamespace {
			return nil
		}

		smcrs, err := ctrlclient.List[scyllav1alpha1.ScyllaDBManagerClusterRegistration](ctx, cache, corev1.NamespaceAll, naming.GlobalScyllaDBManagerClusterRegistrationSelector())
		if err != nil {
			apimachineryutilruntime.HandleError(err)
			return nil
		}

		klog.V(4).InfoS("Enqueuing ScyllaDBManagerClusterRegistrations for global ScyllaDB Manager Namespace")
		requests := make([]reconcile.Request, 0, len(smcrs))
		for _, smcr := range smcrs {
			requests = append(requests, requestFor(smcr.Namespace, smcr.Name))
		}

		return requests
	}
}
