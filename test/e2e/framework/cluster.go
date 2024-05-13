// Copyright (C) 2024 ScyllaDB

package framework

import (
	"context"
	"fmt"
	"os"
	"path"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type ClusterInterface interface {
	AdminClientInterface
	DefaultNamespaceIfAny() (*corev1.Namespace, Client, bool)
	CreateUserNamespace(ctx context.Context) (*corev1.Namespace, Client)
}

type createNamespaceFunc func(ctx context.Context, adminClient kubernetes.Interface, adminClientConfig *restclient.Config) (*corev1.Namespace, Client)

type Cluster struct {
	AdminClient

	name string

	createNamespace    createNamespaceFunc
	defaultNamespace   *corev1.Namespace
	defaultClient      Client
	namespacesToDelete []*corev1.Namespace
}

var _ AdminClientInterface = &Cluster{}
var _ ClusterInterface = &Cluster{}

func NewCluster(name string, restConfig *restclient.Config, createNamespace createNamespaceFunc) *Cluster {
	adminClientConfig := restclient.CopyConfig(restConfig)

	return &Cluster{
		AdminClient: AdminClient{
			Config: adminClientConfig,
		},

		name: name,

		createNamespace:  createNamespace,
		defaultNamespace: nil,
		defaultClient: Client{
			Config: nil,
		},
		namespacesToDelete: []*corev1.Namespace{},
	}
}

func (c *Cluster) DefaultNamespaceIfAny() (*corev1.Namespace, Client, bool) {
	if c.defaultNamespace == nil {
		return nil, Client{}, false
	}

	return c.defaultNamespace, c.defaultClient, true
}

func (c *Cluster) CreateUserNamespace(ctx context.Context) (*corev1.Namespace, Client) {
	ns, nsClient := c.createNamespace(ctx, c.KubeAdminClient(), c.AdminClientConfig())

	c.namespacesToDelete = append(c.namespacesToDelete, ns)

	return ns, nsClient
}

func (c *Cluster) Cleanup(ctx context.Context) {
	for _, ns := range c.namespacesToDelete {
		collectAndDeleteNamespace(ctx, c.KubeAdminClient(), c.DynamicAdminClient(), ns)
	}

	c.namespacesToDelete = []*corev1.Namespace{}
}

func collectAndDeleteNamespace(ctx context.Context, adminClient kubernetes.Interface, dynamicAdminClient dynamic.Interface, ns *corev1.Namespace) {
	defer func() {
		keepNamespace := false
		switch TestContext.DeleteTestingNSPolicy {
		case DeleteTestingNSPolicyNever:
			keepNamespace = true
		case DeleteTestingNSPolicyOnSuccess:
			if g.CurrentSpecReport().Failed() {
				keepNamespace = true
			}
		case DeleteTestingNSPolicyAlways:
		default:
		}

		if keepNamespace {
			By("Keeping namespace %q for debugging", ns.Name)
			return
		}

		deleteNamespace(ctx, adminClient, dynamicAdminClient, ns)

	}()

	// Print events if the test failed.
	if g.CurrentSpecReport().Failed() {
		By(fmt.Sprintf("Collecting events from namespace %q.", ns.Name))
		DumpEventsInNamespace(ctx, adminClient, ns.Name)
	}

	// CI can't keep namespaces alive because it could get out of resources for the other tests
	// so we need to collect the namespaced dump before destroying the namespace.
	// Collecting artifacts even for successful runs helps to verify if it went
	// as expected and the amount of data is bearable.
	if len(TestContext.ArtifactsDir) != 0 {
		By(fmt.Sprintf("Collecting dumps from namespace %q.", ns.Name))

		d := path.Join(TestContext.ArtifactsDir, "e2e")
		err := os.Mkdir(d, 0777)
		if err != nil && !os.IsExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		err = DumpNamespace(ctx, cacheddiscovery.NewMemCacheClient(adminClient.Discovery()), dynamicAdminClient, adminClient.CoreV1(), d, ns.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func deleteNamespace(ctx context.Context, adminClient kubernetes.Interface, dynamicAdminClient dynamic.Interface, ns *corev1.Namespace) {
	By("Destroying namespace %q.", ns.Name)
	var gracePeriod int64 = 0
	var propagation = metav1.DeletePropagationForeground
	err := adminClient.CoreV1().Namespaces().Delete(
		ctx,
		ns.Name,
		metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			PropagationPolicy:  &propagation,
			Preconditions: &metav1.Preconditions{
				UID: &ns.UID,
			},
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	// We have deleted only the namespace object but it is still there with deletionTimestamp set.
	By("Waiting for namespace %q to be removed.", ns.Name)
	err = WaitForObjectDeletion(ctx, dynamicAdminClient, corev1.SchemeGroupVersion.WithResource("namespaces"), "", ns.Name, &ns.UID)
	o.Expect(err).NotTo(o.HaveOccurred())
	klog.InfoS("Namespace removed.", "Namespace", ns.Name)
}
