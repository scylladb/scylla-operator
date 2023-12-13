package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
		func(required *appsv1.StatefulSet, existing *appsv1.StatefulSet) (string, *metav1.DeletionPropagation, error) {
			if !equality.Semantic.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
				existingPodLabels := existing.Spec.Template.Labels
				requiredSelector, err := metav1.LabelSelectorAsSelector(required.Spec.Selector)
				if err != nil {
					return "", nil, fmt.Errorf("can't parse required StatefulSet selector: %w", err)
				}

				if !requiredSelector.Matches(labels.Set(existingPodLabels)) {
					return "", nil, fmt.Errorf("required StatefulSet selector %q doesn't match existing Pod Labels set %v", requiredSelector, existingPodLabels)
				}

				return "spec.selector is immutable", pointer.Ptr(metav1.DeletePropagationOrphan), nil
			}

			return "", nil, nil
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
		func(required *appsv1.DaemonSet, existing *appsv1.DaemonSet) (string, *metav1.DeletionPropagation, error) {
			if !equality.Semantic.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
				return "spec.selector is immutable", nil, nil
			}
			return "", nil, nil
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
