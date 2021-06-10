package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type StatefulSetControlInterface interface {
	PatchStatefulSet(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealStatefulSetControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ StatefulSetControlInterface = &RealStatefulSetControl{}

func (r RealStatefulSetControl) PatchStatefulSet(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.AppsV1().StatefulSets(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type StatefulSetControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx                context.Context
	controllerGVK      schema.GroupVersionKind
	statefulSetControl StatefulSetControlInterface
}

func NewStatefulSetControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	statefulSetControl StatefulSetControlInterface,
) *StatefulSetControllerRefManager {
	return &StatefulSetControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:                ctx,
		controllerGVK:      controllerGVK,
		statefulSetControl: statefulSetControl,
	}
}

func (m *StatefulSetControllerRefManager) ClaimStatefulSets(sets []*appsv1.StatefulSet) (map[string]*appsv1.StatefulSet, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptStatefulSet(obj.(*appsv1.StatefulSet))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseStatefulSet(obj.(*appsv1.StatefulSet))
	}

	claimedMap := make(map[string]*appsv1.StatefulSet, len(sets))
	var errors []error
	for _, sts := range sets {
		ok, err := m.ClaimObject(sts, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[sts.Name] = sts
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *StatefulSetControllerRefManager) AdoptStatefulSet(statefulSet *appsv1.StatefulSet) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt StatefulSet %v/%v (%v): %v", statefulSet.Namespace, statefulSet.Name, statefulSet.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, statefulSet.UID)
	if err != nil {
		return err
	}

	return m.statefulSetControl.PatchStatefulSet(m.ctx, statefulSet.Namespace, statefulSet.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *StatefulSetControllerRefManager) ReleaseStatefulSet(statefulSet *appsv1.StatefulSet) error {
	klog.V(2).InfoS("Patching StatefulSet to remove its controllerRef",
		"StatefulSet",
		klog.KObj(statefulSet),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(statefulSet.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.statefulSetControl.PatchStatefulSet(m.ctx, statefulSet.Namespace, statefulSet.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the StatefulSet no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch StatefulSet as it was missing", klog.KObj(statefulSet))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the StatefulSet has no owner reference
			// 2. the UID of the StatefulSet doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch StatefulSet as it was invalid", klog.KObj(statefulSet))
			return nil
		}

		return err
	}

	return nil
}
