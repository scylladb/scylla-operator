// Copyright (c) 2024 ScyllaDB.

package informers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/remoteclient/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	clusterOne = "cluster-1"
	clusterTwo = "cluster-2"
)

func TestDynamicSharedInformerFactory_EventHandlers(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		existingObj *corev1.Pod
		namespace   string
		trigger     func(namespace string, fakeClient kubernetes.Interface, testObject *corev1.Pod) *corev1.Pod
		handler     func(rcvCh chan<- *corev1.Pod) *cache.ResourceEventHandlerFuncs
	}{
		{
			name:        "creating an object triggers AddFunc",
			existingObj: nil,
			namespace:   "ns",
			trigger: func(namespace string, fakeClient kubernetes.Interface, testObj *corev1.Pod) *corev1.Pod {
				createdObj, err := fakeClient.CoreV1().Pods(namespace).Create(context.TODO(), testObj, metav1.CreateOptions{})
				if err != nil {
					t.Error(err)
				}
				return createdObj
			},
			handler: func(rcvCh chan<- *corev1.Pod) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					AddFunc: func(obj any) {
						rcvCh <- obj.(*corev1.Pod)
					},
				}
			},
		},
		{
			name:        "updating an object triggers UpdateFunc",
			existingObj: newPod("ns", "name", "foo"),
			namespace:   "ns",
			trigger: func(namespace string, fakeClient kubernetes.Interface, testObject *corev1.Pod) *corev1.Pod {
				testObject.Spec.Hostname = "bar"
				updatedObj, err := fakeClient.CoreV1().Pods(namespace).Update(context.TODO(), testObject, metav1.UpdateOptions{})
				if err != nil {
					t.Error(err)
				}
				return updatedObj
			},
			handler: func(rcvCh chan<- *corev1.Pod) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					UpdateFunc: func(obj, updatedObj any) {
						rcvCh <- updatedObj.(*corev1.Pod)
					},
				}
			},
		},
		{
			name:        "deleting an object triggers DeleteFunc",
			existingObj: newPod("ns", "name", "foo"),
			namespace:   "ns",
			trigger: func(namespace string, fakeClient kubernetes.Interface, testObject *corev1.Pod) *corev1.Pod {
				err := fakeClient.CoreV1().Pods(namespace).Delete(context.TODO(), testObject.GetName(), metav1.DeleteOptions{})
				if err != nil {
					t.Error(err)
				}
				return testObject
			},
			handler: func(rcvCh chan<- *corev1.Pod) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					DeleteFunc: func(obj any) {
						rcvCh <- obj.(*corev1.Pod)
					},
				}
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var objs []runtime.Object
			if tc.existingObj != nil {
				objs = append(objs, tc.existingObj)
			}

			clusterClient := client.NewClusterClient(func(_ []byte) (kubernetes.Interface, error) {
				return fake.NewSimpleClientset(objs...), nil
			})

			sharedFactory := NewSharedInformerFactory[kubernetes.Interface](clusterClient, 0)

			informerObjCh := make(chan *corev1.Pod, 2)
			eventHandler := tc.handler(informerObjCh)

			unboundInformer := sharedFactory.ForResource(&corev1.Pod{}, ClusterListWatch[kubernetes.Interface]{
				ListFunc: func(client client.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
					return func(options metav1.ListOptions) (runtime.Object, error) {
						cc, err := client.Cluster(cluster)
						if err != nil {
							return nil, err
						}
						return cc.CoreV1().Pods(ns).List(ctx, options)
					}
				},
				WatchFunc: func(client client.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
					return func(options metav1.ListOptions) (watch.Interface, error) {
						cc, err := client.Cluster(cluster)
						if err != nil {
							return nil, err
						}
						return cc.CoreV1().Pods(ns).Watch(ctx, options)
					}
				},
			})
			unboundInformer.Informer().AddEventHandler(eventHandler)

			sharedFactory.Start(ctx.Done())

			if err := clusterClient.UpdateCluster(clusterOne, nil); err != nil {
				t.Fatal(err)
			}

			if err := sharedFactory.UpdateCluster(clusterOne, nil); err != nil {
				t.Fatal(err)
			}

			podType := reflect.TypeOf(&corev1.Pod{})
			if synced := sharedFactory.WaitForCacheSync(ctx.Done()); !synced[podType] {
				t.Fatalf("informer for *corev1.Pod not synced, synced ones are: %v", synced)
			}

			cc, err := clusterClient.Cluster(clusterOne)
			if err != nil {
				t.Fatal(err)
			}
			pod := newPod(tc.namespace, "name", "foo")
			testObject := tc.trigger(tc.namespace, cc, pod)

			select {
			case informerObj := <-informerObjCh:
				if !equality.Semantic.DeepEqual(testObject, informerObj) {
					t.Fatalf("expected %s, got: %s", testObject, informerObj)
				}
			case <-ctx.Done():
				t.Fatalf("informer hasn't received an object within timeout")
			}
		})
	}
}

