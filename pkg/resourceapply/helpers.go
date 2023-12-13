package resourceapply

import (
	"context"
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

func verifyDesiredObject(obj metav1.Object) error {
	if obj.GetUID() != "" {
		return fmt.Errorf("desired objects are not allowed to have UID set")
	}

	if !pointer.Ptr(obj.GetCreationTimestamp()).IsZero() {
		return fmt.Errorf("desired objects are not allowed to have creationTimestamp set")
	}

	if obj.GetGeneration() != 0 {
		return fmt.Errorf("desired objects are not allowed to have generation set")
	}

	if len(obj.GetManagedFields()) != 0 {
		return fmt.Errorf("desired objects are not allowed to contain managedFields")
	}

	if len(obj.GetSelfLink()) != 0 {
		return fmt.Errorf("desired objects are not allowed to have selfLink set")
	}

	return nil
}

func SetHashAnnotation(obj metav1.Object) error {
	err := verifyDesiredObject(obj)
	if err != nil {
		return fmt.Errorf("invalid desider object %q: %w", naming.ObjRef(obj), err)
	}

	// Do not hash ResourceVersion.
	rv := obj.GetResourceVersion()
	obj.SetResourceVersion("")
	defer obj.SetResourceVersion(rv)

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// Clear annotation to have consistent hashing for the same objects.
	delete(annotations, naming.ManagedHash)

	hash, err := hashutil.HashObjects(obj)
	if err != nil {
		return err
	}

	annotations[naming.ManagedHash] = hash
	obj.SetAnnotations(annotations)

	return nil
}

func reportEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error, verb string) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		klog.ErrorS(err, "can't get object metadata")
		return
	}
	gvk, err := resource.GetObjectGVK(obj)
	if err != nil {
		klog.ErrorS(err, "can't determine object GVK", "Object", klog.KObj(objMeta))
		return
	}

	if operationErr != nil {
		recorder.Eventf(
			obj,
			corev1.EventTypeWarning,
			fmt.Sprintf("%s%sFailed", strings.Title(verb), gvk.Kind),
			"Failed to %s %s %s: %v",
			strings.ToLower(verb), gvk.Kind, naming.ObjRef(objMeta), operationErr,
		)
		return
	}
	recorder.Eventf(
		obj,
		corev1.EventTypeNormal,
		fmt.Sprintf("%s%sd", gvk.Kind, strings.Title(verb)),
		"%s %s %sd",
		gvk.Kind, naming.ObjRef(objMeta), verb,
	)
}

func ReportCreateEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	if apierrors.HasStatusCause(operationErr, corev1.NamespaceTerminatingCause) {
		// If the namespace is being terminated, we don't have to do
		// anything because any creation will fail.
		return
	}

	reportEvent(recorder, obj, operationErr, "create")
}

func ReportUpdateEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "update")
}

func ReportDeleteEvent(recorder record.EventRecorder, obj runtime.Object, operationErr error) {
	reportEvent(recorder, obj, operationErr, "delete")
}

type ApplyControlUntypedInterface interface {
	GetCached(name string) (kubeinterfaces.ObjectInterface, error)
	Create(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error)
	Update(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

type ApplyControlUntypedFuncs struct {
	GetCachedFunc func(name string) (kubeinterfaces.ObjectInterface, error)
	CreateFunc    func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error)
	UpdateFunc    func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error)
	DeleteFunc    func(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (acf ApplyControlUntypedFuncs) GetCached(name string) (kubeinterfaces.ObjectInterface, error) {
	return acf.GetCachedFunc(name)
}

func (acf ApplyControlUntypedFuncs) Create(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error) {
	return acf.CreateFunc(ctx, obj, opts)
}

func (acf ApplyControlUntypedFuncs) Update(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error) {
	return acf.UpdateFunc(ctx, obj, opts)
}

func (acf ApplyControlUntypedFuncs) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return acf.DeleteFunc(ctx, name, opts)
}

var _ ApplyControlUntypedInterface = ApplyControlUntypedFuncs{}

type ApplyControlInterface[T kubeinterfaces.ObjectInterface] interface {
	GetCached(name string) (T, error)
	Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error)
	Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

type ApplyControlFuncs[T kubeinterfaces.ObjectInterface] struct {
	GetCachedFunc func(name string) (T, error)
	CreateFunc    func(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error)
	UpdateFunc    func(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
	DeleteFunc    func(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (acf ApplyControlFuncs[T]) GetCached(name string) (T, error) {
	return acf.GetCachedFunc(name)
}

func (acf ApplyControlFuncs[T]) Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
	return acf.CreateFunc(ctx, obj, opts)
}

func (acf ApplyControlFuncs[T]) Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	return acf.UpdateFunc(ctx, obj, opts)
}

func (acf ApplyControlFuncs[T]) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return acf.DeleteFunc(ctx, name, opts)
}

func (acf ApplyControlFuncs[T]) ToUntyped() ApplyControlUntypedFuncs {
	return ApplyControlUntypedFuncs{
		GetCachedFunc: func(name string) (kubeinterfaces.ObjectInterface, error) {
			return acf.GetCached(name)
		},
		CreateFunc: func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error) {
			return acf.Create(ctx, obj.(T), opts)
		},
		UpdateFunc: func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error) {
			return acf.Update(ctx, obj.(T), opts)
		},
		DeleteFunc: acf.DeleteFunc,
	}
}

