package collect

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	namespacesDirName    = "namespaces"
	clusterScopedDirName = "cluster-scoped"
)

type ResourceInfo struct {
	Scope    meta.RESTScope
	Resource schema.GroupVersionResource
}

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
	discoveryClient  discovery.DiscoveryInterface
	dynamicClient    dynamic.Interface
	podCollector     *PodCollector
	resourceWriter   *ResourceWriter
	relatedResources bool
	keepGoing        bool

	collectedResources apimachineryutilsets.Set[string]
}

func NewCollector(
	baseDir string,
	printers []ResourcePrinterInterface,
	restConfig *rest.Config,
	discoveryClient discovery.DiscoveryInterface,
	corev1Client corev1client.CoreV1Interface,
	dynamicClient dynamic.Interface,
	relatedResources bool,
	keepGoing bool,
	logsLimitBytes int64,
) *Collector {
	rw := NewResourceWriter(baseDir, printers)
	pc := NewPodCollector(restConfig, corev1Client, rw, logsLimitBytes)

	return &Collector{
		discoveryClient:    discoveryClient,
		dynamicClient:      dynamicClient,
		podCollector:       pc,
		resourceWriter:     rw,
		relatedResources:   relatedResources,
		keepGoing:          keepGoing,
		collectedResources: apimachineryutilsets.Set[string]{},
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

func isPublicSecretKey(key string) bool {
	switch key {
	case "ca.crt", "tls.crt", "service-ca.crt":
		return true
	default:
		return false
	}
}

func (c *Collector) collectSecret(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	secret := &corev1.Secret{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, secret)
	if err != nil {
		return fmt.Errorf("can't convert secret from unstructured: %w", err)
	}

	for k := range secret.Data {
		if !isPublicSecretKey(k) {
			secret.Data[k] = []byte("<redacted>")
		}
	}

	err = c.resourceWriter.WriteResource(ctx, secret, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	return nil
}

func (c *Collector) DiscoverResources(ctx context.Context, filter discovery.ResourcePredicateFunc) ([]*ResourceInfo, error) {
	all, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("can't discover resources: %w", err)
	}

	rls := discovery.FilteredBy(filter, all)

	// There should be at least one resource per group, likely more.
	resourceInfos := make([]*ResourceInfo, 0, len(rls))
	for _, rl := range rls {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, r := range rl.APIResources {
			var scope meta.RESTScope
			if r.Namespaced {
				scope = meta.RESTScopeNamespace
			} else {
				scope = meta.RESTScopeRoot
			}
			resourceInfos = append(resourceInfos, &ResourceInfo{
				Scope:    scope,
				Resource: gv.WithResource(r.Name),
			})
		}
	}

	return resourceInfos, nil
}

func ReplaceIsometricResourceInfosIfPresent(resourceInfos []*ResourceInfo) ([]*ResourceInfo, error) {
	// Replacements are order dependent and should start from the oldest API.
	replacements := []struct {
		old schema.GroupVersionResource
		new schema.GroupVersionResource
	}{
		{
			old: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "events",
			},
			new: schema.GroupVersionResource{
				Group:    "events.k8s.io",
				Version:  "v1",
				Resource: "events",
			},
		},
	}

	// Resources to exclude from discovery (deprecated APIs without replacement)
	excludedResources := []schema.GroupVersionResource{
		{
			Group:    "",
			Version:  "v1",
			Resource: "componentstatuses",
		},
	}

	resourceInfosMap := make(map[schema.GroupVersionResource]*ResourceInfo, len(resourceInfos))
	for _, m := range resourceInfos {
		resourceInfosMap[m.Resource] = m
	}

	for _, replacement := range replacements {
		_, found := resourceInfosMap[replacement.new]
		if found {
			delete(resourceInfosMap, replacement.old)
		}
	}

	// Remove deprecated resources that have no replacement
	for _, excluded := range excludedResources {
		delete(resourceInfosMap, excluded)
	}

	return helpers.GetMapValues(resourceInfosMap), nil
}

func (c *Collector) collectNamespace(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	err := c.resourceWriter.WriteResource(ctx, u, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	if !c.relatedResources {
		return nil
	}

	namespacedResourceInfos, err := c.DiscoverResources(ctx, func(gv string, r *metav1.APIResource) bool {
		if !r.Namespaced {
			return false
		}

		return discovery.SupportsAllVerbs{
			Verbs: []string{"list"},
		}.Match(gv, r)
	})
	if err != nil {
		return fmt.Errorf("can't discover resource: %w", err)
	}

	// Filter out native resources that share storage across groups.
	namespacedResourceInfos, err = ReplaceIsometricResourceInfosIfPresent(namespacedResourceInfos)
	if err != nil {
		return fmt.Errorf("can't repalce isometric resourceInfos: %w", err)
	}

	namespace := u.GetName()
	for _, m := range namespacedResourceInfos {
		var errs []error
		err = c.CollectResources(ctx, m, namespace)
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
	err := c.CollectResource(
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
	case corev1.SchemeGroupVersion.WithResource("secrets").GroupResource():
		return c.collectSecret(ctx, u, resourceInfo)

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

func (c *Collector) CollectResource(ctx context.Context, resourceInfo *ResourceInfo, namespace, name string) error {
	obj, err := c.dynamicClient.Resource(resourceInfo.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get resource %q: %w", resourceInfo.Resource, err)
	}

	return c.CollectObject(ctx, obj, resourceInfo)
}

func (c *Collector) CollectResources(ctx context.Context, resourceInfo *ResourceInfo, namespace string) error {
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
