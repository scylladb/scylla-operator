package framework

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func DumpResource(ctx context.Context, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, corev1Client corev1client.CoreV1Interface, artifactsDir string, resourceInfo *collect.ResourceInfo, namespace string, name string) error {
	collector := collect.NewCollector(
		artifactsDir,
		[]collect.ResourcePrinterInterface{
			&collect.OmitManagedFieldsPrinter{
				Delegate: &collect.YAMLPrinter{},
			},
		},
		discoveryClient,
		corev1Client,
		dynamicClient,
		true,
		true,
		0,
	)
	err := collector.CollectResource(
		ctx,
		resourceInfo,
		namespace,
		name,
	)
	if err != nil {
		return fmt.Errorf("can't collect object %q (%s): %w", naming.ManualRef(namespace, name), resourceInfo.Resource.String(), err)
	}

	return nil
}

func DumpNamespace(ctx context.Context, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, corev1Client corev1client.CoreV1Interface, artifactsDir string, name string) error {
	return DumpResource(
		ctx,
		discoveryClient,
		dynamicClient,
		corev1Client,
		artifactsDir,
		&collect.ResourceInfo{
			Resource: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "namespaces",
			},
			Scope: meta.RESTScopeRoot,
		},
		corev1.NamespaceAll,
		name,
	)
}
