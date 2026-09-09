// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	"context"
	"fmt"

	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Informer exposes an informer owned by a controller-runtime cache through the two-method interface
// (Informer() and Lister()) that client-go's generated typed informers implement and that the controllers
// not yet migrated to controller-runtime take in their constructors. This is what lets the legacy controllers
// and the controller-runtime reconcilers share a single set of informers instead of running two caches.
type Informer[L any] struct {
	informer toolscache.SharedIndexInformer
	lister   L
}

func (i Informer[L]) Informer() toolscache.SharedIndexInformer {
	return i.informer
}

func (i Informer[L]) Lister() L {
	return i.lister
}

// InformerFor returns the shared informer for obj from the cache, wrapped together with a typed lister built
// over the informer's indexer. Requested before the cache is started, the informer is only registered and starts
// together with the cache, the same way informers requested from a client-go SharedInformerFactory do.
func InformerFor[L any](ctx context.Context, c cache.Cache, obj client.Object, newLister func(toolscache.Indexer) L) (Informer[L], error) {
	informer, err := c.GetInformer(ctx, obj)
	if err != nil {
		return Informer[L]{}, fmt.Errorf("can't get informer for %T: %w", obj, err)
	}

	// controller-runtime's cache is built on client-go shared index informers and hands out the concrete
	// informer; the typed listers need its indexer.
	sharedIndexInformer, ok := informer.(toolscache.SharedIndexInformer)
	if !ok {
		return Informer[L]{}, fmt.Errorf("informer for %T is %T, not a client-go SharedIndexInformer", obj, informer)
	}

	return Informer[L]{
		informer: sharedIndexInformer,
		lister:   newLister(sharedIndexInformer.GetIndexer()),
	}, nil
}

// informerFor is InformerFor with the error left for errs to report, so that wiring code can request every informer
// up front and check once.
func informerFor[L any](ctx context.Context, c cache.Cache, errs *errorCollector, obj client.Object, newLister func(toolscache.Indexer) L) Informer[L] {
	return collectError(errs, func() (Informer[L], error) {
		return InformerFor(ctx, c, obj, newLister)
	})
}
