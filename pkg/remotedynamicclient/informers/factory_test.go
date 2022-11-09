// Copyright (c) 2022 ScyllaDB.

package informers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/remotedynamicclient/client"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	regionOne = "region-1"
	regionTwo = "region-2"
)

var (
	gvr = schema.GroupVersionResource{
		Group:    "test",
		Version:  "v1beta1",
		Resource: "testobjects",
	}

	gvk = schema.GroupVersionKind{
		Group:   "test",
		Version: "v1beta1",
		Kind:    "TestObject",
	}
)

func TestDynamicSharedInformerFactory_EventHandlers(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		existingObj *unstructured.Unstructured
		namespace   string
		trigger     func(gvr schema.GroupVersionResource, namespace string, fakeClient dynamic.Interface, testObject *unstructured.Unstructured) *unstructured.Unstructured
		handler     func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs
	}{
		{
			name:        "adding an object triggers AddFunc",
			existingObj: nil,
			namespace:   "ns",
			trigger: func(gvr schema.GroupVersionResource, namespace string, fakeClient dynamic.Interface, _ *unstructured.Unstructured) *unstructured.Unstructured {
				testObject := newUnstructured("test/v1beta1", "TestObject", "ns", "name", "spec")
				createdObj, err := fakeClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), testObject, metav1.CreateOptions{})
				if err != nil {
					t.Error(err)
				}
				return createdObj
			},
			handler: func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					AddFunc: func(obj any) {
						rcvCh <- obj.(*unstructured.Unstructured)
					},
				}
			},
		},
		{
			name:        "updating an object triggers UpdateFunc",
			existingObj: newUnstructured("test/v1beta1", "TestObject", "ns", "name", "spec"),
			namespace:   "ns",
			trigger: func(gvr schema.GroupVersionResource, namespace string, fakeClient dynamic.Interface, testObject *unstructured.Unstructured) *unstructured.Unstructured {
				testObject.Object["spec"] = "updated"
				updatedObj, err := fakeClient.Resource(gvr).Namespace(namespace).Update(context.TODO(), testObject, metav1.UpdateOptions{})
				if err != nil {
					t.Error(err)
				}
				return updatedObj
			},
			handler: func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					UpdateFunc: func(obj, updatedObj any) {
						rcvCh <- updatedObj.(*unstructured.Unstructured)
					},
				}
			},
		},
		{
			name:        "deleting an object triggers DeleteFunc",
			existingObj: newUnstructured("test/v1beta1", "TestObject", "ns", "name", "spec"),
			namespace:   "ns",
			trigger: func(gvr schema.GroupVersionResource, namespace string, fakeClient dynamic.Interface, testObject *unstructured.Unstructured) *unstructured.Unstructured {
				err := fakeClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), testObject.GetName(), metav1.DeleteOptions{})
				if err != nil {
					t.Error(err)
				}
				return testObject
			},
			handler: func(rcvCh chan<- *unstructured.Unstructured) *cache.ResourceEventHandlerFuncs {
				return &cache.ResourceEventHandlerFuncs{
					DeleteFunc: func(obj any) {
						rcvCh <- obj.(*unstructured.Unstructured)
					},
				}
			},
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			scheme := runtime.NewScheme()

			var objs []runtime.Object
			if tc.existingObj != nil {
				objs = append(objs, tc.existingObj)
			}

			remoteClient := client.NewRemoteDynamicClient(client.WithCustomClientFactory(func(_ []byte) (dynamic.Interface, error) {
				gvrToListKind := map[schema.GroupVersionResource]string{
					gvr: fmt.Sprintf("%sList", gvk.Kind),
				}
				return fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...), nil
			}))

			dynamicFactory := NewSharedInformerFactory(*scheme, remoteClient, 0)

			informerObjCh := make(chan *unstructured.Unstructured, 2)
			eventHandler := tc.handler(informerObjCh)

			unboundInformer := dynamicFactory.ForResource(gvr, gvk)
			unboundInformer.Informer().AddEventHandler(eventHandler)

			dynamicFactory.Start(ctx.Done())

			if err := remoteClient.Update(regionOne, nil); err != nil {
				t.Fatal(err)
			}

			if err := dynamicFactory.Update(regionOne, nil); err != nil {
				t.Fatal(err)
			}

			if synced := dynamicFactory.WaitForCacheSync(ctx.Done()); !synced[gvr] {
				t.Fatalf("informer for %s not synced, synced ones are: %v", gvr, synced)
			}

			regionClient, err := remoteClient.Region(regionOne)
			if err != nil {
				t.Fatal(err)
			}

			testObject := tc.trigger(gvr, tc.namespace, regionClient, tc.existingObj)

			select {
			case informerObj := <-informerObjCh:
				if !equality.Semantic.DeepEqual(testObject, informerObj) {
					t.Fatalf("expected %s, got: %s", testObject, informerObj)
				}
			case <-ctx.Done():
				t.Fatalf("informer haven't received an object within timeout")
			}
		})
	}
}

