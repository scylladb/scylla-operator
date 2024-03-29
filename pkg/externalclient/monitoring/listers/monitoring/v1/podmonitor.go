// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PodMonitorLister helps list PodMonitors.
// All objects returned here must be treated as read-only.
type PodMonitorLister interface {
	// List lists all PodMonitors in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.PodMonitor, err error)
	// PodMonitors returns an object that can list and get PodMonitors.
	PodMonitors(namespace string) PodMonitorNamespaceLister
	PodMonitorListerExpansion
}

// podMonitorLister implements the PodMonitorLister interface.
type podMonitorLister struct {
	indexer cache.Indexer
}

// NewPodMonitorLister returns a new PodMonitorLister.
func NewPodMonitorLister(indexer cache.Indexer) PodMonitorLister {
	return &podMonitorLister{indexer: indexer}
}

// List lists all PodMonitors in the indexer.
func (s *podMonitorLister) List(selector labels.Selector) (ret []*v1.PodMonitor, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.PodMonitor))
	})
	return ret, err
}

// PodMonitors returns an object that can list and get PodMonitors.
func (s *podMonitorLister) PodMonitors(namespace string) PodMonitorNamespaceLister {
	return podMonitorNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PodMonitorNamespaceLister helps list and get PodMonitors.
// All objects returned here must be treated as read-only.
type PodMonitorNamespaceLister interface {
	// List lists all PodMonitors in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.PodMonitor, err error)
	// Get retrieves the PodMonitor from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.PodMonitor, error)
	PodMonitorNamespaceListerExpansion
}

// podMonitorNamespaceLister implements the PodMonitorNamespaceLister
// interface.
type podMonitorNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PodMonitors in the indexer for a given namespace.
func (s podMonitorNamespaceLister) List(selector labels.Selector) (ret []*v1.PodMonitor, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.PodMonitor))
	})
	return ret, err
}

// Get retrieves the PodMonitor from the indexer for a given namespace and name.
func (s podMonitorNamespaceLister) Get(name string) (*v1.PodMonitor, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("podmonitor"), name)
	}
	return obj.(*v1.PodMonitor), nil
}
