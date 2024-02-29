package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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

func writeObject(printer ResourcePrinterInterface, filePath string, resourceInfo *ResourceInfo, obj kubeinterfaces.ObjectInterface) error {
	buf := bytes.NewBuffer(nil)
	err := printer.PrintObj(resourceInfo, obj, buf)
	if err != nil {
		return fmt.Errorf("can't print object %q (%s): %w", naming.ObjRef(obj), resourceInfo.Resource, err)
	}

	err = os.WriteFile(filePath, buf.Bytes(), 0770)
	if err != nil {
		return fmt.Errorf("can't write file %q: %w", filePath, err)
	}

	klog.V(4).InfoS("Written resource", "Path", filePath)

	return nil
}

func getResourceKey(obj *unstructured.Unstructured, resourceInfo *ResourceInfo) string {
	var nsPrefix string
	if resourceInfo.Scope.Name() == meta.RESTScopeNameNamespace {
		nsPrefix = obj.GetNamespace() + "/"
	}

	return nsPrefix + fmt.Sprintf("%s/%s/%s", resourceInfo.Resource.Group, resourceInfo.Resource.Resource, obj.GetName())
}

type Collector struct {
	baseDir          string
	printers         []ResourcePrinterInterface
	discoveryClient  discovery.DiscoveryInterface
	corev1Client     corev1client.CoreV1Interface
	dynamicClient    dynamic.Interface
	relatedResources bool
	keepGoing        bool
	logsLimitBytes   int64

	collectedResources sets.Set[string]
}

func NewCollector(
	baseDir string,
	printers []ResourcePrinterInterface,
	discoveryClient discovery.DiscoveryInterface,
	corev1Client corev1client.CoreV1Interface,
	dynamicClient dynamic.Interface,
	relatedResources bool,
	keepGoing bool,
	logsLimitBytes int64,
) *Collector {
	return &Collector{
		baseDir:            baseDir,
		printers:           printers,
		discoveryClient:    discoveryClient,
		corev1Client:       corev1Client,
		dynamicClient:      dynamicClient,
		relatedResources:   relatedResources,
		keepGoing:          keepGoing,
		logsLimitBytes:     logsLimitBytes,
		collectedResources: sets.Set[string]{},
	}
}

func (c *Collector) getResourceDir(obj kubeinterfaces.ObjectInterface, resourceInfo *ResourceInfo) (string, error) {
	scope := resourceInfo.Scope.Name()
	switch scope {
	case meta.RESTScopeNameNamespace:
		return filepath.Join(
			c.baseDir,
			namespacesDirName,
			obj.GetNamespace(),
			resourceInfo.Resource.GroupResource().String(),
		), nil

	case meta.RESTScopeNameRoot:
		return filepath.Join(
			c.baseDir,
			clusterScopedDirName,
			resourceInfo.Resource.GroupResource().String(),
		), nil

	default:
		return "", fmt.Errorf("unrecognized scope %q", scope)
	}
}

func (c *Collector) writeObject(ctx context.Context, dirPath string, obj kubeinterfaces.ObjectInterface, resourceInfo *ResourceInfo) error {
	var err error
	for _, printer := range c.printers {
		filePath := filepath.Join(dirPath, obj.GetName()+printer.GetSuffix())
		err = writeObject(printer, filePath, resourceInfo, obj)
		if err != nil {
			return fmt.Errorf("can't write object: %w", err)
		}
	}

	return nil
}

func (c *Collector) writeResource(ctx context.Context, obj kubeinterfaces.ObjectInterface, resourceInfo *ResourceInfo) error {
	resourceDir, err := c.getResourceDir(obj, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't get resourceDir: %q", err)
	}

	err = os.MkdirAll(resourceDir, 0770)
	if err != nil {
		return fmt.Errorf("can't create resource dir %q: %w", resourceDir, err)
	}

	err = c.writeObject(ctx, resourceDir, obj, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write object: %w", err)
	}

	return nil
}

func (c *Collector) collect(
	ctx context.Context,
	obj kubeinterfaces.ObjectInterface,
	resourceInfo *ResourceInfo,
) error {
	err := c.writeResource(ctx, obj, resourceInfo)
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

	err = c.writeResource(ctx, secret, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	return nil
}

func retrieveContainerLogs(ctx context.Context, podClient corev1client.PodInterface, destinationPath string, podName string, logOptions *corev1.PodLogOptions) error {
	dest, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("can't open file %q: %w", destinationPath, err)
	}
	defer func() {
		err := dest.Close()
		if err != nil {
			klog.ErrorS(err, "can't close file", "Path", destinationPath)
		}
	}()

	logsReq := podClient.GetLogs(podName, logOptions)
	readCloser, err := logsReq.Stream(ctx)
	if err != nil {
		return fmt.Errorf("can't create a log stream: %w", err)
	}
	defer func() {
		err := readCloser.Close()
		if err != nil {
			klog.ErrorS(err, "can't close log stream", "Path", destinationPath, "Pod", podName, "Container", logOptions.Container)
		}
	}()

	_, err = io.Copy(dest, readCloser)
	if err != nil {
		return fmt.Errorf("can't read logs: %w", err)
	}

	return nil
}

