package controllertools

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/resource"
	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

type ControllerRefManagerControlInterface interface {
	PatchObject(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
	GetControllerUncached(ctx context.Context, namespace, name string) (metav1.Object, error)
}

type ControllerRefManagerControlFuncs struct {
	PatchObjectFunc           func(ctx context.Context, namespace string, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
	GetControllerUncachedFunc func(ctx context.Context, namespace, name string) (metav1.Object, error)
}

func (cf ControllerRefManagerControlFuncs) PatchObject(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	return cf.PatchObjectFunc(ctx, namespace, name, pt, data, options)
}

func (cf ControllerRefManagerControlFuncs) GetControllerUncached(ctx context.Context, namespace, name string) (metav1.Object, error) {
	return cf.GetControllerUncachedFunc(ctx, namespace, name)
}

var _ ControllerRefManagerControlInterface = ControllerRefManagerControlFuncs{}

type ControllerRefManagerControlFuncsConverter[CT, T kubeinterfaces.ObjectInterface] struct {
	GetControllerUncachedFunc func(ctx context.Context, name string, opts metav1.GetOptions) (CT, error)
	PatchObjectFunc           func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

func (c ControllerRefManagerControlFuncsConverter[CT, T]) Convert() ControllerRefManagerControlFuncs {
	return ControllerRefManagerControlFuncs{
		GetControllerUncachedFunc: func(ctx context.Context, namespace, name string) (metav1.Object, error) {
			return c.GetControllerUncachedFunc(ctx, name, metav1.GetOptions{})
		},
		PatchObjectFunc: func(ctx context.Context, namespace string, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
			_, err := c.PatchObjectFunc(ctx, name, pt, data, options)
			return err
		},
	}
}

type ControllerRefManager[T kubeinterfaces.ObjectInterface] struct {
	kubecontroller.BaseControllerRefManager
	ctx           context.Context
	controllerGVK schema.GroupVersionKind
	Control       ControllerRefManagerControlInterface

	ReleasePatchType     types.PatchType
	GetReleasePatchBytes func(obj metav1.Object, controllerUID types.UID) ([]byte, error)
}

type GetReleasePatchBytesFunc func(obj metav1.Object, controllerUID types.UID) ([]byte, error)

func NewControllerRefManager[T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	control ControllerRefManagerControlInterface,
) *ControllerRefManager[T] {
	crm := &ControllerRefManager[T]{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller: controller,
			Selector:   selector,
		},
		ctx:           ctx,
		controllerGVK: controllerGVK,
		Control:       control,
	}

	crm.CanAdoptFunc = crm.canAdopt
	crm.GetReleasePatchBytes = crm.getReleasePatchBytesFunc()
	crm.ReleasePatchType = types.StrategicMergePatchType

	return crm
}

func (m *ControllerRefManager[T]) getReleasePatchBytesFunc() GetReleasePatchBytesFunc {
	return func(obj metav1.Object, controllerUID types.UID) ([]byte, error) {
		return kubecontroller.DeleteOwnerRefStrategicMergePatch(obj.GetUID(), controllerUID)
	}
}

func (m *ControllerRefManager[T]) canAdopt() error {
	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached read.
	fresh, err := m.Control.GetControllerUncached(m.ctx, m.Controller.GetNamespace(), m.Controller.GetName())
	if err != nil {
		return err
	}

	if fresh.GetUID() != m.Controller.GetUID() {
		return fmt.Errorf("original %s controller %s/%s is gone: got uid %s, wanted %s", m.controllerGVK, m.Controller.GetNamespace(), m.Controller.GetName(), fresh.GetUID(), m.Controller.GetUID())
	}

	if fresh.GetDeletionTimestamp() != nil {
		return fmt.Errorf("%s %s/%s has just been deleted at %v", m.controllerGVK, m.Controller.GetNamespace(), m.Controller.GetName(), m.Controller.GetDeletionTimestamp())
	}

	return nil
}

func (m *ControllerRefManager[T]) ClaimObjects(objects []T) (map[string]T, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptObject(obj.(T))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseObject(obj.(T))
	}

	claimedMap := make(map[string]T, len(objects))
	var errors []error
	for _, obj := range objects {
		ok, err := m.ClaimObject(obj, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[obj.GetName()] = obj
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *ControllerRefManager[T]) AdoptObject(obj T) error {
	err := m.CanAdopt()
	if err != nil {
		gvk := resource.GetObjectGVKOrUnknown(obj)
		return fmt.Errorf("can't adopt %s %s/%s (%s): %w", gvk, obj.GetNamespace(), obj.GetName(), obj.GetUID(), err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, obj.GetUID())
	if err != nil {
		return err
	}

	return m.Control.PatchObject(m.ctx, obj.GetNamespace(), obj.GetName(), types.MergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *ControllerRefManager[T]) ReleaseObject(obj T) error {
	gvk := resource.GetObjectGVKOrUnknown(obj)

	klog.V(2).InfoS("Removing controllerRef",
		"GVK", gvk,
		"Object", klog.KObj(obj),
		"ControllerRef", fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := m.GetReleasePatchBytes(obj, m.Controller.GetUID())
	err = m.Control.PatchObject(m.ctx, obj.GetNamespace(), obj.GetName(), m.ReleasePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the object no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch %s as it was missing", gvk, klog.KObj(obj))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the Service has no owner reference
			// 2. the UID of the Service doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch %s as it was invalid", gvk, klog.KObj(obj))
			return nil
		}

		return err
	}

	return nil
}
