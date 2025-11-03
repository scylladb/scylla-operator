package collect

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	namespacesDirName    = "namespaces"
	clusterScopedDirName = "cluster-scoped"
)

func NewResourceInfoFromMapping(mapping *meta.RESTMapping) *ResourceInfo {
	return &ResourceInfo{
		Scope:    mapping.Scope,
		Resource: mapping.Resource,
	}
}

func getResourceKey(obj *unstructured.Unstructured, resourceInfo *ResourceInfo) string {
	var nsPrefix string
	if resourceInfo.Scope.Name() == meta.RESTScopeNameNamespace {
		nsPrefix = obj.GetNamespace() + "/"
	}

	return nsPrefix + fmt.Sprintf("%s/%s/%s", resourceInfo.Resource.Group, resourceInfo.Resource.Resource, obj.GetName())
}

type Collector struct {
	dynamicClient    dynamic.Interface
	podCollector     *PodCollector
	resourceWriter   *ResourceWriter
	relatedResources bool
	keepGoing        bool

	discoveredResources []*ResourceInfo
	collectedResources  apimachineryutilsets.Set[string]
}

func NewCollector(
	baseDir string,
	printers []ResourcePrinterInterface,
	restConfig *rest.Config,
	discoveredResources []*ResourceInfo,
	corev1Client corev1client.CoreV1Interface,
	dynamicClient dynamic.Interface,
	relatedResources bool,
	keepGoing bool,
	logsLimitBytes int64,
) *Collector {
	rw := NewResourceWriter(baseDir, printers)
	pc := NewPodCollector(restConfig, corev1Client, rw, logsLimitBytes)

	return &Collector{
		discoveredResources: discoveredResources,
		dynamicClient:       dynamicClient,
		podCollector:        pc,
		resourceWriter:      rw,
		relatedResources:    relatedResources,
		keepGoing:           keepGoing,
		collectedResources:  apimachineryutilsets.Set[string]{},
	}
}

func (c *Collector) collect(
	ctx context.Context,
	obj kubeinterfaces.ObjectInterface,
	resourceInfo *ResourceInfo,
) error {
	err := c.resourceWriter.WriteResource(ctx, obj, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	return nil
}

func (c *Collector) collectNamespace(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	err := c.resourceWriter.WriteResource(ctx, u, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	if !c.relatedResources {
		return nil
	}

	namespace := u.GetName()
	for _, m := range c.onlyNamespacedResources() {
		var errs []error
		err = c.CollectResourceObjects(ctx, m, namespace)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't collect %s in namespace %s: %w", m.Resource, namespace, err))

			if !c.keepGoing {
				break
			}

			klog.Error(err, "Can't collect resource", "Resource", m.Resource, "Namespace", namespace)
		}
	}

	return nil
}

func (c *Collector) collectNamespaceForObject(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	err := c.CollectResourceObject(
		ctx,
		&ResourceInfo{
			Scope:    meta.RESTScopeRoot,
			Resource: corev1.SchemeGroupVersion.WithResource("namespaces"),
		},
		"",
		u.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("can't collect namespace %q: %w", resourceInfo, err)
	}

	return nil
}

func (c *Collector) collectScyllaCluster(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	err := c.resourceWriter.WriteResource(ctx, u, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	if !c.relatedResources {
		return nil
	}

	err = c.collectNamespaceForObject(ctx, u, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't collect related namespace for object %q: %w", resourceInfo, err)
	}

	return nil
}

func (c *Collector) collectScyllaDBMonitoring(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	err := c.resourceWriter.WriteResource(ctx, u, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	if !c.relatedResources {
		return nil
	}

	err = c.collectNamespaceForObject(ctx, u, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't collect related namespace for object %q: %w", resourceInfo, err)
	}

	return nil
}

func (c *Collector) CollectObject(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	key := getResourceKey(u, resourceInfo)
	if c.collectedResources.Has(key) {
		klog.V(3).InfoS("Skipping already collected resource", "Resource", resourceInfo.Resource, "Ref", naming.ObjRef(u))
		return nil
	}
	c.collectedResources.Insert(key)

	switch resourceInfo.Resource.GroupResource() {
	case corev1.SchemeGroupVersion.WithResource("pods").GroupResource():
		return c.podCollector.Collect(ctx, u, resourceInfo)

	case corev1.SchemeGroupVersion.WithResource("namespaces").GroupResource():
		return c.collectNamespace(ctx, u, resourceInfo)

	case scyllav1.GroupVersion.WithResource("scyllaclusters").GroupResource():
		return c.collectScyllaCluster(ctx, u, resourceInfo)

	case scyllav1alpha1.GroupVersion.WithResource("scylladbmonitorings").GroupResource():
		return c.collectScyllaDBMonitoring(ctx, u, resourceInfo)

	default:
		return c.collect(ctx, u, resourceInfo)
	}
}

func (c *Collector) CollectResourceObject(ctx context.Context, resourceInfo *ResourceInfo, namespace, name string) error {
	obj, err := c.dynamicClient.Resource(resourceInfo.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get resource %q: %w", resourceInfo.Resource, err)
	}

	return c.CollectObject(ctx, obj, resourceInfo)
}

func (c *Collector) CollectResourceObjects(ctx context.Context, resourceInfo *ResourceInfo, namespace string) error {
	l, err := c.dynamicClient.Resource(resourceInfo.Resource).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("can't list resource %q: %w", resourceInfo.Resource, err)
	}

	var errs []error
	for _, obj := range l.Items {
		err = c.CollectObject(ctx, &obj, resourceInfo)
		if err != nil {
			errs = append(errs, err)

			if !c.keepGoing {
				break
			}

			klog.Error(err, "can't collect object", "Object", klog.KObj(&obj))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (c *Collector) CollectResourcesObjects(ctx context.Context, resources []*ResourceInfo) error {
	var errs []error

	for _, r := range resources {
		namespace := corev1.NamespaceAll
		err := c.CollectResourceObjects(ctx, r, namespace)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't collect resource %q: %w", r.Resource, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (c *Collector) CollectNamespaces(ctx context.Context, namespaces []string) error {
	var errs []error

	for _, ns := range namespaces {
		err := c.CollectResourceObject(
			ctx,
			&ResourceInfo{
				Scope:    meta.RESTScopeRoot,
				Resource: corev1.SchemeGroupVersion.WithResource("namespaces"),
			},
			"",
			ns,
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't collect namespace %q: %w", ns, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (c *Collector) onlyNamespacedResources() []*ResourceInfo {
	var namespacedResources []*ResourceInfo
	for _, r := range c.discoveredResources {
		if r.Scope.Name() == meta.RESTScopeNameNamespace {
			namespacedResources = append(namespacedResources, r)
		}
	}
	return namespacedResources
}
