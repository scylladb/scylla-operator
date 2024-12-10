// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeScyllaV1alpha1 struct {
	*testing.Fake
}

func (c *FakeScyllaV1alpha1) NodeConfigs() v1alpha1.NodeConfigInterface {
	return &FakeNodeConfigs{c}
}

func (c *FakeScyllaV1alpha1) RemoteKubernetesClusters() v1alpha1.RemoteKubernetesClusterInterface {
	return &FakeRemoteKubernetesClusters{c}
}

func (c *FakeScyllaV1alpha1) RemoteOwners(namespace string) v1alpha1.RemoteOwnerInterface {
	return &FakeRemoteOwners{c, namespace}
}

func (c *FakeScyllaV1alpha1) ScyllaDBClusters(namespace string) v1alpha1.ScyllaDBClusterInterface {
	return &FakeScyllaDBClusters{c, namespace}
}

func (c *FakeScyllaV1alpha1) ScyllaDBDatacenters(namespace string) v1alpha1.ScyllaDBDatacenterInterface {
	return &FakeScyllaDBDatacenters{c, namespace}
}

func (c *FakeScyllaV1alpha1) ScyllaDBMonitorings(namespace string) v1alpha1.ScyllaDBMonitoringInterface {
	return &FakeScyllaDBMonitorings{c, namespace}
}

func (c *FakeScyllaV1alpha1) ScyllaOperatorConfigs() v1alpha1.ScyllaOperatorConfigInterface {
	return &FakeScyllaOperatorConfigs{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeScyllaV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
