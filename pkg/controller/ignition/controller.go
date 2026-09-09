package ignition

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ControllerName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	ControllerName = "scylladb-ignition"
)

// Controller is an observer: it re-derives from the member's Service, Pod and tuning ConfigMap whether the ScyllaDB
// node may start, and reports it through IsIgnited and the signal file.
type Controller struct {
	namespace                   string
	serviceName                 string
	nodesBroadcastAddressType   scyllav1alpha1.BroadcastAddressType
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType

	ignited atomic.Bool

	client client.Reader
}

func NewController(
	namespace string,
	serviceName string,
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType,
	nodesBroadcastAddressType scyllav1alpha1.BroadcastAddressType,
	c client.Reader,
) *Controller {
	return &Controller{
		namespace:                   namespace,
		serviceName:                 serviceName,
		clientsBroadcastAddressType: clientsBroadcastAddressType,
		nodesBroadcastAddressType:   nodesBroadcastAddressType,
		ignited:                     atomic.Bool{},
		client:                      c,
	}
}

// CacheOptions restricts the manager's cache to the member's Service and Pod, which share the name, and to the
// NodeConfig data ConfigMaps, all in namespace.
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
			&corev1.ConfigMap{}: {
				Label: labels.Set{
					naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
				}.AsSelector(),
			},
		},
	}
}

// SetupWithManager registers the controller with the manager. Every event of the watched kinds re-runs the sync.
func (c *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(&corev1.ConfigMap{}, controllertools.EnqueueSingleton(ControllerName)).
		Watches(&corev1.Service{}, controllertools.EnqueueSingleton(ControllerName)).
		Watches(&corev1.Pod{}, controllertools.EnqueueSingleton(ControllerName)).
		WithOptions(options).
		Complete(c)
}

func (c *Controller) IsIgnited() bool {
	return c.ignited.Load()
}

