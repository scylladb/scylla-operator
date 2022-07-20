package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	appv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// ApplyStatefulSet will apply the StatefulSet to match the required object.
// forceOwnership allows to apply objects without an ownerReference. Normally such objects
// would be adopted but the old objects may not have correct labels that we need to fix in the new version.
func ApplyStatefulSet(
	ctx context.Context,
	client appv1client.StatefulSetsGetter,
	lister appv1listers.StatefulSetLister,
	recorder record.EventRecorder,
	required *appsv1.StatefulSet,
	forceOwnership bool,
) (*appsv1.StatefulSet, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("StatefulSet %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.StatefulSets(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.StatefulSets(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "StatefulSet", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil {
		if !forceOwnership {
			// This is not the place to handle adoption.
			err = fmt.Errorf("statefulset %q isn't controlled by anyone, won't adopt it in apply", naming.ObjRef(requiredCopy))
			ReportUpdateEvent(recorder, requiredCopy, err)
			return nil, false, err
		}
		klog.V(2).InfoS("Forcing apply to claim the StatefulSet", "StatefulSet", naming.ObjRef(requiredCopy))
	} else if existingControllerRef.UID != requiredControllerRef.UID {
		err = fmt.Errorf("statefulset %q is controlled by someone else", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	existingHash := existing.Annotations[naming.ManagedHash]
	requiredHash := requiredCopy.Annotations[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	// TODO: Handle immutable fields. Maybe orphan delete + recreate.

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.ResourceVersion) == 0 {
		requiredCopy.ResourceVersion = existing.ResourceVersion
	}
	actual, err := client.StatefulSets(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "StatefulSet", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update statefulset: %w", err)
	}
	return actual, true, nil
}

// ApplyDaemonSet will apply the DaemonSet to match the required object.
func ApplyDaemonSet(
	ctx context.Context,
	client appv1client.DaemonSetsGetter,
	lister appv1listers.DaemonSetLister,
	recorder record.EventRecorder,
	required *appsv1.DaemonSet,
) (*appsv1.DaemonSet, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("DaemonSet %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.DaemonSets(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.DaemonSets(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "DaemonSet", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		err = fmt.Errorf("daemonset %q isn't controlled by us", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	existingHash := existing.Annotations[naming.ManagedHash]
	requiredHash := requiredCopy.Annotations[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.ResourceVersion) == 0 {
		requiredCopy.ResourceVersion = existing.ResourceVersion
	}
	actual, err := client.DaemonSets(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "DaemonSet", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update daemonset: %w", err)
	}
	return actual, true, nil
}

// ApplyDeployment will apply the Deployment to match the required object.
func ApplyDeployment(
	ctx context.Context,
	client appv1client.DeploymentsGetter,
	lister appv1listers.DeploymentLister,
	recorder record.EventRecorder,
	required *appsv1.Deployment,
) (*appsv1.Deployment, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("deployment %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.Deployments(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.Deployments(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Deployment", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		err = fmt.Errorf("deployment %q isn't controlled by us", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	existingHash := existing.Annotations[naming.ManagedHash]
	requiredHash := requiredCopy.Annotations[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.ResourceVersion) == 0 {
		requiredCopy.ResourceVersion = existing.ResourceVersion
	}
	actual, err := client.Deployments(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Deployment", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update deployment: %w", err)
	}
	return actual, true, nil
}
