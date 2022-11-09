// Copyright (c) 2022 ScyllaDB.

package resourceapply

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// ApplyScyllaDatacenter will apply the ScyllaDatacenter to match the required object.
func ApplyScyllaDatacenter(
	ctx context.Context,
	client scyllav1alpha1client.ScyllaDatacentersGetter,
	lister scyllav1alpha1listers.ScyllaDatacenterLister,
	recorder record.EventRecorder,
	required *scyllav1alpha1.ScyllaDatacenter,
) (*scyllav1alpha1.ScyllaDatacenter, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("scylladatacenter %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.ScyllaDatacenters(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.ScyllaDatacenters(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "ScyllaDatacenter", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		// This is not the place to handle adoption.
		err := fmt.Errorf("scylladatacenter %q isn't controlled by us", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	// If they are the same do nothing.
	if existing.Annotations[naming.ManagedHash] == requiredCopy.Annotations[naming.ManagedHash] {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	requiredCopy.ResourceVersion = existing.ResourceVersion
	actual, err := client.ScyllaDatacenters(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "ScyllaDatacenter", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update scylladatacenter: %w", err)
	}
	return actual, true, nil
}
