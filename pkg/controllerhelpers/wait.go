package controllerhelpers

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type listerWatcher[ListObject runtime.Object] interface {
	List(context.Context, metav1.ListOptions) (ListObject, error)
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}

type WaitForStateOptions struct {
	TolerateDelete bool
}

func WaitForObjectState[Object, ListObject runtime.Object](ctx context.Context, client listerWatcher[ListObject], name string, options WaitForStateOptions, condition func(obj Object) (bool, error), additionalConditions ...func(obj Object) (bool, error)) (Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
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

		case watch.Error:
			return true, apierrors.FromObject(event.Object)

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

func WaitForScyllaClusterState(ctx context.Context, client scyllav1client.ScyllaClusterInterface, name string, options WaitForStateOptions, condition func(*scyllav1.ScyllaCluster) (bool, error), additionalConditions ...func(*scyllav1.ScyllaCluster) (bool, error)) (*scyllav1.ScyllaCluster, error) {
	return WaitForObjectState[*scyllav1.ScyllaCluster, *scyllav1.ScyllaClusterList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForScyllaDBMonitoringState(ctx context.Context, client scyllav1alpha1client.ScyllaDBMonitoringInterface, name string, options WaitForStateOptions, condition func(monitoring *scyllav1alpha1.ScyllaDBMonitoring) (bool, error), additionalConditions ...func(monitoring *scyllav1alpha1.ScyllaDBMonitoring) (bool, error)) (*scyllav1alpha1.ScyllaDBMonitoring, error) {
	return WaitForObjectState[*scyllav1alpha1.ScyllaDBMonitoring, *scyllav1alpha1.ScyllaDBMonitoringList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForPodState(ctx context.Context, client corev1client.PodInterface, name string, options WaitForStateOptions, condition func(*corev1.Pod) (bool, error), additionalConditions ...func(*corev1.Pod) (bool, error)) (*corev1.Pod, error) {
	return WaitForObjectState[*corev1.Pod, *corev1.PodList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForServiceAccountState(ctx context.Context, client corev1client.ServiceAccountInterface, name string, options WaitForStateOptions, condition func(*corev1.ServiceAccount) (bool, error), additionalConditions ...func(*corev1.ServiceAccount) (bool, error)) (*corev1.ServiceAccount, error) {
	return WaitForObjectState[*corev1.ServiceAccount, *corev1.ServiceAccountList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForRoleBindingState(ctx context.Context, client rbacv1client.RoleBindingInterface, name string, options WaitForStateOptions, condition func(*rbacv1.RoleBinding) (bool, error), additionalConditions ...func(*rbacv1.RoleBinding) (bool, error)) (*rbacv1.RoleBinding, error) {
	return WaitForObjectState[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForPVCState(ctx context.Context, client corev1client.PersistentVolumeClaimInterface, name string, options WaitForStateOptions, condition func(*corev1.PersistentVolumeClaim) (bool, error), additionalConditions ...func(*corev1.PersistentVolumeClaim) (bool, error)) (*corev1.PersistentVolumeClaim, error) {
	return WaitForObjectState[*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaimList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForNodeConfigState(ctx context.Context, ncClient scyllav1alpha1client.NodeConfigInterface, name string, options WaitForStateOptions, condition func(*scyllav1alpha1.NodeConfig) (bool, error), additionalConditions ...func(*scyllav1alpha1.NodeConfig) (bool, error)) (*scyllav1alpha1.NodeConfig, error) {
	return WaitForObjectState[*scyllav1alpha1.NodeConfig, *scyllav1alpha1.NodeConfigList](ctx, ncClient, name, options, condition, additionalConditions...)
}

func WaitForConfigMapState(ctx context.Context, client corev1client.ConfigMapInterface, name string, options WaitForStateOptions, condition func(*corev1.ConfigMap) (bool, error), additionalConditions ...func(*corev1.ConfigMap) (bool, error)) (*corev1.ConfigMap, error) {
	return WaitForObjectState[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForSecretState(ctx context.Context, client corev1client.SecretInterface, name string, options WaitForStateOptions, condition func(*corev1.Secret) (bool, error), additionalConditions ...func(*corev1.Secret) (bool, error)) (*corev1.Secret, error) {
	return WaitForObjectState[*corev1.Secret, *corev1.SecretList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForServiceState(ctx context.Context, client corev1client.ServiceInterface, name string, options WaitForStateOptions, condition func(*corev1.Service) (bool, error), additionalConditions ...func(*corev1.Service) (bool, error)) (*corev1.Service, error) {
	return WaitForObjectState[*corev1.Service, *corev1.ServiceList](ctx, client, name, options, condition, additionalConditions...)
}
