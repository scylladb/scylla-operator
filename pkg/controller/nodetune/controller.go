// Copyright (C) 2021 ScyllaDB

package nodetune

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/kubelet"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "NodeConfigDaemonController"
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "nodeconfigdaemon"

	maxSyncDuration = 30 * time.Second
)

var (
	nodeConfigGVK          = scyllav1alpha1.GroupVersion.WithKind("NodeConfig")
	daemonSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
)

// Controller tunes the node for the ScyllaDB Pods scheduled on it through Jobs owned by its DaemonSet, and reports the
// node's status into the NodeConfig. It is a single-key controller for its node.
type Controller struct {
	// client reads from the manager's cache, waiting for it to observe this controller's writes, and writes to the
	// API server.
	client client.Client
	// apiReader reads live from the API server, for the decisions that must not be made from a cache: adoption.
	apiReader client.Reader

	criClient                 cri.Client
	kubeletPodResourcesClient kubelet.PodResourcesClient

	namespace      string
	podName        string
	nodeName       string
	nodeUID        types.UID
	nodeConfigName string
	nodeConfigUID  types.UID
	scyllaImage    string
	operatorImage  string

	eventRecorder record.EventRecorder

	// trigger enqueues the node outside the watches: once at start, Scylla might not be scheduled yet but the Node
	// can already be tuned.
	trigger *controllertools.Trigger
}

var _ reconcile.Reconciler = &Controller{}

func NewController(
	c client.Client,
	apiReader client.Reader,
	eventRecorder record.EventRecorder,
	criClient cri.Client,
	kubeletPodResourcesClient kubelet.PodResourcesClient,
	namespace string,
	podName string,
	nodeName string,
	nodeUID types.UID,
	nodeConfigName string,
	nodeConfigUID types.UID,
	scyllaImage string,
	operatorImage string,
) *Controller {
	return &Controller{
		client:    c,
		apiReader: apiReader,

		criClient:                 criClient,
		kubeletPodResourcesClient: kubeletPodResourcesClient,

		namespace:      namespace,
		podName:        podName,
		nodeName:       nodeName,
		nodeUID:        nodeUID,
		nodeConfigName: nodeConfigName,
		nodeConfigUID:  nodeConfigUID,
		scyllaImage:    scyllaImage,
		operatorImage:  operatorImage,

		eventRecorder: eventRecorder,

		trigger: controllertools.NewTrigger(),
	}
}

// CacheOptions restricts the manager's cache to what the node setup daemon watches: the DaemonSets, Jobs and
// ConfigMaps of its namespace, and, in every namespace, the Pods scheduled on its node. The daemon's own Pod runs on
// that node too, so one Pod informer serves both the ScyllaDB Pods to tune and the daemon's identity. NodeConfigs are
// cluster-scoped and served cluster-wide regardless of the namespace restriction.
func CacheOptions(namespace, nodeName string) cache.Options {
	return cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Namespaces: map[string]cache.Config{
					cache.AllNamespaces: {
						FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName),
					},
				},
			},
		},
	}
}

// ControllerOptions returns the controller options the node tune controller runs with: a single worker and a bounded
// sync duration.
func ControllerOptions() controller.Options {
	return controller.Options{
		MaxConcurrentReconciles: 1,
		ReconciliationTimeout:   maxSyncDuration,
	}
}

// SetupWithManager registers the controller with the manager: its NodeConfig, the ScyllaDB Pods on its node, and
// the Jobs and ConfigMaps controlled by its DaemonSet and NodeConfig re-run the sync, and so does the trigger.
func (ncdc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cacheReader := mgr.GetCache()
	enqueue := controllertools.EnqueueSingleton(controllerRuntimeName)

	err := ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		Watches(&scyllav1alpha1.NodeConfig{}, enqueue, ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(ncdc.isNodeConfigControlled))).
		// Deletions of Pods don't re-run the sync: the tuning follows the Pods that run on the node.
		Watches(&corev1.Pod{}, enqueue, ctrlbuilder.WithPredicates(predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return isScyllaDBPod(e.Object) },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isScyllaDBPod(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		})).
		Watches(&batchv1.Job{}, enqueue, ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return ncdc.ownsObject(cacheReader, obj)
		}))).
		Watches(&corev1.ConfigMap{}, enqueue, ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(ncdc.isControlledByNodeConfig))).
		WatchesRawSource(ncdc.trigger.Source(controllerRuntimeName)).
		WithOptions(options).
		Complete(ncdc)
	if err != nil {
		return fmt.Errorf("can't build controller: %w", err)
	}

	// Start right away, Scylla might not be scheduled yet, but Node can already be tuned.
	ncdc.trigger.Enqueue()

	return nil
}

func (ncdc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	err := ncdc.sync(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("syncing key '%v' failed: %w", req.NamespacedName, err)
	}

	return reconcile.Result{}, nil
}

func isScyllaDBPod(obj client.Object) bool {
	return naming.ScyllaSelector().Matches(labels.Set(obj.GetLabels()))
}

// ownsObject tells whether obj is controlled by the daemon's DaemonSet. It reads the daemon's Pod through r; the event
// handlers pass the cache, not to wait there for the controller's writes.
func (ncdc *Controller) ownsObject(r client.Reader, obj metav1.Object) bool {
	selfRef, err := ncdc.newOwningDSControllerRef(context.Background(), r)
	if err != nil {
		apimachineryutilruntime.HandleError(fmt.Errorf("can't get self controller ref: %w", err))
		return false
	}

	objControllerRef := metav1.GetControllerOfNoCopy(obj)
	klog.V(5).InfoS("checking object owner", "ObjectRef", objControllerRef, "SelfRef", selfRef)
	return apiequality.Semantic.DeepEqual(objControllerRef, selfRef)
}

// newOwningDSControllerRef returns the controller reference to the daemon's DaemonSet, from the daemon's Pod read
// through r.
func (ncdc *Controller) newOwningDSControllerRef(ctx context.Context, r client.Reader) (*metav1.OwnerReference, error) {
	pod, err := ctrlclient.Get[corev1.Pod](ctx, r, ncdc.namespace, ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get self Pod %q: %w", naming.ManualRef(ncdc.namespace, ncdc.podName), err)
	}

	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return nil, fmt.Errorf("pod %q doesn't have a controller refference", naming.ObjRef(pod))
	}

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("can't parse GroupVersion %q: %w", ref.APIVersion, err)
	}

	if gv.Group != daemonSetControllerGVK.Group {
		return nil, fmt.Errorf("pod's onwer ref group %q doesn't match the expected group %q", gv.Group, daemonSetControllerGVK.Group)

	}
	if ref.Kind != daemonSetControllerGVK.Kind {
		return nil, fmt.Errorf("pod's onwer ref kind %q doesn't match the expected kind %q", ref.Kind, daemonSetControllerGVK.Kind)
	}

	return ref, nil
}

func (ncdc *Controller) newNodeConfigObjectRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion:      nodeConfigGVK.Version,
		Kind:            nodeConfigGVK.Kind,
		Name:            ncdc.nodeConfigName,
		Namespace:       corev1.NamespaceAll,
		UID:             ncdc.nodeConfigUID,
		ResourceVersion: "",
	}
}

func (ncdc *Controller) isNodeConfigControlled(nc client.Object) bool {
	return nc.GetName() == ncdc.nodeConfigName && nc.GetUID() == ncdc.nodeConfigUID
}

func (ncdc *Controller) isControlledByNodeConfig(obj client.Object) bool {
	ref := metav1.GetControllerOfNoCopy(obj)
	if ref == nil {
		return false
	}
	return ref.UID == ncdc.nodeConfigUID
}
