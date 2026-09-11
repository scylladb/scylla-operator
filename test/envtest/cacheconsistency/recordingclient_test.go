//go:build envtest

package cacheconsistency

import (
	"context"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/cacheconsistency"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

const (
	// informerLag keeps the informers behind the API server, so that a read right after a write is provably stale
	// without WaitReady.
	informerLag = 1 * time.Second

	waitReadyTimeout = 15 * time.Second
	// bookmarkWaitTimeout covers the API server's watch bookmark interval, which is how a write the informers can't
	// observe gets past WaitReady. Bookmarks arrive within seconds here, but the interval is up to a minute.
	bookmarkWaitTimeout = 2 * time.Minute
)

// harness runs lagging informers over the spec namespace behind a recording client and a ConsistencyStore, the way a
// controller would, with no controller around.
type harness struct {
	env    *envtest.Environment
	store  *cacheconsistency.ConsistencyStore
	client kubernetes.Interface

	configMaps   corev1listers.ConfigMapNamespaceLister
	pods         corev1listers.PodNamespaceLister
	statefulSets appsv1listers.StatefulSetNamespaceLister
}

func newHarness(ctx context.Context, env *envtest.Environment) *harness {
	g.GinkgoHelper()

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(
		env.TypedKubeClient(),
		0,
		informers.WithNamespace(env.Namespace()),
		informers.WithTransform(func(obj any) (any, error) {
			time.Sleep(informerLag)
			return obj, nil
		}),
	)

	store := cacheconsistency.NewConsistencyStore()
	o.Expect(store.Register(&corev1.ConfigMap{}, kubeInformers.Core().V1().ConfigMaps().Informer())).To(o.Succeed())
	o.Expect(store.Register(&corev1.Pod{}, kubeInformers.Core().V1().Pods().Informer())).To(o.Succeed())
	o.Expect(store.Register(&appsv1.StatefulSet{}, kubeInformers.Apps().V1().StatefulSets().Informer())).To(o.Succeed())

	h := &harness{
		env:          env,
		store:        store,
		client:       cacheconsistency.NewRecordingKubeClient(env.TypedKubeClient(), store),
		configMaps:   kubeInformers.Core().V1().ConfigMaps().Lister().ConfigMaps(env.Namespace()),
		pods:         kubeInformers.Core().V1().Pods().Lister().Pods(env.Namespace()),
		statefulSets: kubeInformers.Apps().V1().StatefulSets().Lister().StatefulSets(env.Namespace()),
	}

	kubeInformers.Start(ctx.Done())
	g.DeferCleanup(kubeInformers.Shutdown)
	for informerType, synced := range kubeInformers.WaitForCacheSync(ctx.Done()) {
		o.Expect(synced).To(o.BeTrue(), "informer %v didn't sync", informerType)
	}
	o.Expect(cache.WaitForCacheSync(ctx.Done(), store.HasSynced)).To(o.BeTrue())

	return h
}

// waitReady is WaitReady with the spec's timeout.
func (h *harness) waitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, waitReadyTimeout)
	defer cancel()

	return h.store.WaitReady(ctx)
}

func newConfigMap(namespace, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string]string{"key": "value"},
	}
}

func newStatefulSet(namespace, name string) *appsv1.StatefulSet {
	labels := map[string]string{"app": name}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(1)),
			ServiceName: name,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "envtest"}},
				},
			},
		},
	}
}

func newPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "envtest"}},
		},
	}
}

func parseResourceVersion(rv string) int64 {
	g.GinkgoHelper()

	parsed, err := strconv.ParseInt(rv, 10, 64)
	o.Expect(err).NotTo(o.HaveOccurred(), "resourceVersion %q is not an integer", rv)

	return parsed
}

