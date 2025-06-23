// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/systemd"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

const (
	ControllerName = "NodeSetupController"
)

var (
	keyFunc                 = cache.DeletionHandlingMetaNamespaceKeyFunc
	nodeConfigControllerGVK = scyllav1alpha1.GroupVersion.WithKind("NodeConfig")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface

	nodeConfigLister scyllav1alpha1listers.NodeConfigLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.RateLimitingInterface
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.NodeConfig]

	nodeName       string
	nodeUID        types.UID
	nodeConfigName string
	nodeConfigUID  types.UID

	executor           exec.Interface
	systemdControl     *systemd.SystemdControl
	systemdUnitManager *systemd.UnitManager
	sysfsPath          string
	devtmpfsPath       string
}

func NewController(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	scyllaClient scyllav1alpha1client.ScyllaV1alpha1Interface,
	nodeConfigInformer scyllav1alpha1informers.NodeConfigInformer,
	nodeName string,
	nodeUID types.UID,
	nodeConfigName string,
	nodeConfigUID types.UID,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	systemdControl, err := systemd.NewSystemdSystemControl(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create systemd control: %w", err)
	}

	ncc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		nodeConfigLister: nodeConfigInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			nodeConfigInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "nodesetup-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeSetup"),

		nodeName:       nodeName,
		nodeUID:        nodeUID,
		nodeConfigName: nodeConfigName,
		nodeConfigUID:  nodeConfigUID,

		executor:           exec.New(),
		systemdControl:     systemdControl,
		systemdUnitManager: systemd.NewUnitManager("scylla-operator-node-setup"),
		sysfsPath:          "/sys",
		devtmpfsPath:       "/dev",
	}

	ncc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.NodeConfig](
		ncc.queue,
		keyFunc,
		scheme.Scheme,
		nodeConfigControllerGVK,
		kubeinterfaces.GlobalGetList[*scyllav1alpha1.NodeConfig]{
			GetFunc: func(name string) (*scyllav1alpha1.NodeConfig, error) {
				return ncc.nodeConfigLister.Get(name)
			},
			ListFunc: func(selector labels.Selector) (ret []*scyllav1alpha1.NodeConfig, err error) {
				return ncc.nodeConfigLister.List(selector)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	nodeConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ncc.addNodeConfig,
		UpdateFunc: ncc.updateNodeConfig,
		DeleteFunc: ncc.deleteNodeConfig,
	})

	return ncc, nil
}

func (nsc *Controller) Close() {
	nsc.systemdControl.Close()
}

func (nsc *Controller) addNodeConfig(obj interface{}) {
	nsc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.NodeConfig),
		nsc.handlers.Enqueue,
	)
}

func (nsc *Controller) updateNodeConfig(old, cur interface{}) {
	nsc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.NodeConfig),
		cur.(*scyllav1alpha1.NodeConfig),
		nsc.handlers.Enqueue,
		nsc.deleteNodeConfig,
	)
}

func (nsc *Controller) deleteNodeConfig(obj interface{}) {
	nsc.handlers.HandleDelete(
		obj,
		nsc.handlers.Enqueue,
	)
}

func (nsc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := nsc.queue.Get()
	if quit {
		return false
	}
	defer nsc.queue.Done(key)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err := nsc.sync(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		nsc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		apimachineryutilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	nsc.queue.AddRateLimited(key)

	return true
}

func (nsc *Controller) runWorker(ctx context.Context) {
	for nsc.processNextItem(ctx) {
	}
}

func (nsc *Controller) Run(ctx context.Context) {
	defer apimachineryutilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		nsc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), nsc.cachesToSync...) {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		apimachineryutilwait.UntilWithContext(ctx, nsc.runWorker, time.Second)
	}()

	<-ctx.Done()
}
