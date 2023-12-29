package resourceapply

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyClusterRoleWithControl(
	ctx context.Context,
	control ApplyControlInterface[*rbacv1.ClusterRole],
	recorder record.EventRecorder,
	required *rbacv1.ClusterRole,
	options ApplyOptions,
) (*rbacv1.ClusterRole, bool, error) {
	return ApplyGeneric[*rbacv1.ClusterRole](ctx, control, recorder, required, options)
}

func ApplyClusterRole(
	ctx context.Context,
	client rbacv1client.ClusterRolesGetter,
	lister rbacv1listers.ClusterRoleLister,
	recorder record.EventRecorder,
	required *rbacv1.ClusterRole,
	options ApplyOptions,
) (*rbacv1.ClusterRole, bool, error) {
	return ApplyClusterRoleWithControl(
		ctx,
		ApplyControlFuncs[*rbacv1.ClusterRole]{
			GetCachedFunc: lister.Get,
			CreateFunc:    client.ClusterRoles().Create,
			UpdateFunc:    client.ClusterRoles().Update,
			DeleteFunc:    client.ClusterRoles().Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyRoleBindingWithControl(
	ctx context.Context,
	control ApplyControlInterface[*rbacv1.RoleBinding],
	recorder record.EventRecorder,
	required *rbacv1.RoleBinding,
	options ApplyOptions,
) (*rbacv1.RoleBinding, bool, error) {
	return ApplyGenericWithHandlers[*rbacv1.RoleBinding](
		ctx,
		control,
		recorder,
		required,
		options,
		nil,
		func(required *rbacv1.RoleBinding, existing *rbacv1.RoleBinding) (string, *metav1.DeletionPropagation, error) {
			if !equality.Semantic.DeepEqual(existing.RoleRef, (*required).RoleRef) {
				return "roleRef is immutable", nil, nil
			}
			return "", nil, nil
		},
	)
}

func ApplyRoleBinding(
	ctx context.Context,
	client rbacv1client.RoleBindingsGetter,
	lister rbacv1listers.RoleBindingLister,
	recorder record.EventRecorder,
	required *rbacv1.RoleBinding,
	options ApplyOptions,
) (*rbacv1.RoleBinding, bool, error) {
	return ApplyRoleBindingWithControl(
		ctx,
		ApplyControlFuncs[*rbacv1.RoleBinding]{
			GetCachedFunc: lister.RoleBindings(required.Namespace).Get,
			CreateFunc:    client.RoleBindings(required.Namespace).Create,
			UpdateFunc:    client.RoleBindings(required.Namespace).Update,
			DeleteFunc:    client.RoleBindings(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyClusterRoleBindingWithControl(
	ctx context.Context,
	control ApplyControlInterface[*rbacv1.ClusterRoleBinding],
	recorder record.EventRecorder,
	required *rbacv1.ClusterRoleBinding,
	options ApplyOptions,
) (*rbacv1.ClusterRoleBinding, bool, error) {
	return ApplyGenericWithHandlers[*rbacv1.ClusterRoleBinding](
		ctx,
		control,
		recorder,
		required,
		options,
		nil,
		func(required *rbacv1.ClusterRoleBinding, existing *rbacv1.ClusterRoleBinding) (string, *metav1.DeletionPropagation, error) {
			if !equality.Semantic.DeepEqual(existing.RoleRef, (*required).RoleRef) {
				return "roleRef is immutable", nil, nil
			}
			return "", nil, nil
		},
	)
}

func ApplyClusterRoleBinding(
	ctx context.Context,
	client rbacv1client.ClusterRoleBindingsGetter,
	lister rbacv1listers.ClusterRoleBindingLister,
	recorder record.EventRecorder,
	required *rbacv1.ClusterRoleBinding,
	options ApplyOptions,
) (*rbacv1.ClusterRoleBinding, bool, error) {
	return ApplyClusterRoleBindingWithControl(
		ctx,
		ApplyControlFuncs[*rbacv1.ClusterRoleBinding]{
			GetCachedFunc: lister.Get,
			CreateFunc:    client.ClusterRoleBindings().Create,
			UpdateFunc:    client.ClusterRoleBindings().Update,
			DeleteFunc:    client.ClusterRoleBindings().Delete,
		},
		recorder,
		required,
		options,
	)
}