func (c *Collector) collectContainerLogs(ctx context.Context, logsDir string, podMeta *metav1.ObjectMeta, podCSs []corev1.ContainerStatus, containerName string) error {
	var err error

	cs, _, found := slices.Find(podCSs, func(s corev1.ContainerStatus) bool {
		return s.Name == containerName
	})
	if !found {
		klog.InfoS("Container doesn't yet have a status", "Pod", naming.ObjRef(podMeta), "Container", containerName)
		return nil
	}

	var limitBytes *int64
	if c.logsLimitBytes > 0 {
		limitBytes = pointer.Ptr(c.logsLimitBytes)
	}

	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Timestamps: true,
		Follow:     false,
		LimitBytes: limitBytes,
	}

	// TODO: Tolerate errors in case state changes in the meantime (like when a pod is being restarted in backoff)
	//       It's error prone to just ignore it, maybe we should retry and refreshing the state and retrying instead.
	//       https://github.com/scylladb/scylla-operator/issues/1400

	if cs.State.Running != nil {
		// Retrieve current logs.
		logOptions.Previous = false
		err = retrieveContainerLogs(ctx, c.corev1Client.Pods(podMeta.Namespace), filepath.Join(logsDir, containerName+".current"), podMeta.Name, logOptions)
		if err != nil {
			return fmt.Errorf("can't retrieve pod logs for container %q in pod %q: %w", containerName, naming.ObjRef(podMeta), err)
		}
	}

	if cs.LastTerminationState.Terminated != nil {
		logOptions.Previous = true
		err = retrieveContainerLogs(ctx, c.corev1Client.Pods(podMeta.Namespace), filepath.Join(logsDir, containerName+".previous"), podMeta.Name, logOptions)
		if err != nil {
			return fmt.Errorf("can't retrieve previous pod logs for container %q in pod %q: %w", containerName, naming.ObjRef(podMeta), err)
		}
	}

	return nil
}

func (c *Collector) collectPod(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	pod := &corev1.Pod{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pod)
	if err != nil {
		return fmt.Errorf("can't convert secret from unstructured: %w", err)
	}

	err = c.writeResource(ctx, pod, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	resourceDir, err := c.getResourceDir(pod, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't get resourceDir: %q", err)
	}
	logsDir := filepath.Join(resourceDir, pod.GetName())

	err = os.MkdirAll(logsDir, 0770)
	if err != nil {
		return fmt.Errorf("can't create logs dir %q: %w", logsDir, err)
	}

	for _, container := range pod.Spec.InitContainers {
		err = c.collectContainerLogs(ctx, logsDir, &pod.ObjectMeta, pod.Status.InitContainerStatuses, container.Name)
		if err != nil {
			return fmt.Errorf("can't collect logs for init container %q in pod %q: %w", container.Name, naming.ObjRef(pod), err)
		}
	}

	for _, container := range pod.Spec.Containers {
		err = c.collectContainerLogs(ctx, logsDir, &pod.ObjectMeta, pod.Status.ContainerStatuses, container.Name)
		if err != nil {
			return fmt.Errorf("can't collect logs for container %q in pod %q: %w", container.Name, naming.ObjRef(pod), err)
		}
	}

	for _, container := range pod.Spec.EphemeralContainers {
		err = c.collectContainerLogs(ctx, logsDir, &pod.ObjectMeta, pod.Status.EphemeralContainerStatuses, container.Name)
		if err != nil {
			return fmt.Errorf("can't collect logs for ephemeral container %q in pod %q: %w", container.Name, naming.ObjRef(pod), err)
		}
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

	return helpers.GetMapValues(resourceInfosMap), nil
}

func (c *Collector) collectNamespace(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	err := c.writeResource(ctx, u, resourceInfo)
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
		return c.collectPod(ctx, u, resourceInfo)

	case corev1.SchemeGroupVersion.WithResource("namespaces").GroupResource():
		return c.collectNamespace(ctx, u, resourceInfo)

	default:
		return c.collect(ctx, u, resourceInfo)
	}
}

func (c *Collector) CollectResource(ctx context.Context, resourceInfo *ResourceInfo, namespace, name string) error {
	obj, err := c.dynamicClient.Resource(resourceInfo.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

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

	return apierrors.NewAggregate(errs)
}
