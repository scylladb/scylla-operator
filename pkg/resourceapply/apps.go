package resourceapply

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyStatefulSetWithControl(
	ctx context.Context,
	control ApplyControlInterface[*appsv1.StatefulSet],
	recorder record.EventRecorder,
	required *appsv1.StatefulSet,
	options ApplyOptions,
) (*appsv1.StatefulSet, bool, error) {
	return ApplyGenericWithHandlers[*appsv1.StatefulSet](
		ctx,
		control,
		recorder,
		required,
		options,
		nil,
		func(required *appsv1.StatefulSet, existing *appsv1.StatefulSet) string {
			if !equality.Semantic.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
				return "spec.selector is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.VolumeClaimTemplates, required.Spec.VolumeClaimTemplates) {
				return "spec.volumeClaimTemplates is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.ServiceName, required.Spec.ServiceName) {
				return "spec.serviceName is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.PodManagementPolicy, required.Spec.PodManagementPolicy) {
				return "spec.podManagementPolicy is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.UpdateStrategy, required.Spec.UpdateStrategy) {
				return "spec.updateStrategy is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.RevisionHistoryLimit, required.Spec.RevisionHistoryLimit) {
				return "spec.revisionHistoryLimit is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.Ordinals, required.Spec.Ordinals) {
				return "spec.ordinals is immutable"
			}

			return ""
		},
	)
}

func ApplyStatefulSet(
	ctx context.Context,
	client appsv1client.StatefulSetsGetter,
	lister appsv1listers.StatefulSetLister,
	recorder record.EventRecorder,
	required *appsv1.StatefulSet,
	options ApplyOptions,
) (*appsv1.StatefulSet, bool, error) {
	return ApplyStatefulSetWithControl(
		ctx,
		ApplyControlFuncs[*appsv1.StatefulSet]{
			GetCachedFunc: lister.StatefulSets(required.Namespace).Get,
			CreateFunc:    client.StatefulSets(required.Namespace).Create,
			UpdateFunc:    client.StatefulSets(required.Namespace).Update,
			DeleteFunc:    client.StatefulSets(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyDaemonSetWithControl(
	ctx context.Context,
	control ApplyControlInterface[*appsv1.DaemonSet],
	recorder record.EventRecorder,
	required *appsv1.DaemonSet,
	options ApplyOptions,
) (*appsv1.DaemonSet, bool, error) {
	return ApplyGenericWithHandlers[*appsv1.DaemonSet](
		ctx,
		control,
		recorder,
		required,
		options,
		nil,
		func(required *appsv1.DaemonSet, existing *appsv1.DaemonSet) string {
			if !equality.Semantic.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
				return "spec.selector is immutable"
			}
			return ""
		},
	)
}

func ApplyDaemonSet(
	ctx context.Context,
	client appsv1client.DaemonSetsGetter,
	lister appsv1listers.DaemonSetLister,
	recorder record.EventRecorder,
	required *appsv1.DaemonSet,
	options ApplyOptions,
) (*appsv1.DaemonSet, bool, error) {
	return ApplyDaemonSetWithControl(
		ctx,
		ApplyControlFuncs[*appsv1.DaemonSet]{
			GetCachedFunc: lister.DaemonSets(required.Namespace).Get,
			CreateFunc:    client.DaemonSets(required.Namespace).Create,
			UpdateFunc:    client.DaemonSets(required.Namespace).Update,
			DeleteFunc:    client.DaemonSets(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyDeploymentWithControl(
	ctx context.Context,
	control ApplyControlInterface[*appsv1.Deployment],
	recorder record.EventRecorder,
	required *appsv1.Deployment,
	options ApplyOptions,
) (*appsv1.Deployment, bool, error) {
	return ApplyGeneric[*appsv1.Deployment](ctx, control, recorder, required, options)
}

func ApplyDeployment(
	ctx context.Context,
	client appsv1client.DeploymentsGetter,
	lister appsv1listers.DeploymentLister,
	recorder record.EventRecorder,
	required *appsv1.Deployment,
	options ApplyOptions,
) (*appsv1.Deployment, bool, error) {
	return ApplyDeploymentWithControl(
		ctx,
		ApplyControlFuncs[*appsv1.Deployment]{
			GetCachedFunc: lister.Deployments(required.Namespace).Get,
			CreateFunc:    client.Deployments(required.Namespace).Create,
			UpdateFunc:    client.Deployments(required.Namespace).Update,
			DeleteFunc:    client.Deployments(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