func TestDynamicSharedInformerFactory_EventHandles_MultipleRegions(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	scheme := runtime.NewScheme()

	testObject := newUnstructured("test/v1beta1", "TestObject", "ns", "name", "spec")

	remoteClient := client.NewRemoteDynamicClient(client.WithCustomClientFactory(func(_ []byte) (dynamic.Interface, error) {
		gvrToListKind := map[schema.GroupVersionResource]string{
			gvr: fmt.Sprintf("%sList", gvk.Kind),
		}
		return fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, testObject), nil
	}))

	dynamicFactory := NewSharedInformerFactory(*scheme, remoteClient, 0)

	informerUpdateObjCh := make(chan *unstructured.Unstructured, 2)

	unboundInformer := dynamicFactory.ForResource(gvr, gvk)
	unboundInformer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(obj, updatedObj any) {
			informerUpdateObjCh <- updatedObj.(*unstructured.Unstructured)
		},
	})

	dynamicFactory.Start(ctx.Done())

	tss := []struct {
		region                  string
		expectedInformerObjects []*unstructured.Unstructured
	}{
		{
			region: "region-1",
			expectedInformerObjects: []*unstructured.Unstructured{
				newUnstructured("test/v1beta1", "TestObject", "ns", "name", "region-1-updated-0"),
			},
		},
		{
			region: "region-2",
			expectedInformerObjects: []*unstructured.Unstructured{
				newUnstructured("test/v1beta1", "TestObject", "ns", "name", "region-2-updated-1"),
			},
		},
		{
			region: "region-1",
			expectedInformerObjects: []*unstructured.Unstructured{
				newUnstructured("test/v1beta1", "TestObject", "ns", "name", "region-1-updated-2"),
			},
		},
	}

	for i, ts := range tss {
		if err := remoteClient.Update(ts.region, nil); err != nil {
			t.Fatal(err)
		}

		if err := dynamicFactory.Update(ts.region, nil); err != nil {
			t.Fatal(err)
		}

		if synced := dynamicFactory.WaitForCacheSync(ctx.Done()); !synced[gvr] {
			t.Fatalf("informer for %s not synced, synced ones are: %v", gvr, synced)
		}

		regionClient, err := remoteClient.Region(ts.region)
		if err != nil {
			t.Fatal(err)
		}

		if synced := dynamicFactory.WaitForCacheSync(ctx.Done()); !synced[gvr] {
			t.Fatalf("informer for %s not synced, synced ones are: %v", gvr, synced)
		}

		testObjectCopy := testObject.DeepCopy()
		testObjectCopy.Object["spec"] = fmt.Sprintf("%s-updated-%d", ts.region, i)

		_, err = regionClient.Resource(gvr).Namespace(testObject.GetNamespace()).Update(context.TODO(), testObjectCopy, metav1.UpdateOptions{})
		if err != nil {
			t.Error(err)
		}

		var observedObjects []*unstructured.Unstructured
		collectTimeout := time.NewTimer(time.Second)
		defer collectTimeout.Stop()

	drain:
		for {
			select {
			case informerObj := <-informerUpdateObjCh:
				observedObjects = append(observedObjects, informerObj)
			case <-collectTimeout.C:
				break drain
			case <-ctx.Done():
				t.Fatalf("informer haven't received an object within timeout")
			}
		}

		if !equality.Semantic.DeepEqual(ts.expectedInformerObjects, observedObjects) {
			t.Fatalf("expected %v, got: %v, diff: %s", ts.expectedInformerObjects, observedObjects, cmp.Diff(ts.expectedInformerObjects, observedObjects))
		}
	}
}

func newUnstructured(apiVersion, kind, namespace, name, spec string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": spec,
		},
	}
}
