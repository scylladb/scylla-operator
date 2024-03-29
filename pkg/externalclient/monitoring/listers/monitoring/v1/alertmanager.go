// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// AlertmanagerLister helps list Alertmanagers.
// All objects returned here must be treated as read-only.
type AlertmanagerLister interface {
	// List lists all Alertmanagers in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.Alertmanager, err error)
	// Alertmanagers returns an object that can list and get Alertmanagers.
	Alertmanagers(namespace string) AlertmanagerNamespaceLister
	AlertmanagerListerExpansion
}

// alertmanagerLister implements the AlertmanagerLister interface.
type alertmanagerLister struct {
	indexer cache.Indexer
}

// NewAlertmanagerLister returns a new AlertmanagerLister.
func NewAlertmanagerLister(indexer cache.Indexer) AlertmanagerLister {
	return &alertmanagerLister{indexer: indexer}
}

// List lists all Alertmanagers in the indexer.
func (s *alertmanagerLister) List(selector labels.Selector) (ret []*v1.Alertmanager, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Alertmanager))
	})
	return ret, err
}

// Alertmanagers returns an object that can list and get Alertmanagers.
func (s *alertmanagerLister) Alertmanagers(namespace string) AlertmanagerNamespaceLister {
	return alertmanagerNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// AlertmanagerNamespaceLister helps list and get Alertmanagers.
// All objects returned here must be treated as read-only.
type AlertmanagerNamespaceLister interface {
	// List lists all Alertmanagers in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.Alertmanager, err error)
	// Get retrieves the Alertmanager from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.Alertmanager, error)
	AlertmanagerNamespaceListerExpansion
}

// alertmanagerNamespaceLister implements the AlertmanagerNamespaceLister
// interface.
type alertmanagerNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Alertmanagers in the indexer for a given namespace.
func (s alertmanagerNamespaceLister) List(selector labels.Selector) (ret []*v1.Alertmanager, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Alertmanager))
	})
	return ret, err
}

// Get retrieves the Alertmanager from the indexer for a given namespace and name.
func (s alertmanagerNamespaceLister) Get(name string) (*v1.Alertmanager, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("alertmanager"), name)
	}
	return obj.(*v1.Alertmanager), nil
}
