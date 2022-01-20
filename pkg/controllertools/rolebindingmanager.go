package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type RoleBindingControlInterface interface {
	PatchRoleBinding(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealRoleBindingControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ RoleBindingControlInterface = &RealRoleBindingControl{}

func (r RealRoleBindingControl) PatchRoleBinding(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.RbacV1().RoleBindings(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type RoleBindingControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx                context.Context
	controllerGVK      schema.GroupVersionKind
	RoleBindingControl RoleBindingControlInterface
}

func NewRoleBindingControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	RoleBindingControl RoleBindingControlInterface,
) *RoleBindingControllerRefManager {
	return &RoleBindingControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:                ctx,
		controllerGVK:      controllerGVK,
		RoleBindingControl: RoleBindingControl,
	}
}

func (m *RoleBindingControllerRefManager) ClaimRoleBindings(roleBindings []*rbacv1.RoleBinding) (map[string]*rbacv1.RoleBinding, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptRoleBinding(obj.(*rbacv1.RoleBinding))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseRoleBinding(obj.(*rbacv1.RoleBinding))
	}

	claimedMap := make(map[string]*rbacv1.RoleBinding, len(roleBindings))
	var errors []error
	for _, roleBinding := range roleBindings {
		ok, err := m.ClaimObject(roleBinding, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[roleBinding.Name] = roleBinding
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *RoleBindingControllerRefManager) AdoptRoleBinding(roleBinding *rbacv1.RoleBinding) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt RoleBinding %v/%v (%v): %v", roleBinding.Namespace, roleBinding.Name, roleBinding.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, roleBinding.UID)
	if err != nil {
		return err
	}

	return m.RoleBindingControl.PatchRoleBinding(m.ctx, roleBinding.Namespace, roleBinding.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *RoleBindingControllerRefManager) ReleaseRoleBinding(roleBinding *rbacv1.RoleBinding) error {
	klog.V(2).InfoS("Patching RoleBinding to remove its controllerRef",
		"RoleBinding",
		klog.KObj(roleBinding),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(roleBinding.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.RoleBindingControl.PatchRoleBinding(m.ctx, roleBinding.Namespace, roleBinding.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the RoleBinding no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch RoleBinding as it was missing", klog.KObj(roleBinding))
			return nil
		}

		if apierrors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the RoleBinding has no owner reference
			// 2. the UID of the RoleBinding doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch RoleBinding as it was invalid", klog.KObj(roleBinding))
			return nil
		}

		return err
	}

	return nil
}
