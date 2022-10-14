package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// ApplyService will apply the Service to match the required object.
// forceOwnership allows to apply objects without an ownerReference. Normally such objects
// would be adopted but the old objects may not have correct labels that we need to fix in the new version.
func ApplyService(
	ctx context.Context,
	client corev1client.ServicesGetter,
	lister corev1listers.ServiceLister,
	recorder record.EventRecorder,
	required *corev1.Service,
	forceOwnership bool,
) (*corev1.Service, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("service %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.Services(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.Services(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Service", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, err == nil, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		if existingControllerRef == nil && forceOwnership {
			klog.V(2).InfoS("Forcing apply to claim the Service", "Service", naming.ObjRef(requiredCopy))
		} else {
			// This is not the place to handle adoption.
			err := fmt.Errorf("service %q isn't controlled by us", naming.ObjRef(requiredCopy))
			ReportUpdateEvent(recorder, requiredCopy, err)
			return nil, false, err
		}
	}

	// If they are the same do nothing.
	if existing.Annotations[naming.ManagedHash] == requiredCopy.Annotations[naming.ManagedHash] {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	// Preserve allocated fields.
	requiredCopy.Spec.ClusterIP = existing.Spec.ClusterIP
	requiredCopy.Spec.ClusterIPs = existing.Spec.ClusterIPs

	requiredCopy.ResourceVersion = existing.ResourceVersion
	actual, err := client.Services(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Service", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update service: %w", err)
	}
	return actual, true, nil
}

// ApplySecret will apply the Secret to match the required object.
// forceOwnership allows to apply objects without an ownerReference. Normally such objects
// would be adopted but the old objects may not have correct labels that we need to fix in the new version.
func ApplySecret(
	ctx context.Context,
	client corev1client.SecretsGetter,
	lister corev1listers.SecretLister,
	recorder record.EventRecorder,
	required *corev1.Secret,
	forceOwnership bool,
) (*corev1.Secret, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("secret %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.Secrets(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.Secrets(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Secret", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		if existingControllerRef == nil && forceOwnership {
			klog.V(2).InfoS("Forcing apply to claim the Secret", "Secret", naming.ObjRef(requiredCopy))
		} else {
			// This is not the place to handle adoption.
			err := fmt.Errorf("secret %q isn't controlled by us", naming.ObjRef(requiredCopy))
			ReportUpdateEvent(recorder, requiredCopy, err)
			return nil, false, err
		}
	}

	// If they are the same do nothing.
	if existing.Annotations[naming.ManagedHash] == requiredCopy.Annotations[naming.ManagedHash] {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(&requiredCopy.ObjectMeta, existing.ObjectMeta)

	requiredCopy.ResourceVersion = existing.ResourceVersion
	actual, err := client.Secrets(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Secret", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update secret: %w", err)
	}
	return actual, true, nil
}

// ApplyConfigMap will apply the ConfigMap to match the required object.
func ApplyConfigMap(
	ctx context.Context,
	client corev1client.ConfigMapsGetter,
	lister corev1listers.ConfigMapLister,
	recorder record.EventRecorder,
	required *corev1.ConfigMap,
) (*corev1.ConfigMap, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("ConfigMap %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.ConfigMaps(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.ConfigMaps(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "ConfigMap", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		// This is not the place to handle adoption.
		err := fmt.Errorf("configmap %q isn't controlled by us", naming.ObjRef(requiredCopy))
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
	actual, err := client.ConfigMaps(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "ConfigMap", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update configmap: %w", err)
	}
	return actual, true, nil
}

// ApplyServiceAccount will apply the ServiceAccount to match the required object.
func ApplyServiceAccount(
	ctx context.Context,
	client corev1client.ServiceAccountsGetter,
	lister corev1listers.ServiceAccountLister,
	recorder record.EventRecorder,
	required *corev1.ServiceAccount,
	forceOwnership bool,
	allowMissingControllerRef bool,
) (*corev1.ServiceAccount, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if !allowMissingControllerRef && requiredControllerRef == nil {
		return nil, false, fmt.Errorf("ServiceAccount %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.ServiceAccounts(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.ServiceAccounts(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "ServiceAccount", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)

	existingControllerRefUID := types.UID("")
	if existingControllerRef != nil {
		existingControllerRefUID = existingControllerRef.UID
	}
	requiredControllerRefUID := types.UID("")
	if requiredControllerRef != nil {
		requiredControllerRefUID = requiredControllerRef.UID
	}

	if existingControllerRef == nil && requiredControllerRef != nil && forceOwnership {
		klog.V(2).InfoS("Forcing apply to claim the ServiceAccount", "ServiceAccount", naming.ObjRef(requiredCopy))
	} else if existingControllerRefUID != requiredControllerRefUID {
		// This is not the place to handle adoption.
		err := fmt.Errorf("serviceAccount %q isn't controlled by us", naming.ObjRef(requiredCopy))
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
	actual, err := client.ServiceAccounts(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "ServiceAccount", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update serviceaccount: %w", err)
	}
	return actual, true, nil
}

// ApplyNamespace will apply the Namespace to match the required object.
func ApplyNamespace(
	ctx context.Context,
	client corev1client.NamespacesGetter,
	lister corev1listers.NamespaceLister,
	recorder record.EventRecorder,
	required *corev1.Namespace,
	allowMissingControllerRef bool,
) (*corev1.Namespace, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if !allowMissingControllerRef && requiredControllerRef == nil {
		return nil, false, fmt.Errorf("namespace %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	existing, err := lister.Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := client.Namespaces().Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Namespace", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if !equality.Semantic.DeepEqual(existingControllerRef, requiredControllerRef) {
		// This is not the place to handle adoption.
		err := fmt.Errorf("namespace %q isn't controlled by us", naming.ObjRef(requiredCopy))
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
	actual, err := client.Namespaces().Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Namespace", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update namespace: %w", err)
	}
	return actual, true, nil
}
