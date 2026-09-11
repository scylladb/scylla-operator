package cacheconsistency

import (
	"context"
	"testing"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllafake "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func newService(rv string, uid types.UID) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            testName,
			UID:             uid,
			ResourceVersion: rv,
		},
	}
}

// recordingKubeClientTestEnv runs Service and StatefulSet informers fed by fake watchers behind a recording client over
// a fake clientset.
type recordingKubeClientTestEnv struct {
	client             kubernetes.Interface
	store              *ConsistencyStore
	serviceWatcher     *watch.RaceFreeFakeWatcher
	statefulSetWatcher *watch.RaceFreeFakeWatcher
}

func newRecordingKubeClientTestEnv(t *testing.T, services []*corev1.Service, statefulSets []*appsv1.StatefulSet) *recordingKubeClientTestEnv {
	t.Helper()

	var objects []runtime.Object

	serviceList := &corev1.ServiceList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}}
	for _, svc := range services {
		serviceList.Items = append(serviceList.Items, *svc)
		objects = append(objects, svc)
	}
	statefulSetList := &appsv1.StatefulSetList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}}
	for _, sts := range statefulSets {
		statefulSetList.Items = append(statefulSetList.Items, *sts)
		objects = append(objects, sts)
	}

	serviceInformer, serviceWatcher := newTestSharedInformer(t, &corev1.Service{}, serviceList)
	statefulSetInformer, statefulSetWatcher := newTestSharedInformer(t, &appsv1.StatefulSet{}, statefulSetList)

	store := NewConsistencyStore()
	if err := store.Register(&corev1.Service{}, serviceInformer); err != nil {
		t.Fatalf("can't register services: %v", err)
	}
	if err := store.Register(&appsv1.StatefulSet{}, statefulSetInformer); err != nil {
		t.Fatalf("can't register statefulsets: %v", err)
	}
	runTestInformer(t, serviceInformer)
	runTestInformer(t, statefulSetInformer)
	if !cache.WaitForCacheSync(t.Context().Done(), store.HasSynced) {
		t.Fatal("can't sync the consistency store")
	}

	return &recordingKubeClientTestEnv{
		client:             NewRecordingKubeClient(fake.NewSimpleClientset(objects...), store),
		store:              store,
		serviceWatcher:     serviceWatcher,
		statefulSetWatcher: statefulSetWatcher,
	}
}

func expectStoreWaitBlocks(t *testing.T, store *ConsistencyStore) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	err := store.WaitReady(ctx)
	if err == nil {
		t.Fatal("expected wait to block until the deadline")
	}
}

func expectStoreWaitReturns(t *testing.T, store *ConsistencyStore) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := store.WaitReady(ctx)
	if err != nil {
		t.Fatalf("expected wait to return, got: %v", err)
	}
}

func TestKubeClient_UpdateIsRecorded(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, []*corev1.Service{newService("5", testUID)}, nil)

	// The fake clientset stores the object as given, so the resourceVersion stands in for the one the API server
	// would assign.
	_, err := env.client.CoreV1().Services(testNamespace).Update(t.Context(), newService("20", testUID), metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("can't update service: %v", err)
	}
	expectStoreWaitBlocks(t, env.store)

	env.serviceWatcher.Modify(newService("20", testUID))
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_CreateIsRecorded(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, nil, nil)

	_, err := env.client.CoreV1().Services(testNamespace).Create(t.Context(), newService("20", testUID), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("can't create service: %v", err)
	}
	expectStoreWaitBlocks(t, env.store)

	env.serviceWatcher.Add(newService("20", testUID))
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_DeleteIsRecordedByTheCachedInstance(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, []*corev1.Service{newService("5", testUID)}, nil)

	err := env.client.CoreV1().Services(testNamespace).Delete(t.Context(), testName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("can't delete service: %v", err)
	}
	expectStoreWaitBlocks(t, env.store)

	env.serviceWatcher.Delete(newService("30", testUID))
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_DeleteOfAnObjectMissingFromTheAPIServerIsRecorded(t *testing.T) {
	t.Parallel()

	// The cache still holds the object, the API server doesn't.
	env := newRecordingKubeClientTestEnv(t, []*corev1.Service{newService("5", testUID)}, nil)
	err := env.client.CoreV1().Services(testNamespace).Delete(t.Context(), testName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("can't delete service: %v", err)
	}
	env.serviceWatcher.Delete(newService("30", testUID))
	expectStoreWaitReturns(t, env.store)

	// Recording the recreated instance as a write makes the wait return only once the cache holds it.
	env.serviceWatcher.Add(newService("40", "uid-2"))
	env.store.WroteAt(corev1.SchemeGroupVersion.WithKind("Service"), testNamespace, testName, "40")
	expectStoreWaitReturns(t, env.store)

	err = env.client.CoreV1().Services(testNamespace).Delete(t.Context(), testName, metav1.DeleteOptions{})
	if err == nil {
		t.Fatal("expected a not found error")
	}
	expectStoreWaitBlocks(t, env.store)

	env.serviceWatcher.Delete(newService("40", "uid-2"))
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_DeleteOfAnObjectMissingFromTheCacheIsNotRecorded(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, nil, nil)

	err := env.client.CoreV1().Services(testNamespace).Delete(t.Context(), testName, metav1.DeleteOptions{})
	if err == nil {
		t.Fatal("expected a not found error")
	}
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_UpdateScaleIsRecordedAgainstTheStatefulSet(t *testing.T) {
	t.Parallel()

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            testName,
			UID:             testUID,
			ResourceVersion: "5",
		},
	}
	env := newRecordingKubeClientTestEnv(t, nil, []*appsv1.StatefulSet{sts})

	scale := &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            testName,
			ResourceVersion: "20",
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 1},
	}
	_, err := env.client.AppsV1().StatefulSets(testNamespace).UpdateScale(t.Context(), testName, scale, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("can't update scale: %v", err)
	}
	expectStoreWaitBlocks(t, env.store)

	scaled := sts.DeepCopy()
	scaled.ResourceVersion = "20"
	env.statefulSetWatcher.Modify(scaled)
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_UnregisteredKindsPassThrough(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, nil, nil)

	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "pvc", ResourceVersion: "20"}}
	_, err := env.client.CoreV1().PersistentVolumeClaims(testNamespace).Create(t.Context(), pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("can't create pvc: %v", err)
	}
	err = env.client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(t.Context(), "pvc", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("can't delete pvc: %v", err)
	}
	err = env.client.CoreV1().Pods(testNamespace).EvictV1(t.Context(), &policyv1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: "pod"}})
	if err == nil {
		t.Fatal("expected a not found error")
	}

	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_NilStoreIsNoop(t *testing.T) {
	t.Parallel()

	client := NewRecordingKubeClient(fake.NewSimpleClientset(newService("5", testUID)), nil)

	_, err := client.CoreV1().Services(testNamespace).Update(t.Context(), newService("20", testUID), metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("can't update service: %v", err)
	}
	err = client.CoreV1().Services(testNamespace).Delete(t.Context(), testName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("can't delete service: %v", err)
	}
}

