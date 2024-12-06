package controllerhelpers

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type ControlleeManagerGetObjectsInterface[CT, T kubeinterfaces.ObjectInterface] interface {
	GetControllerUncached(ctx context.Context, name string, opts metav1.GetOptions) (CT, error)
	ListObjects(selector labels.Selector) ([]T, error)
	PatchObject(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

type ControlleeManagerGetObjectsFuncs[CT, T kubeinterfaces.ObjectInterface] struct {
	GetControllerUncachedFunc func(ctx context.Context, name string, opts metav1.GetOptions) (CT, error)
	ListObjectsFunc           func(selector labels.Selector) ([]T, error)
	PatchObjectFunc           func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

func (f ControlleeManagerGetObjectsFuncs[CT, T]) GetControllerUncached(ctx context.Context, name string, opts metav1.GetOptions) (CT, error) {
	return f.GetControllerUncachedFunc(ctx, name, opts)
}

func (f ControlleeManagerGetObjectsFuncs[CT, T]) ListObjects(selector labels.Selector) ([]T, error) {
	return f.ListObjectsFunc(selector)
}

func (f ControlleeManagerGetObjectsFuncs[CT, T]) PatchObject(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error) {
	return f.PatchObjectFunc(ctx, name, pt, data, opts, subresources...)
}

var _ ControlleeManagerGetObjectsInterface[kubeinterfaces.ObjectInterface, kubeinterfaces.ObjectInterface] = ControlleeManagerGetObjectsFuncs[kubeinterfaces.ObjectInterface, kubeinterfaces.ObjectInterface]{}

func GetObjectsWithFilter[CT, T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	filterFunc func(T) bool,
	control ControlleeManagerGetObjectsInterface[CT, T],
) (map[string]T, error) {
	// List all objects to find even those that no longer match our selector.
	// They will be orphaned in ClaimObjects().
	allObjects, err := control.ListObjects(labels.Everything())
	if err != nil {
		return nil, err
	}

	var objects []T
	for i := range allObjects {
		if filterFunc(allObjects[i]) {
			objects = append(objects, allObjects[i])
		}
	}

	return controllertools.NewControllerRefManager[T](
		ctx,
		controller,
		controllerGVK,
		selector,
		controllertools.ControllerRefManagerControlFuncsConverter[CT, T]{
			GetControllerUncachedFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (CT, error) {
				return control.GetControllerUncached(ctx, name, opts)
			},
			PatchObjectFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error) {
				return control.PatchObject(ctx, name, pt, data, opts, subresources...)
			},
		}.Convert(),
	).ClaimObjects(objects)
}

func GetObjects[CT, T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	control ControlleeManagerGetObjectsInterface[CT, T],
) (map[string]T, error) {
	return GetObjectsWithFilter(
		ctx,
		controller,
		controllerGVK,
		selector,
		func(T) bool {
			return true
		},
		control,
	)
}

func GetCustomResourceObjects[CT, T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	control ControlleeManagerGetObjectsInterface[CT, T],
) (map[string]T, error) {
	allObjects, err := control.ListObjects(labels.Everything())
	if err != nil {
		return nil, err
	}

	crm := controllertools.NewControllerRefManager[T](
		ctx,
		controller,
		controllerGVK,
		selector,
		controllertools.ControllerRefManagerControlFuncsConverter[CT, T]{
			GetControllerUncachedFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (CT, error) {
				return control.GetControllerUncached(ctx, name, opts)
			},
			PatchObjectFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error) {
				return control.PatchObject(ctx, name, pt, data, opts, subresources...)
			},
		}.Convert(),
	)

	// StrategicMergePatch used by ControllerRefManager by default during object releasing is not supported on CRDs.
	crm.GetReleasePatchBytes = GetDeleteOwnerReferenceMergePatchBytes
	crm.ReleasePatchType = types.MergePatchType

	return crm.ClaimObjects(allObjects)
}

func RunSync(conditions *[]metav1.Condition, progressingConditionType, degradedCondType string, observedGeneration int64, syncFn func() ([]metav1.Condition, error)) error {
	progressingConditions, err := syncFn()
	SetStatusConditionFromError(conditions, err, degradedCondType, observedGeneration)
	if err != nil {
		return err
	}

	progressingCondition, err := AggregateStatusConditions(
		progressingConditions,
		metav1.Condition{
			Type:               progressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: observedGeneration,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate progressing conditions %q: %w", progressingConditionType, err)
	}
	apimeta.SetStatusCondition(conditions, progressingCondition)

	return nil
}

func SetAggregatedWorkloadConditions(conditions *[]metav1.Condition, generation int64) error {
	availableCondition, err := AggregateStatusConditions(
		FindStatusConditionsWithSuffix(*conditions, scyllav1.AvailableCondition),
		metav1.Condition{
			Type:               scyllav1.AvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(conditions, availableCondition)

	progressingCondition, err := AggregateStatusConditions(
		FindStatusConditionsWithSuffix(*conditions, scyllav1.ProgressingCondition),
		metav1.Condition{
			Type:               scyllav1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(conditions, progressingCondition)

	degradedCondition, err := AggregateStatusConditions(
		FindStatusConditionsWithSuffix(*conditions, scyllav1.DegradedCondition),
		metav1.Condition{
			Type:               scyllav1.DegradedCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(conditions, degradedCondition)

	return nil
}

type ClusterControlleeManagerGetObjectsInterface[CT, T kubeinterfaces.ObjectInterface] interface {
	Cluster(string) (ControlleeManagerGetObjectsInterface[CT, T], error)
}

type ClusterControlleeManagerGetObjectsFuncs[CT, T kubeinterfaces.ObjectInterface] struct {
	ClusterFunc func(string) (ControlleeManagerGetObjectsInterface[CT, T], error)
}

func (c *ClusterControlleeManagerGetObjectsFuncs[CT, T]) Cluster(cluster string) (ControlleeManagerGetObjectsInterface[CT, T], error) {
	return c.ClusterFunc(cluster)
}

func GetRemoteObjects[CT, T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	remoteClusters []string,
	controllerMap map[string]metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	control ClusterControlleeManagerGetObjectsInterface[CT, T],
) (map[string]map[string]T, error) {
	remoteObjectMapMap := make(map[string]map[string]T, len(remoteClusters))
	for _, remoteCluster := range remoteClusters {
		clusterControl, err := control.Cluster(remoteCluster)
		if err != nil {
			return nil, fmt.Errorf("can't get cluster %q control: %w", remoteCluster, err)
		}
		if clusterControl == ControlleeManagerGetObjectsInterface[CT, T](nil) {
			klog.InfoS("Cluster control is not yet available, it may not have been created yet", "Cluster", remoteCluster)
			return nil, nil
		}

		controller, ok := controllerMap[remoteCluster]
		if !ok {
			klog.InfoS("Controller object for cluster is missing, it may not have been created yet", "Cluster", remoteCluster)
			remoteObjectMapMap[remoteCluster] = make(map[string]T)
			continue
		}

		objs, err := GetObjects[CT, T](
			ctx,
			controller,
			controllerGVK,
			selector,
			clusterControl,
		)
		if err != nil {
			return nil, fmt.Errorf("can't get objects: %w", err)
		}

		remoteObjectMapMap[remoteCluster] = objs
	}

	return remoteObjectMapMap, nil
}
