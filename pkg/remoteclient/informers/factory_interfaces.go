package informers

import (
	"reflect"
	"time"

	"github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// GenericClusterInformer provides access to a shared generic unbound informer and indexer.
type GenericClusterInformer interface {
	Informer() UnboundInformer
	Indexer() lister.ClusterIndexer
}

// UnboundInformer is an interface of Informer not bound to any cluster.
// It allows for retrieving state of all registered clusters and registering actions for current and future registered clusters,
// as well as getting informer for particular cluster by their name.
type UnboundInformer interface {
	HasSynced() bool
	AddEventHandler(handler cache.ResourceEventHandler)

	Cluster(string) cache.SharedIndexInformer
}

// NewInformerFunc takes client.ClusterClientInterface, cluster and time.Duration to return a SharedIndexInformer.
type NewInformerFunc[CT any] func(client.ClusterClientInterface[CT], string, time.Duration, ClusterListWatch[CT], []cache.ResourceEventHandler) cache.SharedIndexInformer

// NewUnboundInformerFunc returns an UnboundInformer.
type NewUnboundInformerFunc func() UnboundInformer

// SharedInformerFactory  provides shared informers for resources in all known API group versions for given client CT.
type SharedInformerFactory[CT any] interface {
	Start(stopCh <-chan struct{})
	InformerFor(objType reflect.Type, cluster string, newFunc NewInformerFunc[CT], lw ClusterListWatch[CT]) cache.SharedIndexInformer
	UnboundInformerFor(objType reflect.Type, newFunc NewUnboundInformerFunc) UnboundInformer
	AddEventHandlerFor(objType reflect.Type, handler cache.ResourceEventHandler)
	HasSyncedFor(objType reflect.Type) bool
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
	ForResource(obj runtime.Object, lw ClusterListWatch[CT]) GenericClusterInformer
}

type DynamicSharedInformerFactory[CT any] interface {
	client.DynamicClusterInterface
	SharedInformerFactory[CT]
}

// TweakListOptionsFunc is a function that transforms a metav1.ListOptions.
type TweakListOptionsFunc func(*metav1.ListOptions)

type ClusterListWatch[CT any] struct {
	ListFunc  func(client client.ClusterClientInterface[CT], cluster, ns string) cache.ListFunc
	WatchFunc func(client client.ClusterClientInterface[CT], cluster, ns string) cache.WatchFunc
}
