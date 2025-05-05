package controllerhelpers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
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

type AggregatedConditions[Obj runtime.Object] struct {
	conditions []func(obj Obj) (bool, error)
	state      []*bool
}

func NewAggregatedConditions[Obj runtime.Object](condition func(obj Obj) (bool, error), additionalConditions ...func(obj Obj) (bool, error)) *AggregatedConditions[Obj] {
	conditions := make([]func(obj Obj) (bool, error), 0, 1+len(additionalConditions))
	conditions = append(conditions, condition)
	if len(additionalConditions) != 0 {
		conditions = append(conditions, additionalConditions...)
	}

	return &AggregatedConditions[Obj]{
		conditions: conditions,
		state:      make([]*bool, len(conditions)),
	}
}

func (ac AggregatedConditions[Obj]) clearState() {
	for i := range ac.state {
		ac.state[i] = nil
	}
}

func (ac AggregatedConditions[Obj]) Condition(obj Obj) (bool, error) {
	ac.clearState()

	allDone := true
	var err error
	for i, cond := range ac.conditions {
		var done bool
		done, err = cond(obj)
		ac.state[i] = &done
		if err != nil {
			return done, err
		}

		if !done {
			allDone = false
		}
	}

	return allDone, nil
}

func (ac AggregatedConditions[Obj]) GetStateString() string {
	var sb strings.Builder

	sb.WriteString("[")

	for i, v := range ac.state {
		if v == nil {
			sb.WriteString("<nil>")
		} else {
			sb.WriteString(strconv.FormatBool(*v))
		}

		if i != len(ac.state)-1 {
			sb.WriteString(",")
		}
	}

	sb.WriteString("]")

	return sb.String()
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

	acs := NewAggregatedConditions(condition, additionalConditions...)
	event, err := watchtools.UntilWithSync(ctx, lw, *new(Object), nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return acs.Condition(event.Object.(Object))

		case watch.Error:
			return true, apierrors.FromObject(event.Object)

		case watch.Deleted:
			if options.TolerateDelete {
				return acs.Condition(event.Object.(Object))
			}
			fallthrough

		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		if apimachineryutilwait.Interrupted(err) {
			err = fmt.Errorf("waiting has been interupted (%s): %w", acs.GetStateString(), err)
		}
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

func WaitForDaemonSetState(ctx context.Context, client appsv1client.DaemonSetInterface, name string, options WaitForStateOptions, condition func(*appsv1.DaemonSet) (bool, error), additionalConditions ...func(set *appsv1.DaemonSet) (bool, error)) (*appsv1.DaemonSet, error) {
	return WaitForObjectState[*appsv1.DaemonSet, *appsv1.DaemonSetList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForRemoteKubernetesClusterState(ctx context.Context, client scyllav1alpha1client.RemoteKubernetesClusterInterface, name string, options WaitForStateOptions, condition func(*scyllav1alpha1.RemoteKubernetesCluster) (bool, error), additionalConditions ...func(*scyllav1alpha1.RemoteKubernetesCluster) (bool, error)) (*scyllav1alpha1.RemoteKubernetesCluster, error) {
	return WaitForObjectState[*scyllav1alpha1.RemoteKubernetesCluster, *scyllav1alpha1.RemoteKubernetesClusterList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForScyllaDBClusterState(ctx context.Context, client scyllav1alpha1client.ScyllaDBClusterInterface, name string, options WaitForStateOptions, condition func(cluster *scyllav1alpha1.ScyllaDBCluster) (bool, error), additionalConditions ...func(cluster *scyllav1alpha1.ScyllaDBCluster) (bool, error)) (*scyllav1alpha1.ScyllaDBCluster, error) {
	return WaitForObjectState[*scyllav1alpha1.ScyllaDBCluster, *scyllav1alpha1.ScyllaDBClusterList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForScyllaDBDatacenterState(ctx context.Context, client scyllav1alpha1client.ScyllaDBDatacenterInterface, name string, options WaitForStateOptions, condition func(cluster *scyllav1alpha1.ScyllaDBDatacenter) (bool, error), additionalConditions ...func(*scyllav1alpha1.ScyllaDBDatacenter) (bool, error)) (*scyllav1alpha1.ScyllaDBDatacenter, error) {
	return WaitForObjectState[*scyllav1alpha1.ScyllaDBDatacenter, *scyllav1alpha1.ScyllaDBDatacenterList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForScyllaDBManagerClusterRegistrationState(ctx context.Context, client scyllav1alpha1client.ScyllaDBManagerClusterRegistrationInterface, name string, options WaitForStateOptions, condition func(cluster *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (bool, error), additionalConditions ...func(*scyllav1alpha1.ScyllaDBManagerClusterRegistration) (bool, error)) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	return WaitForObjectState[*scyllav1alpha1.ScyllaDBManagerClusterRegistration, *scyllav1alpha1.ScyllaDBManagerClusterRegistrationList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForScyllaDBManagerTaskState(ctx context.Context, client scyllav1alpha1client.ScyllaDBManagerTaskInterface, name string, options WaitForStateOptions, condition func(*scyllav1alpha1.ScyllaDBManagerTask) (bool, error), additionalConditions ...func(task *scyllav1alpha1.ScyllaDBManagerTask) (bool, error)) (*scyllav1alpha1.ScyllaDBManagerTask, error) {
	return WaitForObjectState[*scyllav1alpha1.ScyllaDBManagerTask, *scyllav1alpha1.ScyllaDBManagerTaskList](ctx, client, name, options, condition, additionalConditions...)
}
