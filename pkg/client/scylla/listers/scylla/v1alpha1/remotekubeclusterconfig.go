// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// RemoteKubeClusterConfigLister helps list RemoteKubeClusterConfigs.
// All objects returned here must be treated as read-only.
type RemoteKubeClusterConfigLister interface {
	// List lists all RemoteKubeClusterConfigs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.RemoteKubeClusterConfig, err error)
	// Get retrieves the RemoteKubeClusterConfig from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.RemoteKubeClusterConfig, error)
	RemoteKubeClusterConfigListerExpansion
}

// remoteKubeClusterConfigLister implements the RemoteKubeClusterConfigLister interface.
type remoteKubeClusterConfigLister struct {
	indexer cache.Indexer
}

// NewRemoteKubeClusterConfigLister returns a new RemoteKubeClusterConfigLister.
func NewRemoteKubeClusterConfigLister(indexer cache.Indexer) RemoteKubeClusterConfigLister {
	return &remoteKubeClusterConfigLister{indexer: indexer}
}

// List lists all RemoteKubeClusterConfigs in the indexer.
func (s *remoteKubeClusterConfigLister) List(selector labels.Selector) (ret []*v1alpha1.RemoteKubeClusterConfig, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.RemoteKubeClusterConfig))
	})
	return ret, err
}

// Get retrieves the RemoteKubeClusterConfig from the index for a given name.
func (s *remoteKubeClusterConfigLister) Get(name string) (*v1alpha1.RemoteKubeClusterConfig, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("remotekubeclusterconfig"), name)
	}
	return obj.(*v1alpha1.RemoteKubeClusterConfig), nil
}
