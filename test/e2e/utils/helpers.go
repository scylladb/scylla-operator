// Copyright (c) 2022 ScyllaDB.

package utils

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type listerWatcher[ListObject runtime.Object] interface {
	List(context.Context, v1.ListOptions) (ListObject, error)
	Watch(context.Context, v1.ListOptions) (watch.Interface, error)
}

type WaitForStateOptions struct {
	TolerateDelete bool
}

func WaitForObjectState[Object, ListObject runtime.Object](ctx context.Context, client listerWatcher[ListObject], name string, options WaitForStateOptions, condition func(obj Object) (bool, error), additionalConditions ...func(obj Object) (bool, error)) (Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options v1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.List(ctx, options)
		}),
		WatchFunc: func(options v1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.Watch(ctx, options)
		},
	}

	conditions := make([]func(Object) (bool, error), 0, 1+len(additionalConditions))
	conditions = append(conditions, condition)
	if len(additionalConditions) != 0 {
		conditions = append(conditions, additionalConditions...)
	}
	aggregatedCond := func(obj Object) (bool, error) {
		allDone := true
		for _, c := range conditions {
			var err error
			var done bool

			done, err = c(obj)
			if err != nil {
				return done, err
			}
			if !done {
				allDone = false
			}
		}
		return allDone, nil
	}

	event, err := watchtools.UntilWithSync(ctx, lw, *new(Object), nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return aggregatedCond(event.Object.(Object))

		case watch.Deleted:
			if options.TolerateDelete {
				return aggregatedCond(event.Object.(Object))
			}
			fallthrough

		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return *new(Object), err
	}

	return event.Object.(Object), nil
}

func WaitForPodState(ctx context.Context, client corev1client.PodInterface, name string, options WaitForStateOptions, condition func(*corev1.Pod) (bool, error), additionalConditions ...func(*corev1.Pod) (bool, error)) (*corev1.Pod, error) {
	return WaitForObjectState[*corev1.Pod, *corev1.PodList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForServiceAccountState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*corev1.ServiceAccount) (bool, error), additionalConditions ...func(*corev1.ServiceAccount) (bool, error)) (*corev1.ServiceAccount, error) {
	return WaitForObjectState[*corev1.ServiceAccount, *corev1.ServiceAccountList](ctx, client.ServiceAccounts(namespace), name, options, condition, additionalConditions...)
}

func WaitForRoleBindingState(ctx context.Context, client rbacv1client.RbacV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*rbacv1.RoleBinding) (bool, error), additionalConditions ...func(*rbacv1.RoleBinding) (bool, error)) (*rbacv1.RoleBinding, error) {
	return WaitForObjectState[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctx, client.RoleBindings(namespace), name, options, condition, additionalConditions...)
}

func WaitForPVCState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*corev1.PersistentVolumeClaim) (bool, error), additionalConditions ...func(*corev1.PersistentVolumeClaim) (bool, error)) (*corev1.PersistentVolumeClaim, error) {
	return WaitForObjectState[*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaimList](ctx, client.PersistentVolumeClaims(namespace), name, options, condition, additionalConditions...)
}

func WaitForConfigMapState(ctx context.Context, client corev1client.ConfigMapInterface, name string, options WaitForStateOptions, condition func(*corev1.ConfigMap) (bool, error), additionalConditions ...func(*corev1.ConfigMap) (bool, error)) (*corev1.ConfigMap, error) {
	return WaitForObjectState[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForSecretState(ctx context.Context, client corev1client.SecretInterface, name string, options WaitForStateOptions, condition func(*corev1.Secret) (bool, error), additionalConditions ...func(*corev1.Secret) (bool, error)) (*corev1.Secret, error) {
	return WaitForObjectState[*corev1.Secret, *corev1.SecretList](ctx, client, name, options, condition, additionalConditions...)
}
