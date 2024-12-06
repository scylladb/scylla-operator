package informers

import (
	"reflect"
	"time"

	"github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type genericClusterInformer[CT any] struct {
	factory          SharedInformerFactory[CT]
	tweakListOptions TweakListOptionsFunc
	namespace        string
	obj              runtime.Object
	objType          reflect.Type
	lw               ClusterListWatch[CT]
}

// Informer returns an unbound informer.
func (gri *genericClusterInformer[CT]) Informer() UnboundInformer {
	return gri.factory.UnboundInformerFor(gri.objType, func() UnboundInformer {
		return &genericUnboundInformer[CT]{
			factory:          gri.factory,
			tweakListOptions: gri.tweakListOptions,
			obj:              gri.obj,
			objType:          gri.objType,
			lw:               gri.lw,
		}
	})
}

func (gri *genericClusterInformer[CT]) Indexer() lister.ClusterIndexer {
	return lister.NewGenericClusterIndexer(gri.Informer().Cluster)
}

type genericUnboundInformer[CT any] struct {
	factory          SharedInformerFactory[CT]
	tweakListOptions TweakListOptionsFunc
	namespace        string
	lw               ClusterListWatch[CT]
	obj              runtime.Object
	objType          reflect.Type
}

func (gui *genericUnboundInformer[CT]) defaultInformer(client client.ClusterClientInterface[CT], cluster string, resyncPeriod time.Duration, lw ClusterListWatch[CT], eventHandlers []cache.ResourceEventHandler) cache.SharedIndexInformer {
	informer := NewFilteredInformer(client, cluster, gui.namespace, gui.obj, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, lw, gui.tweakListOptions)
	for _, eventHandler := range eventHandlers {
		informer.AddEventHandler(eventHandler)
	}
	return informer
}

func (gui *genericUnboundInformer[CT]) Cluster(cluster string) cache.SharedIndexInformer {
	return gui.factory.InformerFor(gui.objType, cluster, gui.defaultInformer, gui.lw)
}

func (gui *genericUnboundInformer[CT]) HasSynced() bool {
	return gui.factory.HasSyncedFor(gui.objType)
}

func (gui *genericUnboundInformer[CT]) AddEventHandler(handler cache.ResourceEventHandler) {
	gui.factory.AddEventHandlerFor(gui.objType, handler)
}
