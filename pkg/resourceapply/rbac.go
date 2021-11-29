package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// ApplyClusterRole will apply the ClusterRole to match the required object.
func ApplyClusterRole(
	ctx context.Context,
	client rbacv1client.ClusterRolesGetter,
	lister rbacv1listers.ClusterRoleLister,
	recorder record.EventRecorder,
	required *rbacv1.ClusterRole,
	allowMissingControllerRef bool,
) (*rbacv1.ClusterRole, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if !allowMissingControllerRef && requiredControllerRef == nil {
		return nil, false, fmt.Errorf("ClusterRole %q is missing controllerRef", naming.ObjRef(required))
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

		actual, err := client.ClusterRoles().Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "ClusterRole", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if !equality.Semantic.DeepEqual(existingControllerRef, requiredControllerRef) {
		// This is not the place to handle adoption.
		err := fmt.Errorf("clusterrole %q isn't controlled by us", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	existingHash := existing.Annotations[naming.ManagedHash]
	requiredHash := requiredCopy.Annotations[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.ResourceVersion) == 0 {
		requiredCopy.ResourceVersion = existing.ResourceVersion
	}
	actual, err := client.ClusterRoles().Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "ClusterRole", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update clusterrole: %w", err)
	}
	return actual, true, nil
}

// ApplyClusterRoleBinding will apply the ClusterRoleBinding to match the required object.
func ApplyClusterRoleBinding(
	ctx context.Context,
	client rbacv1client.ClusterRoleBindingsGetter,
	lister rbacv1listers.ClusterRoleBindingLister,
	recorder record.EventRecorder,
	required *rbacv1.ClusterRoleBinding,
	allowMissingControllerRef bool,
) (*rbacv1.ClusterRoleBinding, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if !allowMissingControllerRef && requiredControllerRef == nil {
		return nil, false, fmt.Errorf("ClusterRoleBinding %q is missing controllerRef", naming.ObjRef(required))
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

		actual, err := client.ClusterRoleBindings().Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "ClusterRoleBinding", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if !equality.Semantic.DeepEqual(existingControllerRef, requiredControllerRef) {
		// This is not the place to handle adoption.
		err := fmt.Errorf("clusterrolebinding %q isn't controlled by us", naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return nil, false, err
	}

	existingHash := existing.Annotations[naming.ManagedHash]
	requiredHash := requiredCopy.Annotations[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	if !equality.Semantic.DeepEqual(existing.RoleRef, requiredCopy.RoleRef) {
		klog.V(2).InfoS(
			"Apply needs to change immutable field(s) and will recreate the object",
			"ClusterRoleBinding", naming.ObjRefWithUID(existing),
		)

		propagationPolicy := metav1.DeletePropagationBackground
		err := client.ClusterRoleBindings().Delete(ctx, existing.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		ReportDeleteEvent(recorder, existing, err)
		if err != nil {
			return nil, false, err
		}

		created, err := client.ClusterRoleBindings().Create(ctx, requiredCopy, metav1.CreateOptions{})
		ReportCreateEvent(recorder, requiredCopy, err)
		if err != nil {
			return nil, false, err
		}

		return created, true, nil
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.ResourceVersion) == 0 {
		requiredCopy.ResourceVersion = existing.ResourceVersion
	}
	actual, err := client.ClusterRoleBindings().Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "ClusterRoleBinding", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update clusterrolebinding: %w", err)
	}
	return actual, true, nil
}
