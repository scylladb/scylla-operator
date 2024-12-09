// Copyright (c) 2024 ScyllaDB.

package informers

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// SharedInformerOption defines the functional option type for SharedInformerFactory.
type SharedInformerOption[CT any] func(*sharedInformerFactory[CT]) *sharedInformerFactory[CT]

// WithCustomResyncConfig sets a custom resync period for the specified informer types.
func WithCustomResyncConfig[CT any](resyncConfig map[reflect.Type]time.Duration) SharedInformerOption[CT] {
	return func(factory *sharedInformerFactory[CT]) *sharedInformerFactory[CT] {
		for k, v := range resyncConfig {
			factory.customResync[k] = v
		}
		return factory
	}
}

// WithTweakListOptions sets a custom filter on all listers of the configured SharedInformerFactory.
func WithTweakListOptions[CT any](tweakListOptions TweakListOptionsFunc) SharedInformerOption[CT] {
	return func(factory *sharedInformerFactory[CT]) *sharedInformerFactory[CT] {
		factory.tweakListOptions = tweakListOptions
		return factory
	}
}

// WithNamespace limits the SharedInformerFactory to the specified namespace.
func WithNamespace[CT any](namespace string) SharedInformerOption[CT] {
	return func(factory *sharedInformerFactory[CT]) *sharedInformerFactory[CT] {
		factory.namespace = namespace
		return factory
	}
}

// NewFilteredInformer constructs a new informer for obj type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredInformer[CT any](client client.ClusterClientInterface[CT], cluster string, namespace string, obj runtime.Object, resyncPeriod time.Duration, indexers cache.Indexers, lw ClusterListWatch[CT], tweakListOptions TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  lw.ListFunc(client, cluster, namespace),
			WatchFunc: lw.WatchFunc(client, cluster, namespace),
		},
		obj,
		resyncPeriod,
		indexers,
	)
}

// NewSharedInformerFactory constructs a new instance of sharedInformerFactory for all namespaces.
func NewSharedInformerFactory[CT any](client client.ClusterClientInterface[CT], defaultResync time.Duration) DynamicSharedInformerFactory[CT] {
	return NewSharedInformerFactoryWithOptions(client, defaultResync)
}

// NewSharedInformerFactoryWithOptions constructs a new instance of a SharedInformerFactory with additional options.
func NewSharedInformerFactoryWithOptions[CT any](client client.ClusterClientInterface[CT], defaultResync time.Duration, options ...SharedInformerOption[CT]) DynamicSharedInformerFactory[CT] {
	factory := &sharedInformerFactory[CT]{
		client:              client,
		namespace:           metav1.NamespaceAll,
		defaultResync:       defaultResync,
		customResync:        make(map[reflect.Type]time.Duration),
		informers:           make(map[reflect.Type]map[string]cache.SharedIndexInformer),
		startedInformers:    make(map[reflect.Type]map[string]chan struct{}),
		unboundInformers:    make(map[reflect.Type]UnboundInformer),
		eventHandlers:       make(map[reflect.Type][]cache.ResourceEventHandler),
		clusterConfigHashes: make(map[string]string),
	}

	for _, opt := range options {
		factory = opt(factory)
	}

	return factory
}

type sharedInformerFactory[CT any] struct {
	client           client.ClusterClientInterface[CT]
	namespace        string
	tweakListOptions TweakListOptionsFunc
	lock             sync.Mutex
	defaultResync    time.Duration
	customResync     map[reflect.Type]time.Duration

	informers           map[reflect.Type]map[string]cache.SharedIndexInformer
	startedInformers    map[reflect.Type]map[string]chan struct{}
	unboundInformers    map[reflect.Type]UnboundInformer
	eventHandlers       map[reflect.Type][]cache.ResourceEventHandler
	clusterConfigHashes map[string]string
	stopCh              <-chan struct{}
}

var _ SharedInformerFactory[any] = &sharedInformerFactory[any]{}

// Start initializes all requested informers.
func (sif *sharedInformerFactory[CT]) Start(stopCh <-chan struct{}) {
	sif.lock.Lock()
	defer sif.lock.Unlock()

	if sif.stopCh == nil {
		sif.stopCh = stopCh
	}
	for gvr, clusterInformers := range sif.informers {
		for cluster, informer := range clusterInformers {
			if _, started := sif.startedInformers[gvr][cluster]; !started {
				sc := make(chan struct{})
				go func() {
					<-stopCh
					close(sc)
				}()

				go informer.Run(sc)
				if _, ok := sif.startedInformers[gvr]; !ok {
					sif.startedInformers[gvr] = map[string]chan struct{}{}
				}
				sif.startedInformers[gvr][cluster] = sc
			}
		}
	}
}

