// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// ThanosRulerLister helps list ThanosRulers.
// All objects returned here must be treated as read-only.
type ThanosRulerLister interface {
	// List lists all ThanosRulers in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*monitoringv1.ThanosRuler, err error)
	// ThanosRulers returns an object that can list and get ThanosRulers.
	ThanosRulers(namespace string) ThanosRulerNamespaceLister
	ThanosRulerListerExpansion
}

// thanosRulerLister implements the ThanosRulerLister interface.
type thanosRulerLister struct {
	listers.ResourceIndexer[*monitoringv1.ThanosRuler]
}

// NewThanosRulerLister returns a new ThanosRulerLister.
func NewThanosRulerLister(indexer cache.Indexer) ThanosRulerLister {
	return &thanosRulerLister{listers.New[*monitoringv1.ThanosRuler](indexer, monitoringv1.Resource("thanosruler"))}
}

// ThanosRulers returns an object that can list and get ThanosRulers.
func (s *thanosRulerLister) ThanosRulers(namespace string) ThanosRulerNamespaceLister {
	return thanosRulerNamespaceLister{listers.NewNamespaced[*monitoringv1.ThanosRuler](s.ResourceIndexer, namespace)}
}

// ThanosRulerNamespaceLister helps list and get ThanosRulers.
// All objects returned here must be treated as read-only.
type ThanosRulerNamespaceLister interface {
	// List lists all ThanosRulers in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*monitoringv1.ThanosRuler, err error)
	// Get retrieves the ThanosRuler from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*monitoringv1.ThanosRuler, error)
	ThanosRulerNamespaceListerExpansion
}

// thanosRulerNamespaceLister implements the ThanosRulerNamespaceLister
// interface.
type thanosRulerNamespaceLister struct {
	listers.ResourceIndexer[*monitoringv1.ThanosRuler]
}
