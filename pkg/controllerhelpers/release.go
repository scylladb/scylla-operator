// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func ReleaseObjects[CT, T kubeinterfaces.ObjectInterface](ctx context.Context, controller metav1.Object, controllerGVK schema.GroupVersionKind, selector labels.Selector, control ControlleeManagerGetObjectsFuncs[CT, T]) error {
	matchingObjects, err := control.ListObjects(selector)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range matchingObjects {
		controllerRef := metav1.GetControllerOfNoCopy(obj)
		if controllerRef == nil {
			// Not owned, nothing to do.
			continue
		}

		if controllerRef.UID != controller.GetUID() {
			// Owned by someone else, ignore.
			continue
		}

		gvk := resource.GetObjectGVKOrUnknown(obj)
		klog.V(2).InfoS("Removing controllerRef", "GVK", gvk, "Object", klog.KObj(obj), "ControllerRef", fmt.Sprintf("%s/%s:%s", controllerGVK.GroupVersion(), controllerGVK.Kind, controller.GetName()))

		patchBytes, err := GetDeleteOwnerReferenceMergePatchBytes(obj, controller.GetUID())
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get delete ownere reference merge patch bytes: %w", err))
		}

		_, err = control.PatchObject(ctx, obj.GetName(), types.MergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				klog.V(4).InfoS("Couldn't patch %s as it was missing", gvk, klog.KObj(obj))
				continue
			}

			errs = append(errs, fmt.Errorf("can't delete controller ref: %w", err))
			continue
		}
	}

	return nil
}

func GetDeleteOwnerReferenceMergePatchBytes(obj metav1.Object, controllerUID types.UID) ([]byte, error) {
	newOwnerRefs := slices.FilterOut(obj.GetOwnerReferences(), func(ownerRef metav1.OwnerReference) bool {
		return ownerRef.UID == controllerUID
	})

	patchBytes, err := json.Marshal(&objectForOwnerReferencePatch{
		objectMetaForOwnerReferencePatch: objectMetaForOwnerReferencePatch{
			UID:             obj.GetUID(),
			ResourceVersion: obj.GetResourceVersion(),
			OwnerReferences: newOwnerRefs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("can't marshal owner reference patch: %w", err)
	}

	return patchBytes, nil
}

type objectMetaForOwnerReferencePatch struct {
	UID             types.UID               `json:"uid"`
	ResourceVersion string                  `json:"resourceVersion"`
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences"`
}

type objectForOwnerReferencePatch struct {
	objectMetaForOwnerReferencePatch `json:"metadata"`
}
