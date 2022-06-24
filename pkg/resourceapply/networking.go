// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkingv1client "k8s.io/client-go/kubernetes/typed/networking/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// ApplyIngress will apply the Ingress to match the required object.
func ApplyIngress(
	ctx context.Context,
	client networkingv1client.IngressesGetter,
	lister networkingv1listers.IngressLister,
	recorder record.EventRecorder,
	required *networkingv1.Ingress,
) (*networkingv1.Ingress, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("ingress %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.Ingresses(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.Ingresses(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Ingress", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		// This is not the place to handle adoption.
		err := fmt.Errorf("ingress %q isn't controlled by us", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	// If they are the same do nothing.
	if existing.Annotations[naming.ManagedHash] == requiredCopy.Annotations[naming.ManagedHash] {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	requiredCopy.ResourceVersion = existing.ResourceVersion
	actual, err := client.Ingresses(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Ingress", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update ingress: %w", err)
	}
	return actual, true, nil
}
