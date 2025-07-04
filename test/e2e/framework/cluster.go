// Copyright (C) 2024 ScyllaDB

package framework

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type ClusterInterface interface {
	AdminClientInterface
	Name() string
	DefaultNamespaceIfAny() (*corev1.Namespace, Client, bool)
	CreateUserNamespace(ctx context.Context) (*corev1.Namespace, Client)
	AddCleaners(cleaners ...Cleaner)
	AddCollectors(collectors ...Collector)
	AddCleanerCollectors(cc ...CleanerCollector)
}

type createNamespaceFunc func(ctx context.Context, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client)

type Cluster struct {
	AdminClient

	name         string
	artifactsDir string

	createNamespace  createNamespaceFunc
	defaultNamespace *corev1.Namespace
	defaultClient    Client

	cleaners   []Cleaner
	collectors []Collector
}

var _ AdminClientInterface = &Cluster{}
var _ ClusterInterface = &Cluster{}

func NewCluster(name string, artifactsDir string, restConfig *restclient.Config, createNamespace createNamespaceFunc) *Cluster {
	adminClientConfig := restclient.CopyConfig(restConfig)

	return &Cluster{
		AdminClient: AdminClient{
			Config: adminClientConfig,
		},

		name:         name,
		artifactsDir: artifactsDir,

		createNamespace:  createNamespace,
		defaultNamespace: nil,
		defaultClient: Client{
			Config: nil,
		},
		cleaners:   nil,
		collectors: nil,
	}
}

func (c *Cluster) Name() string {
	return c.name
}

func (c *Cluster) DefaultNamespaceIfAny() (*corev1.Namespace, Client, bool) {
	if c.defaultNamespace == nil {
		return nil, Client{}, false
	}

	return c.defaultNamespace, c.defaultClient, true
}

func (c *Cluster) AddCleaners(cleaners ...Cleaner) {
	c.cleaners = append(c.cleaners, cleaners...)
}

func (c *Cluster) AddCollectors(collectors ...Collector) {
	c.collectors = append(c.collectors, collectors...)
}

func (c *Cluster) AddCleanerCollectors(cleanerCollectors ...CleanerCollector) {
	for _, cc := range cleanerCollectors {
		c.cleaners = append(c.cleaners, cc)
		c.collectors = append(c.collectors, cc)
	}
}

func (c *Cluster) GetArtifactsDir() string {
	return c.artifactsDir
}

func (c *Cluster) CreateUserNamespace(ctx context.Context) (*corev1.Namespace, Client) {
	ns, nsClient := c.createNamespace(ctx, c.KubeAdminClient(), c.AdminClientConfig())

	cc := NewNamespaceCleanerCollector(c.AdminClientConfig(), c.KubeAdminClient(), c.DynamicAdminClient(), ns)
	c.AddCleaners(cc)
	c.AddCollectors(cc)

	return ns, nsClient
}

func (c *Cluster) Collect(ctx context.Context, ginkgoNamespace string) {
	for _, collector := range c.collectors {
		collector.CollectToLog(ctx)
		if len(c.artifactsDir) != 0 {
			collector.Collect(ctx, c.artifactsDir, ginkgoNamespace)
		}
	}

	c.collectors = c.collectors[:0]
}

func (c *Cluster) Cleanup(ctx context.Context) {
	for _, cleaner := range c.cleaners {
		cleaner.Cleanup(ctx)
	}

	c.cleaners = c.cleaners[:0]
}
