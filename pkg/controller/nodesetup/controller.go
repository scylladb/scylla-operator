// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/systemd"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ControllerName = "NodeSetupController"
)

const (
	// controllerRuntimeName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	controllerRuntimeName = "nodesetup"
)

// Controller sets the node up (RAID arrays, filesystems, mounts, loop devices) as its NodeConfig asks, and reports
// the node's status into it. It is a single-key controller for its own NodeConfig.
type Controller struct {
	client client.Client

	eventRecorder record.EventRecorder

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

var _ reconcile.Reconciler = &Controller{}

func NewController(
	ctx context.Context,
	c client.Client,
	eventRecorder record.EventRecorder,
	nodeName string,
	nodeUID types.UID,
	nodeConfigName string,
	nodeConfigUID types.UID,
) (*Controller, error) {
	systemdControl, err := systemd.NewSystemdSystemControl(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create systemd control: %w", err)
	}

	return &Controller{
		client: c,

		eventRecorder: eventRecorder,

		nodeName:       nodeName,
		nodeUID:        nodeUID,
		nodeConfigName: nodeConfigName,
		nodeConfigUID:  nodeConfigUID,

		executor:           exec.New(),
		systemdControl:     systemdControl,
		systemdUnitManager: systemd.NewUnitManager("scylla-operator-node-setup"),
		sysfsPath:          "/sys",
		devtmpfsPath:       "/dev",
	}, nil
}

// SetupWithManager registers the controller with the manager. Every event of its NodeConfig re-runs the sync.
func (nsc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		Watches(
			&scyllav1alpha1.NodeConfig{},
			controllertools.EnqueueSingleton(controllerRuntimeName),
			ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetName() == nsc.nodeConfigName
			})),
		).
		WithOptions(options).
		Complete(nsc)
}

func (nsc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	err := nsc.sync(ctx)
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = apimachineryutilerrors.Reduce(err)
	switch {
	case err == nil:
		return reconcile.Result{}, nil

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", req.NamespacedName, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", req.NamespacedName, "Error", err)
	}

	return reconcile.Result{}, fmt.Errorf("syncing key '%v' failed: %w", req.NamespacedName, err)
}

func (nsc *Controller) Close() {
	nsc.systemdControl.Close()
}
