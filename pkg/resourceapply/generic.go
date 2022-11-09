package resourceapply

import (
	"context"
	"fmt"
	"reflect"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type DeepCopyableObject[T any] interface {
	runtime.Object
	metav1.Object
	DeepCopy() T
}

// ApplyGenericObject will apply the existing object to match the required object.
func ApplyGenericObject[T DeepCopyableObject[T]](
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	lister cache.GenericLister,
	recorder record.EventRecorder,
	required T,
) (T, bool, error) {

	t := reflect.TypeOf(required)
	value := reflect.ValueOf(required)
	if t.Kind() != reflect.Pointer || value.IsNil() {
		return *new(T), false, fmt.Errorf("ApplyGenericObject requires a non-nil pointer to required object, got %v", t)
	}

	typeName := t.String()

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return *new(T), false, err
	}

	requiredHash := requiredCopy.GetAnnotations()[naming.ManagedHash]
	existingRaw, err := lister.ByNamespace(requiredCopy.GetNamespace()).Get(requiredCopy.GetName())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return *new(T), false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)

		asUnstructured := &unstructured.Unstructured{}
		if err := scheme.Scheme.Convert(requiredCopy, asUnstructured, nil); err != nil {
			return *new(T), false, fmt.Errorf("cannot convert required object to unstructured form: %w", err)
		}

		actualRaw, err := client.Namespace(requiredCopy.GetNamespace()).Create(ctx, asUnstructured, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", typeName, klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		if err != nil {
			return *new(T), false, err
		}

		actualPtr := reflect.New(reflect.TypeOf(new(T)).Elem().Elem()).Interface()
		if err := scheme.Scheme.Convert(actualRaw, actualPtr, nil); err != nil {
			return *new(T), false, fmt.Errorf("cannot convert created object to %s from unstructured form: %w", typeName, err)
		}
		return actualPtr.(T), true, err
	}
	existingPtr := reflect.New(reflect.TypeOf(new(T)).Elem().Elem()).Interface()
	if err := scheme.Scheme.Convert(existingRaw, existingPtr, nil); err != nil {
		return *new(T), false, fmt.Errorf("cannot convert existing object %T to %T: %w", existingRaw, existingPtr, err)
	}

	existing := existingPtr.(T)
	existingHash := existing.GetAnnotations()[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	if requiredAnnotations := requiredCopy.GetAnnotations(); requiredAnnotations != nil {
		resourcemerge.MergeMapInPlaceWithoutRemovalKeys(&requiredAnnotations, existing.GetAnnotations())
		requiredCopy.SetAnnotations(requiredAnnotations)
	}

	if requiredLabels := requiredCopy.GetLabels(); requiredLabels != nil {
		resourcemerge.MergeMapInPlaceWithoutRemovalKeys(&requiredLabels, existing.GetLabels())
		requiredCopy.SetLabels(requiredLabels)
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.GetResourceVersion()) == 0 {
		requiredCopy.SetResourceVersion(existing.GetResourceVersion())
	}

	asUnstructured := &unstructured.Unstructured{}
	if err := scheme.Scheme.Convert(requiredCopy, asUnstructured, nil); err != nil {
		return *new(T), false, fmt.Errorf("cannot convert required object to unstructured form: %w", err)
	}

	actualRaw, err := client.Namespace(requiredCopy.GetNamespace()).Update(ctx, asUnstructured, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", typeName, klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return *new(T), false, fmt.Errorf("can't update %s: %w", typeName, err)
	}

	actual := reflect.New(reflect.TypeOf(new(T)).Elem().Elem()).Interface()
	err = scheme.Scheme.Convert(actualRaw, actual, nil)
	if err != nil {
		return *new(T), false, fmt.Errorf("cannot convert actual object to %s form unstructured form: %w", t.String(), err)
	}
	return actual.(T), true, nil
}

// ApplyGenericObjectNonNamespaced will apply the existing object to match the required object.
func ApplyGenericObjectNonNamespaced[T DeepCopyableObject[T]](
	ctx context.Context,
	client dynamic.ResourceInterface,
	lister cache.GenericNamespaceLister,
	recorder record.EventRecorder,
	required T,
) (T, bool, error) {

	t := reflect.TypeOf(required)
	value := reflect.ValueOf(required)
	if t.Kind() != reflect.Pointer || value.IsNil() {
		return *new(T), false, fmt.Errorf("ApplyGenericObject requires a non-nil pointer to required object, got %v", t)
	}

	typeName := t.String()

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return *new(T), false, err
	}

	requiredHash := requiredCopy.GetAnnotations()[naming.ManagedHash]
	existingRaw, err := lister.Get(requiredCopy.GetName())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return *new(T), false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)

		asUnstructured := &unstructured.Unstructured{}
		if err := scheme.Scheme.Convert(requiredCopy, asUnstructured, nil); err != nil {
			return *new(T), false, fmt.Errorf("cannot convert required object to unstructured form: %w", err)
		}

		actualRaw, err := client.Create(ctx, asUnstructured, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", typeName, klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		if err != nil {
			return *new(T), false, err
		}

		actualPtr := reflect.New(reflect.TypeOf(new(T)).Elem().Elem()).Interface()
		if err := scheme.Scheme.Convert(actualRaw, actualPtr, nil); err != nil {
			return *new(T), false, fmt.Errorf("cannot convert created object to %s from unstructured form: %w", typeName, err)
		}
		return actualPtr.(T), true, err
	}
	existingPtr := reflect.New(reflect.TypeOf(new(T)).Elem().Elem()).Interface()
	if err := scheme.Scheme.Convert(existingRaw, existingPtr, nil); err != nil {
		return *new(T), false, fmt.Errorf("cannot convert existing object %T to %T: %w", existingRaw, existingPtr, err)
	}

	existing := existingPtr.(T)
	existingHash := existing.GetAnnotations()[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	if requiredAnnotations := requiredCopy.GetAnnotations(); requiredAnnotations != nil {
		resourcemerge.MergeMapInPlaceWithoutRemovalKeys(&requiredAnnotations, existing.GetAnnotations())
		requiredCopy.SetAnnotations(requiredAnnotations)
	}

	if requiredLabels := requiredCopy.GetLabels(); requiredLabels != nil {
		resourcemerge.MergeMapInPlaceWithoutRemovalKeys(&requiredLabels, existing.GetLabels())
		requiredCopy.SetLabels(requiredLabels)
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.GetResourceVersion()) == 0 {
		requiredCopy.SetResourceVersion(existing.GetResourceVersion())
	}

	asUnstructured := &unstructured.Unstructured{}
	if err := scheme.Scheme.Convert(requiredCopy, asUnstructured, nil); err != nil {
		return *new(T), false, fmt.Errorf("cannot convert required object to unstructured form: %w", err)
	}

	actualRaw, err := client.Update(ctx, asUnstructured, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", typeName, klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return *new(T), false, fmt.Errorf("can't update %s: %w", typeName, err)
	}

	actual := reflect.New(reflect.TypeOf(new(T)).Elem().Elem()).Interface()
	err = scheme.Scheme.Convert(actualRaw, actual, nil)
	if err != nil {
		return *new(T), false, fmt.Errorf("cannot convert actual object to %s form unstructured form: %w", t.String(), err)
	}
	return actual.(T), true, nil
}