func TestKubeClient_PatchIsRecorded(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, []*corev1.Service{newService("5", testUID)}, nil)

	// The fake clientset applies the patch to the stored object, so patching the resourceVersion stands in for the
	// one the API server would assign.
	_, err := env.client.CoreV1().Services(testNamespace).Patch(t.Context(), testName, types.MergePatchType, []byte(`{"metadata":{"resourceVersion":"20"}}`), metav1.PatchOptions{})
	if err != nil {
		t.Fatalf("can't patch service: %v", err)
	}
	expectStoreWaitBlocks(t, env.store)

	env.serviceWatcher.Modify(newService("20", testUID))
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_UpdateStatusIsRecorded(t *testing.T) {
	t.Parallel()

	env := newRecordingKubeClientTestEnv(t, []*corev1.Service{newService("5", testUID)}, nil)

	_, err := env.client.CoreV1().Services(testNamespace).UpdateStatus(t.Context(), newService("20", testUID), metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("can't update service status: %v", err)
	}
	expectStoreWaitBlocks(t, env.store)

	env.serviceWatcher.Modify(newService("20", testUID))
	expectStoreWaitReturns(t, env.store)
}

func TestKubeClient_EvictionIsRecordedAsDelete(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            testName,
			UID:             testUID,
			ResourceVersion: "5",
		},
	}
	podList := &corev1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: []corev1.Pod{*pod}}
	podInformer, podWatcher := newTestSharedInformer(t, &corev1.Pod{}, podList)

	store := NewConsistencyStore()
	if err := store.Register(&corev1.Pod{}, podInformer); err != nil {
		t.Fatalf("can't register pods: %v", err)
	}
	runTestInformer(t, podInformer, store.HasSynced)

	client := NewRecordingKubeClient(fake.NewSimpleClientset(pod), store)
	err := client.CoreV1().Pods(testNamespace).EvictV1(t.Context(), &policyv1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: testName}})
	if err != nil {
		t.Fatalf("can't evict pod: %v", err)
	}
	expectStoreWaitBlocks(t, store)

	terminating := pod.DeepCopy()
	terminating.ResourceVersion = "30"
	terminating.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	podWatcher.Modify(terminating)
	expectStoreWaitReturns(t, store)
}

func TestScyllaClient_UpdateStatusIsRecorded(t *testing.T) {
	t.Parallel()

	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       testNamespace,
			Name:            testName,
			UID:             testUID,
			ResourceVersion: "5",
		},
	}
	sdcList := &scyllav1alpha1.ScyllaDBDatacenterList{ListMeta: metav1.ListMeta{ResourceVersion: "10"}, Items: []scyllav1alpha1.ScyllaDBDatacenter{*sdc}}
	sdcInformer, sdcWatcher := newTestSharedInformer(t, &scyllav1alpha1.ScyllaDBDatacenter{}, sdcList)

	store := NewConsistencyStore()
	if err := store.Register(&scyllav1alpha1.ScyllaDBDatacenter{}, sdcInformer); err != nil {
		t.Fatalf("can't register ScyllaDBDatacenters: %v", err)
	}
	runTestInformer(t, sdcInformer, store.HasSynced)

	client := NewRecordingScyllaV1alpha1Client(scyllafake.NewSimpleClientset(sdc).ScyllaV1alpha1(), store)
	updated := sdc.DeepCopy()
	updated.ResourceVersion = "20"
	_, err := client.ScyllaDBDatacenters(testNamespace).UpdateStatus(t.Context(), updated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("can't update ScyllaDBDatacenter status: %v", err)
	}
	expectStoreWaitBlocks(t, store)

	sdcWatcher.Modify(updated)
	expectStoreWaitReturns(t, store)
}