var _ ApplyControlInterface[*corev1.Service] = ApplyControlFuncs[*corev1.Service]{}

func TypeApplyControlInterface[T kubeinterfaces.ObjectInterface](untyped ApplyControlUntypedInterface) ApplyControlInterface[T] {
	return ApplyControlFuncs[T]{
		GetCachedFunc: func(name string) (T, error) {
			res, err := untyped.GetCached(name)
			if res == nil {
				return *new(T), err
			}
			return res.(T), err
		},
		CreateFunc: func(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
			res, err := untyped.Create(ctx, obj, opts)
			if res == nil {
				return *new(T), err
			}
			return res.(T), err
		},
		UpdateFunc: func(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
			res, err := untyped.Update(ctx, obj, opts)
			if res == nil {
				return *new(T), err
			}
			return res.(T), err
		},
		DeleteFunc: func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
			return untyped.Delete(ctx, name, opts)
		},
	}
}

type ApplyOptions struct {
	ForceOwnership            bool
	AllowMissingControllerRef bool
	DeletePropagationPolicy   *metav1.DeletionPropagation
}

func ApplyGenericWithHandlers[T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	control ApplyControlInterface[T],
	recorder record.EventRecorder,
	required T,
	options ApplyOptions,
	projectFunc func(required *T, existing T),
	getRecreateReasonFunc func(required T, existing T) string,
) (T, bool, error) {
	gvk := resource.GetObjectGVKOrUnknown(required)

	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if !options.AllowMissingControllerRef && requiredControllerRef == nil {
		return *new(T), false, fmt.Errorf("%s %q is missing controllerRef", gvk, naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopyObject().(T)
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return *new(T), false, err
	}

	createOptions := metav1.CreateOptions{
		FieldValidation: metav1.FieldValidationStrict,
	}

	existing, err := control.GetCached(requiredCopy.GetName())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return *new(T), false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := control.Create(ctx, requiredCopy, createOptions)
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Service", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, err == nil, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)

	existingControllerRefUID := types.UID("")
	if existingControllerRef != nil {
		existingControllerRefUID = existingControllerRef.UID
	}
	requiredControllerRefUID := types.UID("")
	if requiredControllerRef != nil {
		requiredControllerRefUID = requiredControllerRef.UID
	}

	if existingControllerRef == nil && requiredControllerRef != nil && options.ForceOwnership {
		klog.V(2).InfoS("Forcing apply to claim the the object", "GVK", gvk, "Ref", naming.ObjRef(requiredCopy))
	} else if existingControllerRefUID != requiredControllerRefUID {
		// This is not the place to handle adoption.
		err := fmt.Errorf("%s %q isn't controlled by us", gvk, naming.ObjRef(requiredCopy))
		ReportUpdateEvent(recorder, requiredCopy, err)
		return *new(T), false, err
	}

	existingHash := existing.GetAnnotations()[naming.ManagedHash]
	requiredHash := requiredCopy.GetAnnotations()[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	resourcemerge.MergeMetadataInPlace(requiredCopy, existing)

	// Project allocated fields, like spec.clusterIP for services.
	if projectFunc != nil {
		projectFunc(&requiredCopy, existing)
	}

	var recreateReason string
	if getRecreateReasonFunc != nil {
		recreateReason = getRecreateReasonFunc(requiredCopy, existing)
	}
	if len(recreateReason) > 0 {
		klog.V(2).InfoS(
			"Apply needs to recreate the object",
			"Reason", recreateReason,
			"GVK", gvk,
			"Ref", naming.ObjRefWithUID(existing),
		)

		propagationPolicy := metav1.DeletePropagationBackground
		if options.DeletePropagationPolicy != nil {
			propagationPolicy = *options.DeletePropagationPolicy
		}
		err := control.Delete(ctx, existing.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		ReportDeleteEvent(recorder, existing, err)
		if err != nil {
			return *new(T), false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		created, err := control.Create(ctx, requiredCopy, createOptions)
		ReportCreateEvent(recorder, requiredCopy, err)
		if err != nil {
			return *new(T), false, err
		}

		return created, true, nil
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.GetResourceVersion()) == 0 {
		requiredCopy.SetResourceVersion(existing.GetResourceVersion())
	}

	actual, err := control.Update(
		ctx,
		requiredCopy,
		metav1.UpdateOptions{
			FieldValidation: metav1.FieldValidationStrict,
		},
	)
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Service", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return *new(T), false, fmt.Errorf("can't update %s %q: %w", gvk, naming.ObjRef(requiredCopy), err)
	}

	return actual, true, nil
}

func ApplyGeneric[T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	control ApplyControlInterface[T],
	recorder record.EventRecorder,
	required T,
	options ApplyOptions,
) (T, bool, error) {
	return ApplyGenericWithHandlers[T](ctx, control, recorder, required, options, nil, nil)
}
