// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type objectForFinalizersPatch struct {
	objectMetaForFinalizersPatch `json:"metadata"`
}

// objectMetaForFinalizersPatch defines object meta struct for finalizers patch operation.
type objectMetaForFinalizersPatch struct {
	ResourceVersion string   `json:"resourceVersion"`
	Finalizers      []string `json:"finalizers"`
}

func RemoveFinalizerPatch(obj metav1.Object, finalizer string) ([]byte, error) {
	if !HasFinalizer(obj, finalizer) {
		return nil, nil
	}

	finalizers := obj.GetFinalizers()
	var newFinalizers []string

	for _, f := range finalizers {
		if f == finalizer {
			continue
		}
		newFinalizers = append(newFinalizers, f)
	}

	patch, err := json.Marshal(&objectForFinalizersPatch{
		objectMetaForFinalizersPatch: objectMetaForFinalizersPatch{
			ResourceVersion: obj.GetResourceVersion(),
			Finalizers:      newFinalizers,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("can't marshal finalizer remove patch: %w", err)
	}

	return patch, nil
}

func AddFinalizerPatch(obj metav1.Object, finalizer string) ([]byte, error) {
	if HasFinalizer(obj, finalizer) {
		return nil, nil
	}
	newFinalizers := make([]string, 0, len(obj.GetFinalizers())+1)
	for _, f := range obj.GetFinalizers() {
		newFinalizers = append(newFinalizers, f)
	}
	newFinalizers = append(newFinalizers, finalizer)

	patch, err := json.Marshal(&objectForFinalizersPatch{
		objectMetaForFinalizersPatch: objectMetaForFinalizersPatch{
			ResourceVersion: obj.GetResourceVersion(),
			Finalizers:      newFinalizers,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("can't marshal finalizer add patch: %w", err)
	}

	return patch, nil
}

func HasFinalizer(obj metav1.Object, finalizer string) bool {
	found := false
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			found = true
			break
		}
	}
	return found
}