// The ConsistencyStore promises that after WaitReady the listers reflect every write made through the recording
// clients. The unit tests prove the bookkeeping against a fake watch; these specs prove the assumptions that
// bookkeeping rests on against a real API server: resourceVersions are comparable and observed in order, the scale
// subresource carries the StatefulSet's resourceVersion, deletes surface as deletions or deletionTimestamps, and
// server-side apply returns the written object.
var _ = g.Describe("ConsistencyStore with recording clients", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx, envtest.WithoutMonitoringCRDs(), envtest.WithoutMutatingWebhook())
	})

	g.DescribeTable("should make a ConfigMap write visible to the lister after WaitReady",
		func(ctx g.SpecContext, write func(ctx context.Context, h *harness, existing *corev1.ConfigMap) *corev1.ConfigMap) {
			h := newHarness(ctx, env)

			g.By("Creating the ConfigMap and waiting for the lister to see it")
			created, err := h.client.CoreV1().ConfigMaps(env.Namespace()).Create(ctx, newConfigMap(env.Namespace(), "cm"), metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(h.waitReady(ctx)).To(o.Succeed())

			g.By("Writing through the recording client")
			written := write(ctx, h, created)
			o.Expect(parseResourceVersion(written.ResourceVersion)).To(o.BeNumerically(">", parseResourceVersion(created.ResourceVersion)))

			g.By("Verifying the lister is still behind the write")
			cached, err := h.configMaps.Get("cm")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(cached.ResourceVersion).To(o.Equal(created.ResourceVersion))

			g.By("Verifying the lister has the write after WaitReady")
			o.Expect(h.waitReady(ctx)).To(o.Succeed())
			cached, err = h.configMaps.Get("cm")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(cached.ResourceVersion).To(o.Equal(written.ResourceVersion))
		},
		g.Entry("update", func(ctx context.Context, h *harness, existing *corev1.ConfigMap) *corev1.ConfigMap {
			updated := existing.DeepCopy()
			updated.Data["key"] = "updated"
			written, err := h.client.CoreV1().ConfigMaps(existing.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return written
		}),
		g.Entry("patch", func(ctx context.Context, h *harness, existing *corev1.ConfigMap) *corev1.ConfigMap {
			written, err := h.client.CoreV1().ConfigMaps(existing.Namespace).Patch(ctx, existing.Name, types.MergePatchType, []byte(`{"data":{"key":"patched"}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			return written
		}),
		g.Entry("server-side apply", func(ctx context.Context, h *harness, existing *corev1.ConfigMap) *corev1.ConfigMap {
			written, err := h.client.CoreV1().ConfigMaps(existing.Namespace).Apply(ctx, corev1ac.ConfigMap(existing.Name, existing.Namespace).WithData(map[string]string{"key": "applied"}), metav1.ApplyOptions{FieldManager: "envtest", Force: true})
			o.Expect(err).NotTo(o.HaveOccurred())
			return written
		}),
	)

	g.It("should make a created object visible to the lister after WaitReady", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		created, err := h.client.CoreV1().ConfigMaps(env.Namespace()).Create(ctx, newConfigMap(env.Namespace(), "cm"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = h.configMaps.Get("cm")
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected the lister to be behind the create, got: %v", err)

		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		cached, err := h.configMaps.Get("cm")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.UID).To(o.Equal(created.UID))
	})

	g.It("should make a delete visible to the lister after WaitReady", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		_, err := h.client.CoreV1().ConfigMaps(env.Namespace()).Create(ctx, newConfigMap(env.Namespace(), "cm"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		o.Expect(h.client.CoreV1().ConfigMaps(env.Namespace()).Delete(ctx, "cm", metav1.DeleteOptions{})).To(o.Succeed())

		_, err = h.configMaps.Get("cm")
		o.Expect(err).NotTo(o.HaveOccurred(), "expected the lister to be behind the delete")

		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		_, err = h.configMaps.Get("cm")
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected the lister to have dropped the object, got: %v", err)
	})

	g.It("should treat a delete of an object with a finalizer as observed once the lister sees it terminating", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		cm := newConfigMap(env.Namespace(), "cm")
		cm.Finalizers = []string{"envtest.scylladb.com/hold"}
		_, err := h.client.CoreV1().ConfigMaps(env.Namespace()).Create(ctx, cm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		o.Expect(h.client.CoreV1().ConfigMaps(env.Namespace()).Delete(ctx, "cm", metav1.DeleteOptions{})).To(o.Succeed())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		cached, err := h.configMaps.Get("cm")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.DeletionTimestamp).NotTo(o.BeNil())

		g.By("Releasing the finalizer")
		_, err = h.client.CoreV1().ConfigMaps(env.Namespace()).Patch(ctx, "cm", types.MergePatchType, []byte(`{"metadata":{"finalizers":null}}`), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should tell a recreated object apart from the deleted one by UID", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		first, err := h.client.CoreV1().ConfigMaps(env.Namespace()).Create(ctx, newConfigMap(env.Namespace(), "cm"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		o.Expect(h.client.CoreV1().ConfigMaps(env.Namespace()).Delete(ctx, "cm", metav1.DeleteOptions{})).To(o.Succeed())
		second, err := h.client.CoreV1().ConfigMaps(env.Namespace()).Create(ctx, newConfigMap(env.Namespace(), "cm"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(second.UID).NotTo(o.Equal(first.UID))

		cached, err := h.configMaps.Get("cm")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.UID).To(o.Equal(first.UID), "expected the lister to still hold the deleted instance")

		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		cached, err = h.configMaps.Get("cm")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.UID).To(o.Equal(second.UID))
	})

	g.It("should record a scale subresource write against the StatefulSet", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		created, err := h.client.AppsV1().StatefulSets(env.Namespace()).Create(ctx, newStatefulSet(env.Namespace(), "sts"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		scale, err := h.client.AppsV1().StatefulSets(env.Namespace()).UpdateScale(ctx, "sts", &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{Namespace: env.Namespace(), Name: "sts", ResourceVersion: created.ResourceVersion},
			Spec:       autoscalingv1.ScaleSpec{Replicas: 2},
		}, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scale.Name).To(o.Equal("sts"))
		o.Expect(parseResourceVersion(scale.ResourceVersion)).To(o.BeNumerically(">", parseResourceVersion(created.ResourceVersion)))

		cached, err := h.statefulSets.Get("sts")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(*cached.Spec.Replicas).To(o.BeEquivalentTo(1), "expected the lister to be behind the scale")

		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		cached, err = h.statefulSets.Get("sts")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(*cached.Spec.Replicas).To(o.BeEquivalentTo(2))
		o.Expect(cached.ResourceVersion).To(o.Equal(scale.ResourceVersion), "the scale subresource has to carry the StatefulSet's resourceVersion")
	})

	g.It("should record a status subresource write", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		created, err := h.client.AppsV1().StatefulSets(env.Namespace()).Create(ctx, newStatefulSet(env.Namespace(), "sts"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		updated := created.DeepCopy()
		updated.Status.ObservedGeneration = created.Generation
		updated.Status.Replicas = 1
		written, err := h.client.AppsV1().StatefulSets(env.Namespace()).UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		cached, err := h.statefulSets.Get("sts")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.ResourceVersion).To(o.Equal(created.ResourceVersion), "expected the lister to be behind the status write")

		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		cached, err = h.statefulSets.Get("sts")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.ResourceVersion).To(o.Equal(written.ResourceVersion))
	})

	g.It("should record an eviction as a delete", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		// With no kubelet to confirm, an evicted Pod stays terminating with a deletionTimestamp, which is what the
		// store treats as the delete being observed.
		_, err := h.client.CoreV1().Pods(env.Namespace()).Create(ctx, newPod(env.Namespace(), "pod"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(h.waitReady(ctx)).To(o.Succeed())

		o.Expect(h.client.CoreV1().Pods(env.Namespace()).EvictV1(ctx, &policyv1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: "pod"}})).To(o.Succeed())

		cached, err := h.pods.Get("pod")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cached.DeletionTimestamp).To(o.BeNil(), "expected the lister to be behind the eviction")

		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		cached, err = h.pods.Get("pod")
		if err == nil {
			o.Expect(cached.DeletionTimestamp).NotTo(o.BeNil(), "expected the lister to show the evicted pod terminating")
		} else {
			o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected the lister to have dropped the evicted pod, got: %v", err)
		}
	})

	g.It("should not wait for kinds that are not registered", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		_, err := h.client.CoreV1().Secrets(env.Namespace()).Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: env.Namespace(), Name: "secret"}}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		start := time.Now()
		o.Expect(h.waitReady(ctx)).To(o.Succeed())
		o.Expect(time.Since(start)).To(o.BeNumerically("<", informerLag/2), "expected WaitReady to return without waiting")
	})

	// A write outside the informers' scope is a misconfiguration WaitReady can't detect: the API server's watch
	// bookmarks advance the informer store's resourceVersion past the write without delivering it, so WaitReady
	// returns while the lister never sees the object. The spec pins both the bookmark-driven catch-up, which the
	// store relies on after relists too, and the limitation.
	g.It("should catch up through watch bookmarks on a write the informers can't observe", func(ctx g.SpecContext) {
		h := newHarness(ctx, env)

		g.By("Creating a ConfigMap in a namespace the informers don't watch")
		other, err := env.TypedKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "unwatched-"}}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = h.client.CoreV1().ConfigMaps(other.Name).Create(ctx, newConfigMap(other.Name, "cm"), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx, cancel := context.WithTimeout(ctx, bookmarkWaitTimeout)
		defer cancel()
		o.Expect(h.store.WaitReady(waitCtx)).To(o.Succeed())

		_, err = h.configMaps.Get("cm")
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "expected the lister to never see the object, got: %v", err)
	})
})
