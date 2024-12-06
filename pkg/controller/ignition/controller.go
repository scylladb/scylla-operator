package ignition

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Controller struct {
	*controllertools.Observer

	namespace                   string
	serviceName                 string
	nodesBroadcastAddressType   scyllav1alpha1.BroadcastAddressType
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType

	ignited atomic.Bool

	configMapLister corev1listers.ConfigMapLister
	serviceLister   corev1listers.ServiceLister
	podLister       corev1listers.PodLister
}

func NewController(
	namespace string,
	serviceName string,
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType,
	nodesBroadcastAddressType scyllav1alpha1.BroadcastAddressType,
	kubeClient kubernetes.Interface,
	configMapInformer corev1informers.ConfigMapInformer,
	serviceInformer corev1informers.ServiceInformer,
	podInformer corev1informers.PodInformer,
) (*Controller, error) {
	controller := &Controller{
		namespace:                   namespace,
		serviceName:                 serviceName,
		clientsBroadcastAddressType: clientsBroadcastAddressType,
		nodesBroadcastAddressType:   nodesBroadcastAddressType,
		ignited:                     atomic.Bool{},
		configMapLister:             configMapInformer.Lister(),
		serviceLister:               serviceInformer.Lister(),
		podLister:                   podInformer.Lister(),
	}

	observer := controllertools.NewObserver(
		"scylladb-ignition",
		kubeClient.CoreV1().Events(corev1.NamespaceAll),
		controller.Sync,
	)

	configMapHandler, err := configMapInformer.Informer().AddEventHandler(observer.GetGenericHandlers())
	if err != nil {
		return nil, fmt.Errorf("can't add ConfigMap event handler: %w", err)
	}
	observer.AddCachesToSync(configMapHandler.HasSynced)

	serviceHandler, err := serviceInformer.Informer().AddEventHandler(observer.GetGenericHandlers())
	if err != nil {
		return nil, fmt.Errorf("can't add Service event handler: %w", err)
	}
	observer.AddCachesToSync(serviceHandler.HasSynced)

	podHandler, err := podInformer.Informer().AddEventHandler(observer.GetGenericHandlers())
	if err != nil {
		return nil, fmt.Errorf("can't add Pod event handler: %w", err)
	}
	observer.AddCachesToSync(podHandler.HasSynced)

	controller.Observer = observer

	return controller, nil
}

func (c *Controller) IsIgnited() bool {
	return c.ignited.Load()
}

func (c *Controller) Sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", c.Observer.Name(), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", c.Observer.Name(), "duration", time.Since(startTime))
	}()

	ignited := true

	svc, err := c.serviceLister.Services(c.namespace).Get(c.serviceName)
	if err != nil {
		return fmt.Errorf("can't get service %q: %w", c.serviceName, err)
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
			ignited = false
		} else {
			klog.V(2).InfoS(
				"Service is available and has an IP address",
				"Service", naming.ManualRef(svc.Namespace, svc.Name),
				"UID", svc.UID,
			)
		}
	}

	pod, err := c.podLister.Pods(c.namespace).Get(c.serviceName)
	if err != nil {
		return fmt.Errorf("can't get pod %q: %w", c.serviceName, err)
	}

	if len(pod.Status.PodIP) == 0 {
		klog.V(2).InfoS("PodIP is not yet set", "Pod", klog.KObj(pod), "UID", pod.UID)
		ignited = false
	} else {
		klog.V(2).InfoS("PodIP is present on the Pod", "Pod", klog.KObj(pod), "UID", pod.UID, "IP", pod.Status.PodIP)
	}

	containerID, err := controllerhelpers.GetScyllaContainerID(pod)
	if err != nil {
		return controllertools.NonRetriable(
			fmt.Errorf("can't get scylla container id in pod %q: %v", naming.ObjRef(pod), err),
		)
	}

	if len(containerID) == 0 {
		klog.V(2).InfoS("ScyllaDB ContainerID is not yet set", "Pod", klog.KObj(pod), "UID", pod.UID)
		ignited = false
	} else {
		klog.V(2).InfoS("Pod has ScyllaDB ContainerID set", "Pod", klog.KObj(pod), "UID", pod.UID, "ContainerID", containerID)
	}

	cmLabelSelector := labels.Set{
		naming.OwnerUIDLabel:      string(pod.UID),
		naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
	}.AsSelector()
	configMaps, err := c.configMapLister.ConfigMaps(c.namespace).List(cmLabelSelector)
	if err != nil {
		return fmt.Errorf("can't list tuning configmap: %w", err)
	}

	switch l := len(configMaps); l {
	case 0:
		klog.V(2).InfoS("Tuning ConfigMap for pod is not yet available", "Pod", klog.KObj(pod), "UID", pod.UID)
		ignited = false

	case 1:
		cm := configMaps[0]
		src := &internalapi.SidecarRuntimeConfig{}
		src, err = controllerhelpers.GetSidecarRuntimeConfigFromConfigMap(cm)
		if err != nil {
			return controllertools.NonRetriable(
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
				ignited = false
			}
		} else {
			klog.V(2).InfoS("Scylla runtime config is not yet updated with our ContainerID",
				"ConfigMap", klog.KObj(cm),
				"ConfigContainerID", src.ContainerID,
				"SidecarContainerID", containerID,
			)
			ignited = false
		}

	default:
		return fmt.Errorf("mutiple tuning configmaps are present for pod %q with UID %q", naming.ObjRef(pod), pod.UID)
	}

	if ignited {
		klog.V(2).InfoS("Ignition successful", "SignalFile", naming.ScyllaDBIgnitionDonePath)
		err = helpers.TouchFile(naming.ScyllaDBIgnitionDonePath)
		if err != nil {
			return fmt.Errorf("can't touch signal file %q: %w", naming.ScyllaDBIgnitionDonePath, err)
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

	return nil
}
