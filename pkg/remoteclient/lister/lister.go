package lister

import (
	"k8s.io/client-go/tools/cache"
)

type ClusterIndexer interface {
	Cluster(string) cache.Indexer
}

type clusterIndexer struct {
	makeInformer func(string) cache.SharedIndexInformer
}

func NewGenericClusterIndexer(makeInformer func(string) cache.SharedIndexInformer) ClusterIndexer {
	return &clusterIndexer{makeInformer: makeInformer}
}

func (grl *clusterIndexer) Cluster(cluster string) cache.Indexer {
	return grl.makeInformer(cluster).GetIndexer()
}

type GenericClusterLister[T any] interface {
	Cluster(string) T
}

type genericClusterLister[T any] struct {
	factory        func(cache.Indexer) T
	clusterIndexer func(string) cache.Indexer
}

func NewClusterLister[T any](factory func(cache.Indexer) T, clusterIndexer func(string) cache.Indexer) GenericClusterLister[T] {
	return &genericClusterLister[T]{factory: factory, clusterIndexer: clusterIndexer}
}

func (l *genericClusterLister[T]) Cluster(name string) T {
	return l.factory(l.clusterIndexer(name))
}
