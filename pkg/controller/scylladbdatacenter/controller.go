package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	ControllerName = "ScyllaDBDatacenterController"

	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "scylladbdatacenter"
)

var (
	statefulSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
)

// Controller reconciles ScyllaDBDatacenters. It reads through a controller-runtime client with read-your-writes
// consistency, so a sync always decides from a cache that has observed the writes of the previous syncs.
type Controller struct {
	operatorImage   string
	cqlsIngressPort int

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
	operatorImage string,
	cqlsIngressPort int,
	keyGetter crypto.KeyGenerator,
) *Controller {
	return &Controller{
		operatorImage:   operatorImage,
		cqlsIngressPort: cqlsIngressPort,

		client:    c,
		apiReader: apiReader,

		eventRecorder: eventRecorder,

		keyGetter: keyGetter,
	}
}

// SetupWithManager registers the controller with the manager. The event handlers resolve owners through the manager's
// cache directly: waiting there for the controller's own writes would only delay the enqueue.
func (sdcc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBDatacenter{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&batchv1.Job{}).
		Owns(&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapSecretToOwnerOrAgentAuthTokenOverrideReferrers(cache))).
		// We need pods events to know if a pod:
		// - is ready after a replace operation
		// - has updated a status report
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapPodToOwnerThroughStatefulSet(cache))).
		Watches(&scyllav1alpha1.ScyllaOperatorConfig{}, handler.EnqueueRequestsFromMapFunc(mapToAllScyllaDBDatacenters(cache))).
		WithOptions(options).
		Complete(sdcc)
}

// podLister returns a PodLister reading through the client under ctx, for the functions that take one.
func (sdcc *Controller) podLister(ctx context.Context) corev1listers.PodLister {
	return ctrlclient.NewPodLister(ctx, sdcc.client)
}

// secretLister returns a SecretLister reading through the client under ctx, for the functions that take one.
func (sdcc *Controller) secretLister(ctx context.Context) corev1listers.SecretLister {
	return ctrlclient.NewSecretLister(ctx, sdcc.client)
}

func requestFor(namespace, name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// resolveScyllaDBDatacenterController returns the ScyllaDBDatacenter controlling obj, read through r, or nil.
func resolveScyllaDBDatacenterController(ctx context.Context, r client.Reader, obj metav1.Object) *scyllav1alpha1.ScyllaDBDatacenter {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != scyllav1alpha1.ScyllaDBDatacenterGVK.Kind {
		return nil
	}

	sdc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBDatacenter](ctx, r, obj.GetNamespace(), controllerRef.Name)
	if err != nil {
		return nil
	}

	if sdc.UID != controllerRef.UID {
		return nil
	}

	return sdc
}

// resolveStatefulSetController returns the StatefulSet controlling obj, read through r, or nil.
func resolveStatefulSetController(ctx context.Context, r client.Reader, obj metav1.Object) *appsv1.StatefulSet {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return nil
	}

	if controllerRef.Kind != statefulSetControllerGVK.Kind {
		return nil
	}

	sts, err := ctrlclient.Get[appsv1.StatefulSet](ctx, r, obj.GetNamespace(), controllerRef.Name)
	if err != nil {
		return nil
	}

	if sts.UID != controllerRef.UID {
		return nil
	}

	return sts
}

// resolveScyllaDBDatacenterControllerThroughStatefulSet returns the ScyllaDBDatacenter controlling the StatefulSet
// controlling obj, read through r, or nil.
func resolveScyllaDBDatacenterControllerThroughStatefulSet(ctx context.Context, r client.Reader, obj metav1.Object) *scyllav1alpha1.ScyllaDBDatacenter {
	sts := resolveStatefulSetController(ctx, r, obj)
	if sts == nil {
		return nil
	}

	return resolveScyllaDBDatacenterController(ctx, r, sts)
}

func mapPodToOwnerThroughStatefulSet(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sdc := resolveScyllaDBDatacenterControllerThroughStatefulSet(ctx, cache, obj)
		if sdc == nil {
			return nil
		}

		klog.V(4).InfoS("Enqueuing owner of the Pod's StatefulSet", "Pod", klog.KObj(obj), "ScyllaDBDatacenter", klog.KObj(sdc))
		return []reconcile.Request{requestFor(sdc.Namespace, sdc.Name)}
	}
}

// mapSecretToOwnerOrAgentAuthTokenOverrideReferrers enqueues the ScyllaDBDatacenter owning the Secret, and every
// ScyllaDBDatacenter in its namespace that refers to it as its ScyllaDB Manager agent auth token override.
func mapSecretToOwnerOrAgentAuthTokenOverrideReferrers(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		var requests []reconcile.Request

		controllerRef := metav1.GetControllerOf(obj)
		if controllerRef != nil && controllerRef.Kind == scyllav1alpha1.ScyllaDBDatacenterGVK.Kind {
			requests = append(requests, requestFor(obj.GetNamespace(), controllerRef.Name))
		}

		sdcs, err := ctrlclient.List[scyllav1alpha1.ScyllaDBDatacenter](ctx, cache, obj.GetNamespace(), labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list ScyllaDBDatacenters in namespace %q: %w", obj.GetNamespace(), err))
			return requests
		}

		for _, sdc := range sdcs {
			if sdc.Annotations[naming.ScyllaDBManagerAgentAuthTokenOverrideSecretRefAnnotation] == obj.GetName() {
				requests = append(requests, requestFor(sdc.Namespace, sdc.Name))
			}
		}

		return requests
	}
}

// mapToAllScyllaDBDatacenters enqueues every ScyllaDBDatacenter, for the cluster-scoped objects all of them depend on.
func mapToAllScyllaDBDatacenters(cache client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		sdcs, err := ctrlclient.List[scyllav1alpha1.ScyllaDBDatacenter](ctx, cache, obj.GetNamespace(), labels.Everything())
		if err != nil {
			apimachineryutilruntime.HandleError(fmt.Errorf("can't list ScyllaDBDatacenters: %w", err))
			return nil
		}

		klog.V(4).InfoS("Enqueuing all ScyllaDBDatacenters", "Object", klog.KObj(obj), "Count", len(sdcs))
		requests := make([]reconcile.Request, 0, len(sdcs))
		for _, sdc := range sdcs {
			requests = append(requests, requestFor(sdc.Namespace, sdc.Name))
		}

		return requests
	}
}
