// Code generated by informer-gen. DO NOT EDIT.

package internalinterfaces

import (
	time "time"

	versioned "github.com/scylladb/scylla-operator/pkg/multiregionclient/scylla/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	cache "k8s.io/client-go/tools/cache"
)

type UnboundInformer interface {
	HasSynced() bool
	AddEventHandler(handler cache.ResourceEventHandler)

	Datacenter(string) cache.SharedIndexInformer
}

// NewInformerFunc takes versioned.Interface and time.Duration to return a SharedIndexInformer.
type NewInformerFunc func(versioned.RemoteInterface, string, time.Duration, []cache.ResourceEventHandler) cache.SharedIndexInformer

// NewUnboundInformerFunc takes versioned.Interface and time.Duration to return a SharedIndexInformer.
type NewUnboundInformerFunc func() UnboundInformer

// SharedInformerFactory a small interface to allow for adding an informer without an import cycle
type SharedInformerFactory interface {
	Start(stopCh <-chan struct{})
	InformerFor(obj runtime.Object, datacenter string, newFunc NewInformerFunc) cache.SharedIndexInformer
	UnboundInformerFor(obj runtime.Object, newFunc NewUnboundInformerFunc) UnboundInformer
	AddEventHandlerFor(obj runtime.Object, handler cache.ResourceEventHandler)
	HasSyncedFor(obj runtime.Object) bool
}

// TweakListOptionsFunc is a function that transforms a v1.ListOptions.
type TweakListOptionsFunc func(*v1.ListOptions)
