// Copyright (C) 2021 ScyllaDB

package framework

import (
	"context"
	"fmt"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllaclientset "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

const (
	ServiceAccountName                   = "e2e-user"
	ServiceAccountTokenSecretName        = "e2e-user-token"
	serviceAccountWaitTimeout            = 1 * time.Minute
	serviceAccountTokenSecretWaitTimeout = 1 * time.Minute
)

type Framework struct {
	name string

	clusters []*Cluster
}

func NewFramework(name string) *Framework {
	uniqueName := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", name))

	clusters := make([]*Cluster, 0, len(TestContext.RestConfigs))
	for i, restConfig := range TestContext.RestConfigs {
		clusterName := fmt.Sprintf("%s-%d", uniqueName, i)
		c := NewCluster(clusterName, restConfig)
		clusters = append(clusters, c)
	}

	f := &Framework{
		name:     uniqueName,
		clusters: clusters,
	}

	g.BeforeEach(f.beforeEach)
	g.AfterEach(f.afterEach)

	return f
}

func (f *Framework) Cluster(idx int) *Cluster {
	return f.clusters[idx]
}

func (f *Framework) Clusters() []*Cluster {
	return f.clusters
}

func (f *Framework) Namespace() string {
	return f.Cluster(0).Namespace()
}

func (f *Framework) Username() string {
	return f.Cluster(0).Username()
}

func (f *Framework) GetIngressAddress(hostname string) string {
	if TestContext.IngressController == nil || len(TestContext.IngressController.Address) == 0 {
		return hostname
	}

	return TestContext.IngressController.Address
}

func (f *Framework) FieldManager() string {
	return f.Cluster(0).FieldManager()
}

func (f *Framework) ClientConfig() *restclient.Config {
	return f.Cluster(0).ClientConfig()
}

func (f *Framework) AdminClientConfig() *restclient.Config {
	return f.Cluster(0).AdminClientConfig()
}

func (f *Framework) DiscoveryClient() *discovery.DiscoveryClient {
	return f.Cluster(0).DiscoveryClient()
}

func (f *Framework) DynamicClient() dynamic.Interface {
	return f.Cluster(0).DynamicClient()
}

func (f *Framework) DynamicAdminClient() dynamic.Interface {
	return f.Cluster(0).DynamicAdminClient()
}

func (f *Framework) KubeClient() *kubernetes.Clientset {
	return f.Cluster(0).KubeClient()
}

func (f *Framework) KubeAdminClient() *kubernetes.Clientset {
	return f.Cluster(0).KubeAdminClient()
}

func (f *Framework) ScyllaClient() *scyllaclientset.Clientset {
	return f.Cluster(0).ScyllaClient()
}

func (f *Framework) ScyllaAdminClient() *scyllaclientset.Clientset {
	return f.Cluster(0).ScyllaAdminClient()
}

func (f *Framework) CommonLabels() map[string]string {
	return f.Cluster(0).CommonLabels()
}

func (f *Framework) GetDefaultScyllaCluster() *scyllav1.ScyllaCluster {
	renderArgs := map[string]any{
		"nodeServiceType":             TestContext.ScyllaClusterOptions.ExposeOptions.NodeServiceType,
		"nodesBroadcastAddressType":   TestContext.ScyllaClusterOptions.ExposeOptions.NodesBroadcastAddressType,
		"clientsBroadcastAddressType": TestContext.ScyllaClusterOptions.ExposeOptions.ClientsBroadcastAddressType,
	}

	sc, _, err := scyllafixture.ScyllaClusterTemplate.RenderObject(renderArgs)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sc
}

func (f *Framework) beforeEach() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, c := range f.clusters {
		wg.Add(1)

		go func() {
			defer wg.Done()

			c.beforeEach(ctx)
		}()
	}
}

func (f *Framework) afterEach() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, c := range f.clusters {
		wg.Add(1)

		go func() {
			defer wg.Done()

			c.afterEach(ctx)
		}()
	}
}
