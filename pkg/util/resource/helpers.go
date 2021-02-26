// Copyright (C) 2021 ScyllaDB

package resource

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

func GetObjectGVK(object runtime.Object) (*schema.GroupVersionKind, error) {
	gvk := object.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) > 0 {
		return &gvk, nil
	}

	kinds, _, err := scheme.Scheme.ObjectKinds(object)
	if err != nil {
		return nil, err
	}
	if len(kinds) == 0 {
		return nil, errors.New("no kind found")
	}

	return &kinds[0], nil
}
