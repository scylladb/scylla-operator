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

type DaemonSetControlInterface interface {
	PatchDaemonSet(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealDaemonSetControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ DaemonSetControlInterface = &RealDaemonSetControl{}

func (r RealDaemonSetControl) PatchDaemonSet(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.AppsV1().DaemonSets(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type DaemonSetControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx              context.Context
	controllerGVK    schema.GroupVersionKind
	DaemonSetControl DaemonSetControlInterface
}

func NewDaemonSetControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	DaemonSetControl DaemonSetControlInterface,
) *DaemonSetControllerRefManager {
	return &DaemonSetControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:              ctx,
		controllerGVK:    controllerGVK,
		DaemonSetControl: DaemonSetControl,
	}
}

func (m *DaemonSetControllerRefManager) ClaimDaemonSets(daemonsets []*appsv1.DaemonSet) (map[string]*appsv1.DaemonSet, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptDaemonSet(obj.(*appsv1.DaemonSet))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseDaemonSet(obj.(*appsv1.DaemonSet))
	}

	claimedMap := make(map[string]*appsv1.DaemonSet, len(daemonsets))
	var errors []error
	for _, daemonset := range daemonsets {
		ok, err := m.ClaimObject(daemonset, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[daemonset.Name] = daemonset
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *DaemonSetControllerRefManager) AdoptDaemonSet(daemonset *appsv1.DaemonSet) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt DaemonSet %v/%v (%v): %v", daemonset.Namespace, daemonset.Name, daemonset.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, daemonset.UID)
	if err != nil {
		return err
	}

	return m.DaemonSetControl.PatchDaemonSet(m.ctx, daemonset.Namespace, daemonset.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *DaemonSetControllerRefManager) ReleaseDaemonSet(daemonset *appsv1.DaemonSet) error {
	klog.V(2).InfoS("Patching DaemonSet to remove its controllerRef",
		"DaemonSet",
		klog.KObj(daemonset),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(daemonset.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.DaemonSetControl.PatchDaemonSet(m.ctx, daemonset.Namespace, daemonset.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the DaemonSet no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch DaemonSet as it was missing", klog.KObj(daemonset))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the DaemonSet has no owner reference
			// 2. the UID of the DaemonSet doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch DaemonSet as it was invalid", klog.KObj(daemonset))
			return nil
		}

		return err
	}

	return nil
}
