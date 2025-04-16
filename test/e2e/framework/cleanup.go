package framework

import (
	"context"
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

type Cleaner interface {
	Cleanup(ctx context.Context)
}

type Collector interface {
	CollectToLog(ctx context.Context)
	Collect(ctx context.Context, artifactsDir string, ginkgoNamespace string)
}

type NamespaceCleanerCollector struct {
	Client        kubernetes.Interface
	DynamicClient dynamic.Interface
	NS            *corev1.Namespace
}

var _ Cleaner = &NamespaceCleanerCollector{}
var _ Collector = &NamespaceCleanerCollector{}

func NewNamespaceCleanerCollector(client kubernetes.Interface, dynamicClient dynamic.Interface, namespace *corev1.Namespace) *NamespaceCleanerCollector {
	return &NamespaceCleanerCollector{
		Client:        client,
		DynamicClient: dynamicClient,
		NS:            namespace,
	}
}

func (nc *NamespaceCleanerCollector) Cleanup(ctx context.Context) {
	By("Destroying namespace %q.", nc.NS.Name)
	err := nc.Client.CoreV1().Namespaces().Delete(
		ctx,
		nc.NS.Name,
		metav1.DeleteOptions{
			GracePeriodSeconds: pointer.Ptr[int64](0),
			PropagationPolicy:  pointer.Ptr(metav1.DeletePropagationForeground),
			Preconditions: &metav1.Preconditions{
				UID: &nc.NS.UID,
			},
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	// We have deleted only the namespace object, but it can still be there with deletionTimestamp set.
	By("Waiting for namespace %q to be removed.", nc.NS.Name)
	err = WaitForObjectDeletion(ctx, nc.DynamicClient, corev1.SchemeGroupVersion.WithResource("namespaces"), "", nc.NS.Name, &nc.NS.UID)
	o.Expect(err).NotTo(o.HaveOccurred())
	klog.InfoS("Namespace removed.", "Namespace", nc.NS.Name)
}

func (nc *NamespaceCleanerCollector) CollectToLog(ctx context.Context) {
	// Log events if the test failed.
	if g.CurrentSpecReport().Failed() {
		By("Collecting events from namespace %q.", nc.NS.Name)
		DumpEventsInNamespace(ctx, nc.Client, nc.NS.Name)
	}
}

func (nc *NamespaceCleanerCollector) Collect(ctx context.Context, artifactsDir string, _ string) {
	By("Collecting dumps from namespace %q.", nc.NS.Name)

	err := DumpNamespace(ctx, cacheddiscovery.NewMemCacheClient(nc.Client.Discovery()), nc.DynamicClient, nc.Client.CoreV1(), artifactsDir, nc.NS.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

type RestoreStrategy string

const (
	RestoreStrategyRecreate RestoreStrategy = "Recreate"
	RestoreStrategyUpdate   RestoreStrategy = "Update"
)

type RestoringCleaner struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	resourceInfo  collect.ResourceInfo
	object        *unstructured.Unstructured
	strategy      RestoreStrategy
}

var _ Cleaner = &RestoringCleaner{}
var _ Collector = &RestoringCleaner{}

func NewRestoringCleaner(ctx context.Context, client kubernetes.Interface, dynamicClient dynamic.Interface, resourceInfo collect.ResourceInfo, namespace string, name string, strategy RestoreStrategy) *RestoringCleaner {
	g.By(fmt.Sprintf("Snapshotting object %s %q", resourceInfo.Resource, naming.ManualRef(namespace, name)))

	if resourceInfo.Scope.Name() == meta.RESTScopeNameNamespace {
		o.Expect(namespace).NotTo(o.BeEmpty())
	}

	obj, err := dynamicClient.Resource(resourceInfo.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.InfoS("No existing object found", "GVR", resourceInfo.Resource, "Instance", naming.ManualRef(namespace, name))
		obj = &unstructured.Unstructured{
			Object: map[string]interface{}{},
		}
		obj.SetNamespace(namespace)
		obj.SetName(name)
		obj.SetUID("")
	} else {
		o.Expect(err).NotTo(o.HaveOccurred())
		klog.InfoS("Snapshotted object", "GVR", resourceInfo.Resource, "Instance", naming.ManualRef(namespace, name), "UID", obj.GetUID())
	}

	return &RestoringCleaner{
		client:        client,
		dynamicClient: dynamicClient,
		resourceInfo:  resourceInfo,
		object:        obj,
		strategy:      strategy,
	}
}

func (rc *RestoringCleaner) getCleansedObject() *unstructured.Unstructured {
	obj := rc.object.DeepCopy()
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetCreationTimestamp(metav1.Time{})
	obj.SetDeletionTimestamp(nil)
	return obj
}

func (rc *RestoringCleaner) CollectToLog(ctx context.Context) {}

func (rc *RestoringCleaner) Collect(ctx context.Context, clusterArtifactsDir string, ginkgoNamespace string) {
	artifactsDir := clusterArtifactsDir
	if len(artifactsDir) != 0 && rc.resourceInfo.Scope.Name() == meta.RESTScopeNameRoot {
		// We have to prevent global object dumps being overwritten with each "It" block.
		artifactsDir = filepath.Join(artifactsDir, "cluster-scoped-per-ns", ginkgoNamespace)
	}

	By("Collecting global %s %q for namespace %q.", rc.resourceInfo.Resource, naming.ObjRef(rc.object), ginkgoNamespace)

	err := DumpResource(
		ctx,
		rc.client.Discovery(),
		rc.dynamicClient,
		rc.client.CoreV1(),
		artifactsDir,
		&rc.resourceInfo,
		rc.object.GetNamespace(),
		rc.object.GetName(),
	)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("Skipping object collection because it no longer exists", "Ref", naming.ObjRef(rc.object), "Resource", rc.resourceInfo.Resource)
	} else {
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func (rc *RestoringCleaner) DeleteObject(ctx context.Context, ignoreNotFound bool) {
	By("Deleting object %s %q.", rc.resourceInfo.Resource, naming.ObjRef(rc.object))
	err := rc.dynamicClient.Resource(rc.resourceInfo.Resource).Namespace(rc.object.GetNamespace()).Delete(
		ctx,
		rc.object.GetName(),
		metav1.DeleteOptions{
			GracePeriodSeconds: pointer.Ptr[int64](0),
			PropagationPolicy:  pointer.Ptr(metav1.DeletePropagationForeground),
		},
	)
	if apierrors.IsNotFound(err) && ignoreNotFound {
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	// We have deleted only the object, but it can still be there with deletionTimestamp set.
	By("Waiting for object %s %q to be removed.", rc.resourceInfo.Resource, naming.ObjRef(rc.object))
	err = WaitForObjectDeletion(ctx, rc.dynamicClient, rc.resourceInfo.Resource, rc.object.GetNamespace(), rc.object.GetName(), nil)
	o.Expect(err).NotTo(o.HaveOccurred())
	By("Object %s %q has been removed.", rc.resourceInfo.Resource, naming.ObjRef(rc.object))
}

func (rc *RestoringCleaner) recreateObject(ctx context.Context) {
	o.Expect(rc.object.GetUID()).NotTo(o.BeEmpty())

	rc.DeleteObject(ctx, true)

	_, err := rc.dynamicClient.Resource(rc.resourceInfo.Resource).Namespace(rc.object.GetNamespace()).Create(ctx, rc.getCleansedObject(), metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (rc *RestoringCleaner) replaceObject(ctx context.Context) {
	o.Expect(rc.object.GetUID()).NotTo(o.BeEmpty())
	var err error
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var freshObj *unstructured.Unstructured
		freshObj, err = rc.dynamicClient.Resource(rc.resourceInfo.Resource).Namespace(rc.object.GetNamespace()).Get(ctx, rc.object.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = rc.dynamicClient.Resource(rc.resourceInfo.Resource).Namespace(rc.object.GetNamespace()).Create(ctx, rc.getCleansedObject(), metav1.CreateOptions{})
			return err
		}

		obj := rc.getCleansedObject()
		obj.SetResourceVersion(freshObj.GetResourceVersion())

		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = rc.dynamicClient.Resource(rc.resourceInfo.Resource).Namespace(obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (rc *RestoringCleaner) restoreObject(ctx context.Context) {
	By("Restoring original object %s %q.", rc.resourceInfo.Resource, naming.ObjRef(rc.object))
	switch rc.strategy {
	case RestoreStrategyRecreate:
		rc.recreateObject(ctx)
	case RestoreStrategyUpdate:
		rc.replaceObject(ctx)
	default:
		g.Fail(fmt.Sprintf("unexpected strategy %q", rc.strategy))
	}
}

func (rc *RestoringCleaner) Cleanup(ctx context.Context) {
	if len(rc.object.GetUID()) == 0 {
		rc.DeleteObject(ctx, true)
		return
	}

	rc.restoreObject(ctx)
}
