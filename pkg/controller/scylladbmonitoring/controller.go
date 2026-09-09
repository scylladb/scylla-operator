package scylladbmonitoring

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ControllerName = "ScyllaDBMonitoringController"

	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scylladbmonitoring"
)

var (
	scylladbMonitoringControllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDBMonitoring")
)

// Controller reconciles ScyllaDBMonitorings. It reads through a controller-runtime client with read-your-writes
// consistency, so a sync always decides from a cache that has observed the writes of the previous syncs.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: adoption.
	apiReader client.Reader

	eventRecorder record.EventRecorder

	keyGetter crypto.KeyGenerator
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
	keyGetter crypto.KeyGenerator,
) *Controller {
	return &Controller{
		client:    c,
		apiReader: apiReader,

		eventRecorder: eventRecorder,

		keyGetter: keyGetter,
	}
}

// SetupWithManager registers the controller with the manager. The event handlers resolve the referrers through the
// manager's cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (smc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBMonitoring{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&appsv1.Deployment{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&monitoringv1.Prometheus{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		// ConfigMaps and Secrets are either owned or referenced by a ScyllaDBMonitoring's Grafana datasources.
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapToOwnerOrReferrers(cache, getScyllaDBMonitoringGrafanaConfigMapReferences))).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapToOwnerOrReferrers(cache, getScyllaDBMonitoringGrafanaSecretReferences))).
		Watches(&scyllav1alpha1.ScyllaOperatorConfig{}, handler.EnqueueRequestsFromMapFunc(mapToAllScyllaDBMonitorings(cache))).
		WithOptions(options).
		Complete(smc)
}

func (smc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	rq := &controllertools.Requeue{}
	err := smc.sync(ctx, req.NamespacedName, rq)
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

// mapToOwnerOrReferrers enqueues the ScyllaDBMonitoring owning the object, and every ScyllaDBMonitoring in its
// namespace whose references, as returned by getReferences, name it. It replaces the informer indexes of the
// client-go controller with a list through the cache.
func mapToOwnerOrReferrers(cache client.Reader, getReferences func(*scyllav1alpha1.ScyllaDBMonitoring) []string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		var requests []reconcile.Request

		controllerRef := metav1.GetControllerOf(obj)
		if controllerRef != nil && controllerRef.Kind == scylladbMonitoringControllerGVK.Kind {
			requests = append(requests, requestFor(obj.GetNamespace(), controllerRef.Name))
		}

		sms, err := ctrlclient.List[scyllav1alpha1.ScyllaDBMonitoring](ctx, cache, obj.GetNamespace(), labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list ScyllaDBMonitorings in namespace %q: %w", obj.GetNamespace(), err))
			return requests
		}

		for _, sm := range sms {
			for _, name := range getReferences(sm) {
				if name == obj.GetName() {
					klog.V(4).InfoS("Enqueuing ScyllaDBMonitoring for referenced object", "Object", klog.KObj(obj), "ScyllaDBMonitoring", klog.KObj(sm))
					requests = append(requests, requestFor(sm.Namespace, sm.Name))
					break
				}
			}
		}

		return requests
	}
}

// mapToAllScyllaDBMonitorings enqueues every ScyllaDBMonitoring, for the cluster-scoped objects all of them depend on.
func mapToAllScyllaDBMonitorings(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sms, err := ctrlclient.List[scyllav1alpha1.ScyllaDBMonitoring](ctx, cache, obj.GetNamespace(), labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list ScyllaDBMonitorings: %w", err))
			return nil
		}

		klog.V(4).InfoS("Enqueuing all ScyllaDBMonitorings", "Object", klog.KObj(obj), "Count", len(sms))
		requests := make([]reconcile.Request, 0, len(sms))
		for _, sm := range sms {
			requests = append(requests, requestFor(sm.Namespace, sm.Name))
		}

		return requests
	}
}
