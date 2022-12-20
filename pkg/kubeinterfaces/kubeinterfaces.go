package kubeinterfaces

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type ObjectInterface interface {
	runtime.Object
	metav1.Object
}

type GetterLister[T any] interface {
	Get(namespace, name string) (T, error)
	List(namespace string, selector labels.Selector) ([]T, error)
}

type GlobalGetList[T any] struct {
	GetFunc  func(name string) (T, error)
	ListFunc func(selector labels.Selector) ([]T, error)
}

var _ GetterLister[any] = GlobalGetList[any]{}

func (gl GlobalGetList[T]) Get(defaultNamespace, name string) (T, error) {
	return gl.GetFunc(name)
}

func (gl GlobalGetList[T]) List(defaultNamespace string, selector labels.Selector) ([]T, error) {
	return gl.ListFunc(selector)
}

type NamespacedGetList[T any] struct {
	GetFunc  func(namespace, name string) (T, error)
	ListFunc func(namespace string, selector labels.Selector) ([]T, error)
}

var _ GetterLister[any] = NamespacedGetList[any]{}

func (gl NamespacedGetList[T]) Get(defaultNamespace, name string) (T, error) {
	return gl.GetFunc(defaultNamespace, name)
}

func (gl NamespacedGetList[T]) List(defaultNamespace string, selector labels.Selector) ([]T, error) {
	return gl.ListFunc(defaultNamespace, selector)
}