// InformerFor returns the SharedIndexInformer for obj using an internal client.
func (sif *sharedInformerFactory[CT]) InformerFor(objType reflect.Type, cluster string, newFunc NewInformerFunc[CT], lw ClusterListWatch[CT]) cache.SharedIndexInformer {
	sif.lock.Lock()
	defer sif.lock.Unlock()

	clusterInformers, registered := sif.informers[objType]
	if registered {
		informer, exists := clusterInformers[cluster]
		if exists {
			return informer
		}
	}

	resyncPeriod, exists := sif.customResync[objType]
	if !exists {
		resyncPeriod = sif.defaultResync
	}

	informer := newFunc(sif.client, cluster, resyncPeriod, lw, sif.eventHandlers[objType])
	if _, ok := sif.informers[objType]; !ok {
		sif.informers[objType] = map[string]cache.SharedIndexInformer{}
	}
	sif.informers[objType][cluster] = informer

	if sif.stopCh != nil {
		sc := make(chan struct{})
		go func() {
			<-sif.stopCh
			select {
			case <-sc:
			default:
				close(sc)
			}
		}()

		go informer.Run(sc)
		if _, ok := sif.startedInformers[objType]; !ok {
			sif.startedInformers[objType] = map[string]chan struct{}{}
		}
		sif.startedInformers[objType][cluster] = sc
	}

	return informer
}

// UnboundInformerFor returns an UnboundInformer of given objType.
func (sif *sharedInformerFactory[CT]) UnboundInformerFor(objType reflect.Type, newFunc NewUnboundInformerFunc) UnboundInformer {
	sif.lock.Lock()
	defer sif.lock.Unlock()

	informer, exists := sif.unboundInformers[objType]
	if exists {
		return informer
	}

	informer = newFunc()
	sif.unboundInformers[objType] = informer

	return informer
}

// AddEventHandlerFor registers an event handler for given objType.
func (sif *sharedInformerFactory[CT]) AddEventHandlerFor(objType reflect.Type, handler cache.ResourceEventHandler) {
	sif.lock.Lock()
	defer sif.lock.Unlock()

	sif.eventHandlers[objType] = append(sif.eventHandlers[objType], handler)
}

// HasSyncedFor returns whether informer caches are synced for given objType in all registered clusters.
func (sif *sharedInformerFactory[CT]) HasSyncedFor(objType reflect.Type) bool {
	sif.lock.Lock()
	defer sif.lock.Unlock()

	for _, informer := range sif.informers[objType] {
		if !informer.HasSynced() {
			return false
		}
	}
	return true
}

// WaitForCacheSync waits for all started informers' cache were synced.
func (sif *sharedInformerFactory[CT]) WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool {
	resourceInformers := func() map[reflect.Type][]cache.SharedIndexInformer {
		sif.lock.Lock()
		defer sif.lock.Unlock()

		informers := map[reflect.Type][]cache.SharedIndexInformer{}
		for informerType, clusterInformers := range sif.informers {
			if _, started := sif.startedInformers[informerType]; started {
				for _, informer := range clusterInformers {
					informers[informerType] = append(informers[informerType], informer)
				}

			}
		}
		return informers
	}()

	res := map[reflect.Type]bool{}
	for informerType, informers := range resourceInformers {
		synced := true
		for _, informer := range informers {
			if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
				synced = false
				break
			}
		}
		res[informerType] = synced
	}
	return res
}

func (sif *sharedInformerFactory[CT]) ForResource(obj runtime.Object, lw ClusterListWatch[CT]) GenericClusterInformer {
	return &genericClusterInformer[CT]{
		factory:          sif,
		tweakListOptions: sif.tweakListOptions,
		namespace:        sif.namespace,
		obj:              obj,
		objType:          reflect.TypeOf(obj),
		lw:               lw,
	}
}

var _ client.DynamicClusterInterface = &sharedInformerFactory[any]{}

func (sif *sharedInformerFactory[CT]) UpdateCluster(cluster string, config []byte) error {
	unboundInformers, err := func() ([]UnboundInformer, error) {
		sif.lock.Lock()
		defer sif.lock.Unlock()

		configHash, err := hash.HashBytes(config)
		if err != nil {
			return nil, fmt.Errorf("can't hash config bytes: %w", err)
		}
		if credentialsHash, found := sif.clusterConfigHashes[cluster]; found && credentialsHash == configHash {
			return nil, nil
		}

		sif.clusterConfigHashes[cluster] = configHash

		for informerType, clusterInformers := range sif.startedInformers {
			if stopCh, started := clusterInformers[cluster]; started {
				select {
				case <-stopCh:
				default:
					close(stopCh)
				}
			}
			delete(sif.startedInformers[informerType], cluster)
		}

		for informerType := range sif.informers {
			delete(sif.informers[informerType], cluster)
		}

		unboundInformers := make([]UnboundInformer, 0, len(sif.unboundInformers))
		for i := range sif.unboundInformers {
			unboundInformers = append(unboundInformers, sif.unboundInformers[i])
		}
		return unboundInformers, nil
	}()
	if err != nil {
		return fmt.Errorf("can't build unbound informers for cluster %q: %w", cluster, err)
	}

	for _, unboundInformer := range unboundInformers {
		unboundInformer.Cluster(cluster)
	}

	return nil
}

func (sif *sharedInformerFactory[CT]) DeleteCluster(cluster string) {
	sif.lock.Lock()
	defer sif.lock.Unlock()

	for informerType, clusterInformers := range sif.startedInformers {
		if stopCh, started := clusterInformers[cluster]; started {
			select {
			case <-stopCh:
			default:
				close(stopCh)
			}
		}
		delete(sif.startedInformers[informerType], cluster)
		delete(sif.informers[informerType], cluster)
	}

	delete(sif.clusterConfigHashes, cluster)
}