func TestDynamicSharedInformerFactory_EventHandles_MultipleClusters(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testObject := newPod("ns", "name", "foo")

	remoteClient := client.NewClusterClient(func(_ []byte) (kubernetes.Interface, error) {
		return fake.NewSimpleClientset(testObject), nil
	})

	sharedFactory := NewSharedInformerFactory[kubernetes.Interface](remoteClient, 0)

	podInformer := sharedFactory.ForResource(&corev1.Pod{}, ClusterListWatch[kubernetes.Interface]{
		ListFunc: func(client client.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.ListFunc {
			return func(options metav1.ListOptions) (runtime.Object, error) {
				clusterClient, err := client.Cluster(cluster)
				if err != nil {
					return nil, err
				}
				return clusterClient.CoreV1().Pods(ns).List(ctx, options)
			}
		},
		WatchFunc: func(client client.ClusterClientInterface[kubernetes.Interface], cluster, ns string) cache.WatchFunc {
			return func(options metav1.ListOptions) (watch.Interface, error) {
				clusterClient, err := client.Cluster(cluster)
				if err != nil {
					return nil, err
				}
				return clusterClient.CoreV1().Pods(ns).Watch(ctx, options)
			}
		},
	})

	informerUpdateObjCh := make(chan *corev1.Pod, 2)
	podInformer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(obj, updatedObj any) {
			informerUpdateObjCh <- updatedObj.(*corev1.Pod)
		},
	})

	sharedFactory.Start(ctx.Done())

	tss := []struct {
		cluster                 string
		expectedInformerObjects []*corev1.Pod
	}{
		{
			cluster: clusterOne,
			expectedInformerObjects: []*corev1.Pod{
				newPod("ns", "name", "cluster-1-updated-0"),
			},
		},
		{
			cluster: clusterTwo,
			expectedInformerObjects: []*corev1.Pod{
				newPod("ns", "name", "cluster-2-updated-1"),
			},
		},
		{
			cluster: clusterOne,
			expectedInformerObjects: []*corev1.Pod{
				newPod("ns", "name", "cluster-1-updated-2"),
			},
		},
	}

	for i, ts := range tss {
		if err := remoteClient.UpdateCluster(ts.cluster, nil); err != nil {
			t.Fatal(err)
		}

		if err := sharedFactory.UpdateCluster(ts.cluster, nil); err != nil {
			t.Fatal(err)
		}

		podType := reflect.TypeOf(&corev1.Pod{})
		if synced := sharedFactory.WaitForCacheSync(ctx.Done()); !synced[podType] {
			t.Fatalf("informer for *corev1.Pod not synced, synced ones are: %v", synced)
		}

		clusterClient, err := remoteClient.Cluster(ts.cluster)
		if err != nil {
			t.Fatal(err)
		}

		if synced := sharedFactory.WaitForCacheSync(ctx.Done()); !synced[podType] {
			t.Fatalf("informer for *corev1.Pod not synced, synced ones are: %v", synced)
		}

		testObjectCopy := testObject.DeepCopy()
		testObjectCopy.Spec.Hostname = fmt.Sprintf("%s-updated-%d", ts.cluster, i)

		_, err = clusterClient.CoreV1().Pods(testObject.GetNamespace()).Update(ctx, testObjectCopy, metav1.UpdateOptions{})
		if err != nil {
			t.Error(err)
		}

		observedObjects := drainChannel(ctx, informerUpdateObjCh, time.Second)
		if !equality.Semantic.DeepEqual(ts.expectedInformerObjects, observedObjects) {
			t.Fatalf("expected %v, got: %v, diff: %s", ts.expectedInformerObjects, observedObjects, cmp.Diff(ts.expectedInformerObjects, observedObjects))
		}
	}
}

func drainChannel[T any](ctx context.Context, ch <-chan T, timeout time.Duration) []T {
	var observedObjects []T
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case obj := <-ch:
			observedObjects = append(observedObjects, obj)
		case <-ctx.Done():
			return observedObjects
		}
	}
}

func newPod(namespace, name, hostname string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Hostname: hostname,
		},
	}
}
