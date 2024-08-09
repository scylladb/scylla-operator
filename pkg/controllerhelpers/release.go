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
			// Not owned by anyone. Ignore.
			continue
		}

		if controllerRef.UID != controller.GetUID() {
			// Owned by someone else. Ignore.
			continue
		}

		gvk := resource.GetObjectGVKOrUnknown(obj)

		klog.V(2).InfoS("Removing controllerRef",
			"GVK", gvk,
			"Object", klog.KObj(obj),
			"ControllerRef", fmt.Sprintf("%s/%s:%s", controllerGVK.GroupVersion(), controllerGVK.Kind, controller.GetName()),
		)

		newOwnerRefs := slices.FilterOut(obj.GetOwnerReferences(), func(ownerRef metav1.OwnerReference) bool {
			return ownerRef.UID == controller.GetUID()
		})

		patchBytes, err := json.Marshal(&objectForOwnerReferencePatch{
			objectMetaForOwnerReferencePatch: objectMetaForOwnerReferencePatch{
				ResourceVersion: obj.GetResourceVersion(),
				OwnerReferences: newOwnerRefs,
			},
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't marshal owner reference patch: %w", err))
			continue
		}

		_, err = control.PatchObject(ctx, obj.GetName(), types.MergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// If the object no longer exists, ignore it.
				klog.V(4).InfoS("Couldn't patch %s as it was missing", gvk, klog.KObj(obj))
				continue
			}

			if errors.IsInvalid(err) {
				// Invalid error will be returned in two cases:
				// 1. the Service has no owner reference
				// 2. the UID of the Service doesn't match because it was recreated
				// In both cases, the error can be ignored.
				klog.V(4).InfoS("Couldn't patch %s as it was invalid", gvk, klog.KObj(obj))
				continue
			}

			errs = append(errs, fmt.Errorf("can't delete controller ref: %w", err))
			continue
		}
	}

	return nil
}

type objectMetaForOwnerReferencePatch struct {
	ResourceVersion string                  `json:"resourceVersion"`
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences"`
}

type objectForOwnerReferencePatch struct {
	objectMetaForOwnerReferencePatch `json:"metadata"`
}
