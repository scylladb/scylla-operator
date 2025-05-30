package resource

import (
	"errors"
	"reflect"

	"github.com/scylladb/scylla-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var Scheme = scheme.Scheme

func GetObjectGVK(object runtime.Object) (*schema.GroupVersionKind, error) {
	gvk := object.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) > 0 {
		return &gvk, nil
	}

	kinds, _, err := Scheme.ObjectKinds(object)
	if err != nil {
		return nil, err
	}
	if len(kinds) == 0 {
		return nil, errors.New("no kind found")
	}

	return &kinds[0], nil
}

func GetObjectGVKOrUnknown(obj runtime.Object) *schema.GroupVersionKind {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) > 0 {
		return &gvk
	}

	kinds, _, err := Scheme.ObjectKinds(obj)
	if err != nil || len(kinds) == 0 {
		t := reflect.TypeOf(obj)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		return &schema.GroupVersionKind{
			Group:   "unknown",
			Version: "unknown",
			Kind:    t.Name(),
		}
	}

	return &kinds[0]
}
