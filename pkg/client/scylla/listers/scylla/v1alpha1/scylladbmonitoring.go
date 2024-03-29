// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ScyllaDBMonitoringLister helps list ScyllaDBMonitorings.
// All objects returned here must be treated as read-only.
type ScyllaDBMonitoringLister interface {
	// List lists all ScyllaDBMonitorings in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ScyllaDBMonitoring, err error)
	// ScyllaDBMonitorings returns an object that can list and get ScyllaDBMonitorings.
	ScyllaDBMonitorings(namespace string) ScyllaDBMonitoringNamespaceLister
	ScyllaDBMonitoringListerExpansion
}

// scyllaDBMonitoringLister implements the ScyllaDBMonitoringLister interface.
type scyllaDBMonitoringLister struct {
	indexer cache.Indexer
}

// NewScyllaDBMonitoringLister returns a new ScyllaDBMonitoringLister.
func NewScyllaDBMonitoringLister(indexer cache.Indexer) ScyllaDBMonitoringLister {
	return &scyllaDBMonitoringLister{indexer: indexer}
}

// List lists all ScyllaDBMonitorings in the indexer.
func (s *scyllaDBMonitoringLister) List(selector labels.Selector) (ret []*v1alpha1.ScyllaDBMonitoring, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ScyllaDBMonitoring))
	})
	return ret, err
}

// ScyllaDBMonitorings returns an object that can list and get ScyllaDBMonitorings.
func (s *scyllaDBMonitoringLister) ScyllaDBMonitorings(namespace string) ScyllaDBMonitoringNamespaceLister {
	return scyllaDBMonitoringNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ScyllaDBMonitoringNamespaceLister helps list and get ScyllaDBMonitorings.
// All objects returned here must be treated as read-only.
type ScyllaDBMonitoringNamespaceLister interface {
	// List lists all ScyllaDBMonitorings in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ScyllaDBMonitoring, err error)
	// Get retrieves the ScyllaDBMonitoring from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ScyllaDBMonitoring, error)
	ScyllaDBMonitoringNamespaceListerExpansion
}

// scyllaDBMonitoringNamespaceLister implements the ScyllaDBMonitoringNamespaceLister
// interface.
type scyllaDBMonitoringNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all ScyllaDBMonitorings in the indexer for a given namespace.
func (s scyllaDBMonitoringNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.ScyllaDBMonitoring, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ScyllaDBMonitoring))
	})
	return ret, err
}

// Get retrieves the ScyllaDBMonitoring from the indexer for a given namespace and name.
func (s scyllaDBMonitoringNamespaceLister) Get(name string) (*v1alpha1.ScyllaDBMonitoring, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("scylladbmonitoring"), name)
	}
	return obj.(*v1alpha1.ScyllaDBMonitoring), nil
}