func (c *Controller) evaluateIgnitionState(ctx context.Context) (bool, error) {
	svc, err := ctrlclient.Get[corev1.Service](ctx, c.client, c.namespace, c.serviceName)
	if err != nil {
		return false, fmt.Errorf("can't get service %q: %w", c.serviceName, err)
	}

	// TODO: This isn't bound to the lifecycle of the ScyllaDB Pod and should be evaluated in the controller for the resource.
	//       https://github.com/scylladb/scylla-operator/issues/604
	if c.clientsBroadcastAddressType == scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress ||
		c.nodesBroadcastAddressType == scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress {
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			klog.V(2).InfoS(
				"Waiting for identity service to have at least one ingress point",
				"Service", naming.ManualRef(c.namespace, c.serviceName),
			)
			return false, nil
		}
		klog.V(2).InfoS(
			"Service is available and has an IP address",
			"Service", naming.ManualRef(svc.Namespace, svc.Name),
			"UID", svc.UID,
		)
	}

	pod, err := ctrlclient.Get[corev1.Pod](ctx, c.client, c.namespace, c.serviceName)
	if err != nil {
		return false, fmt.Errorf("can't get pod %q: %w", c.serviceName, err)
	}

	if len(pod.Status.PodIP) == 0 {
		klog.V(2).InfoS("PodIP is not yet set", "Pod", klog.KObj(pod), "UID", pod.UID)
		return false, nil
	}
	klog.V(2).InfoS("PodIP is present on the Pod", "Pod", klog.KObj(pod), "UID", pod.UID, "IP", pod.Status.PodIP)

	containerID, err := controllerhelpers.GetScyllaContainerID(pod)
	if err != nil {
		return false, controllertools.NonRetriable(
			fmt.Errorf("can't get scylla container id in pod %q: %v", naming.ObjRef(pod), err),
		)
	}

	if len(containerID) == 0 {
		klog.V(2).InfoS("ScyllaDB ContainerID is not yet set", "Pod", klog.KObj(pod), "UID", pod.UID)
		return false, nil
	}
	klog.V(2).InfoS("Pod has ScyllaDB ContainerID set", "Pod", klog.KObj(pod), "UID", pod.UID, "ContainerID", containerID)

	cmLabelSelector := labels.Set{
		naming.OwnerUIDLabel:      string(pod.UID),
		naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
	}.AsSelector()
	configMaps, err := ctrlclient.List[corev1.ConfigMap](ctx, c.client, c.namespace, cmLabelSelector)
	if err != nil {
		return false, fmt.Errorf("can't list tuning configmap: %w", err)
	}

	switch l := len(configMaps); l {
	case 0:
		klog.V(2).InfoS("Tuning ConfigMap for pod is not yet available", "Pod", klog.KObj(pod), "UID", pod.UID)
		return false, nil

	case 1:
		cm := configMaps[0]
		src := &internalapi.SidecarRuntimeConfig{}
		src, err = controllerhelpers.GetSidecarRuntimeConfigFromConfigMap(cm)
		if err != nil {
			return false, controllertools.NonRetriable(
				fmt.Errorf("can't get sidecar runtime config from configmap %q: %w", naming.ObjRef(cm), err),
			)
		}

		if containerID == src.ContainerID {
			if len(src.BlockingNodeConfigs) > 0 {
				klog.V(2).InfoS("Waiting on NodeConfig(s)",
					"ConfigMap", klog.KObj(cm),
					"ContainerID", containerID,
					"NodeConfig", src.BlockingNodeConfigs,
				)
				return false, nil
			}
		} else {
			klog.V(2).InfoS("Scylla runtime config is not yet updated with our ContainerID",
				"ConfigMap", klog.KObj(cm),
				"ConfigContainerID", src.ContainerID,
				"SidecarContainerID", containerID,
			)
			return false, nil
		}

	default:
		return false, fmt.Errorf("mutiple tuning configmaps are present for pod %q with UID %q", naming.ObjRef(pod), pod.UID)
	}

	return true, nil
}
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", ControllerName, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", ControllerName, "duration", time.Since(startTime))
	}()

	svc, err := ctrlclient.Get[corev1.Service](ctx, c.client, c.namespace, c.serviceName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't get service %q: %w", c.serviceName, err)
	}

	var ignitionOverride *bool
	if svc.Annotations != nil {
		forceIgnitionString, hasForceIgnitionString := svc.Annotations[naming.ForceIgnitionValueAnnotation]
		if hasForceIgnitionString {
			switch forceIgnitionString {
			case "true":
				ignitionOverride = pointer.Ptr(true)
			case "false":
				ignitionOverride = pointer.Ptr(false)
			default:
				klog.ErrorS(errors.New("invalid ignition override value"), "Value", forceIgnitionString, "Key", naming.ForceIgnitionValueAnnotation)
				ignitionOverride = nil
			}
		}
	}

	var ignited bool
	if ignitionOverride != nil {
		ignited = *ignitionOverride
		klog.InfoS("Forcing ignition state", "Ignited", ignited, "Annotation", naming.ForceIgnitionValueAnnotation)
	} else {
		ignited, err = c.evaluateIgnitionState(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("can't evaluate ignition state: %w", err)
		}
	}

	if ignited {
		klog.V(2).InfoS("Ignition successful", "SignalFile", naming.ScyllaDBIgnitionDonePath)
		err = helpers.TouchFile(naming.ScyllaDBIgnitionDonePath)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("can't touch signal file %q: %w", naming.ScyllaDBIgnitionDonePath, err)
		}
	} else {
		klog.V(2).InfoS("Waiting for ignition to complete.", "SignalFile", naming.ScyllaDBIgnitionDonePath)
	}

	klog.V(2).InfoS("Updating ignition", "Ignited", ignited)

	oldIgnited := c.ignited.Load()
	if ignited != oldIgnited {
		klog.InfoS("Ignition state has changed", "New", ignited, "Old", oldIgnited)
	}
	c.ignited.Store(ignited)

	return reconcile.Result{}, nil
}
