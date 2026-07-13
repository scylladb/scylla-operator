package framework

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

func DumpResource(ctx context.Context, restConfig *rest.Config, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, corev1Client corev1client.CoreV1Interface, artifactsDir string, resourceInfo *collect.ResourceInfo, namespace string, name string) error {
	discoverer := collect.NewResourceDiscoverer(true, discoveryClient)
	discoveredResources, err := discoverer.DiscoverResources()
	if err != nil {
		return fmt.Errorf("can't discover resources: %w", err)
	}

	collector := collect.NewCollector(
		artifactsDir,
		[]collect.ResourcePrinterInterface{
			&collect.OmitManagedFieldsPrinter{
				Delegate: &collect.YAMLPrinter{},
			},
		},
		restConfig,
		discoveredResources,
		corev1Client,
		dynamicClient,
		true,
		true,
		0,
	)
	if err := collector.CollectResourceObject(
		ctx,
		resourceInfo,
		namespace,
		name,
	); err != nil {
		return fmt.Errorf("can't collect object %q (%s): %w", naming.ManualRef(namespace, name), resourceInfo.Resource.String(), err)
	}

	return nil
}

func DumpNamespace(ctx context.Context, restConfig *rest.Config, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, corev1Client corev1client.CoreV1Interface, artifactsDir string, name string) error {
	return DumpResource(
		ctx,
		restConfig,
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

// DumpPodAndFollowLogs starts dumping the pod manifest and following its current container logs.
// The first returned function waits until all current log streams are open or collection fails,
// and the second one cancels log following and waits for collection to finish.
func DumpPodAndFollowLogs(ctx context.Context, restConfig *rest.Config, dynamicClient dynamic.Interface, corev1Client corev1client.CoreV1Interface, artifactsDir string, namespace string, name string, options collect.CollectObjectOptions) (func(context.Context) error, func(context.Context) error, error) {
	collector := collect.NewCollector(
		artifactsDir,
		[]collect.ResourcePrinterInterface{
			&collect.OmitManagedFieldsPrinter{
				Delegate: &collect.YAMLPrinter{},
			},
		},
		restConfig,
		nil,
		corev1Client,
		dynamicClient,
		false,
		true,
		0,
	)

	followCtx, followCtxCancel := context.WithCancel(ctx)
	streamOpenCh := make(chan struct{})
	doneCh := make(chan struct{})

	streamOpenCallback := func() {
		close(streamOpenCh)
	}

	podResourceInfo := &collect.ResourceInfo{
		Scope:    meta.RESTScopeNamespace,
		Resource: corev1.SchemeGroupVersion.WithResource("pods"),
	}

	var wg sync.WaitGroup
	var collectErr error

	wg.Add(1)
	go func() {
		defer wg.Done()

		collectErr = collector.CollectResourceObjectWithOptionsAndFollowLogs(followCtx, podResourceInfo, namespace, name, options, streamOpenCallback)
		close(doneCh)
	}()

	waitForStreamOpened := func(ctx context.Context) error {
		select {
		case <-streamOpenCh:
			return nil

		case <-doneCh:
			if collectErr != nil {
				return fmt.Errorf("failed to collect pod logs: %w", collectErr)
			}

			return nil

		case <-ctx.Done():
			return fmt.Errorf("failed to wait for pod logs to open: %w", ctx.Err())

		}
	}

	stopAndWait := func(ctx context.Context) error {
		followCtxCancel()
		wg.Wait()

		select {
		case <-doneCh:
			// Calling followCtxCancel() surfaces as context.Canceled. It's an expected stop path.
			if collectErr != nil && !errors.Is(collectErr, context.Canceled) {
				return fmt.Errorf("failed to collect pod logs: %w", collectErr)
			}

			return nil

		case <-ctx.Done():
			return fmt.Errorf("failed to stop collecting pod logs: %w", ctx.Err())

		}
	}

	return waitForStreamOpened, stopAndWait, nil
}
