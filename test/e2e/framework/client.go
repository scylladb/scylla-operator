// Copyright (C) 2024 ScyllaDB

package framework

import (
	o "github.com/onsi/gomega"
	promclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	scyllaclientset "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type GenericClientInterface interface {
	DiscoveryClient() *discovery.DiscoveryClient
}

type ClientInterface interface {
	GenericClientInterface
	ClientConfig() *restclient.Config
	KubeClient() *kubernetes.Clientset
	DynamicClient() dynamic.Interface
	ScyllaClient() *scyllaclientset.Clientset
}

type AdminClientInterface interface {
	GenericClientInterface
	AdminClientConfig() *restclient.Config
	KubeAdminClient() *kubernetes.Clientset
	DynamicAdminClient() dynamic.Interface
	ScyllaAdminClient() *scyllaclientset.Clientset
	PrometheusOperatorAdminClient() *promclient.Clientset
}

type FullClientInterface interface {
	ClientInterface
	AdminClientInterface
}

type Client struct {
	Config *restclient.Config
}

var _ GenericClientInterface = &Client{}
var _ ClientInterface = &Client{}

func (c *Client) ClientConfig() *restclient.Config {
	o.Expect(c.Config).NotTo(o.BeNil())
	return c.Config
}

func (c *Client) DynamicClient() dynamic.Interface {
	dc, err := dynamic.NewForConfig(c.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return dc
}

func (c *Client) KubeClient() *kubernetes.Clientset {
	cs, err := kubernetes.NewForConfig(c.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cs
}

func (c *Client) ScyllaClient() *scyllaclientset.Clientset {
	cs, err := scyllaclientset.NewForConfig(c.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cs
}

func (c *Client) DiscoveryClient() *discovery.DiscoveryClient {
	client, err := discovery.NewDiscoveryClientForConfig(c.ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

type AdminClient struct {
	Config *restclient.Config
}

var _ AdminClientInterface = &AdminClient{}

func (ac *AdminClient) AdminClientConfig() *restclient.Config {
	o.Expect(ac.Config).NotTo(o.BeNil())
	return ac.Config
}

func (ac *AdminClient) DynamicAdminClient() dynamic.Interface {
	dc, err := dynamic.NewForConfig(ac.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return dc
}

func (ac *AdminClient) KubeAdminClient() *kubernetes.Clientset {
	cs, err := kubernetes.NewForConfig(ac.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cs
}

func (ac *AdminClient) ScyllaAdminClient() *scyllaclientset.Clientset {
	cs, err := scyllaclientset.NewForConfig(ac.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cs
}

func (ac *AdminClient) DiscoveryClient() *discovery.DiscoveryClient {
	client, err := discovery.NewDiscoveryClientForConfig(ac.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func (ac *AdminClient) PrometheusOperatorAdminClient() *promclient.Clientset {
	cs, err := promclient.NewForConfig(ac.AdminClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cs
}

type FullClient struct {
	Client
	AdminClient
}

var _ FullClientInterface = &FullClient{}

func (c *FullClient) DiscoveryClient() *discovery.DiscoveryClient {
	return c.Client.DiscoveryClient()
}
