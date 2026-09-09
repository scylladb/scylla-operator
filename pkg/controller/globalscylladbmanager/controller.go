// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// ControllerName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	ControllerName = "globalscylladbmanager-controller"
)

// Controller is an observer: it keeps the ScyllaDBManagerClusterRegistrations of the global ScyllaDB Manager
// instance in line with the ScyllaDBDatacenters and ScyllaDBClusters that opted into it, re-deriving all of them
// from the cache on every relevant event.
type Controller struct {
	client        client.Client
	eventRecorder record.EventRecorder

	// trigger enqueues the sync outside the watches: once at start, the global ScyllaDB Manager may already be
	// deployed.
	trigger *controllertools.Trigger
}

func NewController(
	c client.Client,
	eventRecorder record.EventRecorder,
) *Controller {
	return &Controller{
		client:        c,
		eventRecorder: eventRecorder,

		trigger: controllertools.NewTrigger(),
	}
}

// SetupWithManager registers the controller with the manager. Every event of the watched kinds, filtered the way the
// former handlers did, re-runs the sync.
func (gsmc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	enqueue := controllertools.EnqueueSingleton(ControllerName)

	err := ctrlbuilder.ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(&scyllav1alpha1.ScyllaDBManagerClusterRegistration{}, enqueue, ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			smcr, ok := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
			return ok && controllerhelpers.IsManagedByGlobalScyllaDBManagerInstance(smcr)
		}))).
		Watches(&scyllav1alpha1.ScyllaDBDatacenter{}, enqueue).
		Watches(&scyllav1alpha1.ScyllaDBCluster{}, enqueue).
		Watches(&corev1.Namespace{}, enqueue, ctrlbuilder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetName() == naming.ScyllaManagerNamespace
		}))).
		WatchesRawSource(gsmc.trigger.Source(ControllerName)).
		WithOptions(options).
		Complete(gsmc)
	if err != nil {
		return fmt.Errorf("can't build controller: %w", err)
	}

	// Start immediately, global ScyllaDB Manager may already be deployed.
	gsmc.trigger.Enqueue()

	return nil
}
