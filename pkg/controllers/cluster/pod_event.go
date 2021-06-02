package cluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ handler.EventHandler = &EnqueueRequestForPod{}

// EnqueueRequestForPod enqueues Requests for the Owners of an object.  E.g. the object that created
// the object that was the source of the Event.
//
// If a ReplicaSet creates Pods, users may reconcile the ReplicaSet in response to Pod Events using:
//
// - a source.Kind Source with Type of Pod.
//
// - a handler.EnqueueRequestForPod EventHandler with an OwnerType of ReplicaSet and IsController set to true.
type EnqueueRequestForPod struct {
	// groupStsKind is the cached Group and Kind for sts OwnerType
	groupStsKind schema.GroupKind

	// groupClusterKind is the cached Group and Kind for ScyllaCluster OwnerType
	groupClusterKind schema.GroupKind

	// mapper maps GroupVersionKinds to Resources
	mapper meta.RESTMapper

	KubeClient kubernetes.Interface
}

// Create implements EventHandler
func (e *EnqueueRequestForPod) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.Object.(metav1.Object)) {
		q.Add(req)
	}
}

// Update implements EventHandler
func (e *EnqueueRequestForPod) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.ObjectOld.(metav1.Object)) {
		q.Add(req)
	}
	for _, req := range e.getOwnerReconcileRequest(evt.ObjectNew.(metav1.Object)) {
		q.Add(req)
	}
}

// Delete implements EventHandler
func (e *EnqueueRequestForPod) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.Object.(metav1.Object)) {
		q.Add(req)
	}
}

// Generic implements EventHandler
func (e *EnqueueRequestForPod) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.Object.(metav1.Object)) {
		q.Add(req)
	}
}

// generateGroupKind generate Group and Kind and caches the result for sts and ScyllaCluster type.  Returns false
// if sts or ScyllaCluster type could not be parsed using the scheme.
func (e *EnqueueRequestForPod) generateGroupKind(scheme *runtime.Scheme) error {
	// Get the kinds of the type
	stsType := &appsv1.StatefulSet{}
	clusterType := &scyllav1.ScyllaCluster{}

	kindsSts, _, err := scheme.ObjectKinds(stsType)
	if err != nil {
		return err
	}
	kindsCluster, _, err := scheme.ObjectKinds(clusterType)
	if err != nil {
		return err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kindsSts) != 1 {
		err := fmt.Errorf("Expected exactly 1 kind for OwnerType %T, but found %s kinds", stsType, kindsSts)
		return err

	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kindsCluster) != 1 {
		err := fmt.Errorf("Expected exactly 1 kind for OwnerType %T, but found %s kinds", clusterType, kindsCluster)
		return err

	}
	// Cache the Group and Kind for the OwnerType
	e.groupStsKind = schema.GroupKind{Group: kindsSts[0].Group, Kind: kindsSts[0].Kind}
	e.groupClusterKind = schema.GroupKind{Group: kindsCluster[0].Group, Kind: kindsCluster[0].Kind}
	return nil
}

// getOwnerReconcileRequest looks at object and returns a slice of reconcile.Request to reconcile
// owners of object that match the ScyllaCluster type through the sts type
func (e *EnqueueRequestForPod) getOwnerReconcileRequest(object metav1.Object) []reconcile.Request {
	// Iterate through the OwnerReferences looking for a match on Group and Kind against what was requested
	// by the user
	var result []reconcile.Request
	for _, ref := range e.getOwnersReferences(object) {
		// Parse the Group out of the OwnerReference to compare it
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil
		}

		// Compare the OwnerReference Group and Kind against the sts Group and Kind.
		// If the two match, get the reconcile request from the ScyllaCluster type owner
		if ref.Kind == e.groupStsKind.Kind && refGV.Group == e.groupStsKind.Group {
			// Get sts and then get ScyllaCluster reference
			sts, err := e.KubeClient.AppsV1().StatefulSets(object.GetNamespace()).Get(context.Background(), ref.Name, metav1.GetOptions{})
			if err != nil {
				return nil
			}
			resultSts := e.getStsOwnerReconcileRequest(sts.GetObjectMeta())

			result = append(result, resultSts...)
		}
	}

	// Return the matches
	return result
}

// getStsOwnerReconcileRequest looks at object and returns a slice of reconcile.Request to reconcile
// owners of object that match ScyllaCluster type
func (e *EnqueueRequestForPod) getStsOwnerReconcileRequest(object metav1.Object) []reconcile.Request {
	// Iterate through the OwnerReferences looking for a match on Group and Kind against what was requested
	// by the user
	var result []reconcile.Request
	for _, ref := range e.getOwnersReferences(object) {
		// Parse the Group out of the OwnerReference to compare it
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil
		}

		// Compare the OwnerReference Group and Kind against the ScyllaCluster Group and Kind.
		// If the two match, create a Request for the objected referred to by
		// the OwnerReference.  Use the Name from the OwnerReference and the Namespace from the
		// object in the event.
		if ref.Kind == e.groupClusterKind.Kind && refGV.Group == e.groupClusterKind.Group {
			// Match found - add a Request for the object referred to in the OwnerReference
			request := reconcile.Request{NamespacedName: types.NamespacedName{
				Name: ref.Name,
			}}

			request.Namespace = object.GetNamespace()

			result = append(result, request)
		}
	}

	// Return the matches
	return result
}

// getOwnersReferences returns the OwnerReferences for an object as specified by the EnqueueRequestForOwner
// - if IsController is true: only take the Controller OwnerReference (if found)
// - if IsController is false: take all OwnerReferences
func (e *EnqueueRequestForPod) getOwnersReferences(object metav1.Object) []metav1.OwnerReference {
	if object == nil {
		return nil
	}

	// If filtered to a Controller, only take the Controller OwnerReference
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		return []metav1.OwnerReference{*ownerRef}
	}
	// No Controller OwnerReference found
	return nil
}

var _ inject.Scheme = &EnqueueRequestForPod{}

// InjectScheme is called by the Controller to provide a singleton scheme to the EnqueueRequestForOwner.
func (e *EnqueueRequestForPod) InjectScheme(s *runtime.Scheme) error {
	return e.generateGroupKind(s)
}

var _ inject.Mapper = &EnqueueRequestForPod{}

// InjectMapper  is called by the Controller to provide the rest mapper used by the manager.
func (e *EnqueueRequestForPod) InjectMapper(m meta.RESTMapper) error {
	e.mapper = m
	return nil
}
