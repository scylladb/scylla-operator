// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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
	ControllerName = "ScyllaClusterController"

	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scyllaclustermigration"
)

var (
	scyllaClusterControllerGVK = scyllav1.GroupVersion.WithKind("ScyllaCluster")
)

// Controller translates v1 ScyllaClusters into v1alpha1 ScyllaDBDatacenters and ScyllaDBManagerTasks, releases the
// objects the ScyllaCluster used to own to the ScyllaDBDatacenter, and reports the ScyllaDBDatacenter's status back.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: adoption,
	// release, and the existence check before migrating an upgrade in flight.
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

// SetupWithManager registers the controller with the manager. The event handlers resolve relations through the
// manager's cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (scmc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1.ScyllaCluster{}).
		// The Kubernetes objects are owned by the ScyllaCluster until they are released to the ScyllaDBDatacenter.
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&batchv1.Job{}).
		Owns(&scyllav1alpha1.ScyllaDBDatacenter{}).
		Owns(&scyllav1alpha1.ScyllaDBManagerTask{}).
		Watches(&scyllav1alpha1.ScyllaDBManagerClusterRegistration{}, handler.EnqueueRequestsFromMapFunc(mapScyllaDBManagerClusterRegistrationToScyllaClusters(cache))).
		WithOptions(options).
		Complete(scmc)
}

func (scmc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	rq := &controllertools.Requeue{}
	err := scmc.sync(ctx, req.NamespacedName, rq)
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

func requestFor(namespace, name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// mapScyllaDBManagerClusterRegistrationToScyllaClusters enqueues the ScyllaClusters in the registration's namespace
// whose ScyllaDBDatacenter the registration is named after. Registrations are owned by neither, so the relation is
// resolved by name.
func mapScyllaDBManagerClusterRegistrationToScyllaClusters(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		scs, err := ctrlclient.List[scyllav1.ScyllaCluster](ctx, cache, obj.GetNamespace(), labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list ScyllaClusters in namespace %q: %w", obj.GetNamespace(), err))
			return nil
		}

		var requests []reconcile.Request
		for _, sc := range scs {
			sdc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBDatacenter](ctx, cache, sc.Namespace, sc.Name)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				continue
			}

			smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBDatacenter(sdc)
			if err != nil {
				apimachineryutilruntime.HandleError(err)
				continue
			}

			if obj.GetName() == smcrName {
				requests = append(requests, requestFor(sc.Namespace, sc.Name))
			}
		}

		return requests
	}
}
