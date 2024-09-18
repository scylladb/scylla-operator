// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestReleaseObjects(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name         string
		existing     []runtime.Object
		selector     labels.Selector
		controller   *corev1.ConfigMap
		expectedPods []corev1.Pod
	}{
		{
			name: "releases object owned by provided controller and matching selector",
			existing: []runtime.Object{
				func() *corev1.Pod {
					return &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "matching-pod",
							Namespace: "ns",
							Labels: map[string]string{
								"foo": "bar",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion:         "",
									Kind:               "ConfigMap",
									Name:               "controller",
									UID:                "controller-uid",
									Controller:         pointer.Ptr(true),
									BlockOwnerDeletion: pointer.Ptr(true),
								},
							},
						},
					}
				}(),
				func() *corev1.Pod {
					return &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "non-controlled-pod-matching-selector",
							Namespace: "ns",
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					}
				}(),
				func() *corev1.Pod {
					return &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "non-matching-selector-pod",
							Namespace: "ns",
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion:         "",
									Kind:               "ConfigMap",
									Name:               "controller",
									UID:                "controller-uid",
									Controller:         pointer.Ptr(true),
									BlockOwnerDeletion: pointer.Ptr(true),
								},
							},
						},
					}
				}(),
				func() *corev1.Pod {
					return &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "matching-selector-pod-owned-by-someone-else",
							Namespace: "ns",
							Labels: map[string]string{
								"foo": "bar",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion:         "",
									Kind:               "ConfigMap",
									Name:               "foo",
									UID:                "foo-uid",
									Controller:         pointer.Ptr(true),
									BlockOwnerDeletion: pointer.Ptr(true),
								},
							},
						},
					}
				}(),
			},
			selector: labels.SelectorFromSet(map[string]string{
				"foo": "bar",
			}),
			controller: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "controller",
					Namespace: "ns",
					UID:       "controller-uid",
				},
			},
			expectedPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "matching-pod",
						Namespace: "ns",
						Labels: map[string]string{
							"foo": "bar",
						},
						OwnerReferences: nil,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "matching-selector-pod-owned-by-someone-else",
						Namespace: "ns",
						Labels: map[string]string{
							"foo": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "",
								Kind:               "ConfigMap",
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "non-controlled-pod-matching-selector",
						Namespace: "ns",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "non-matching-selector-pod",
						Namespace: "ns",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "",
								Kind:               "ConfigMap",
								Name:               "controller",
								UID:                "controller-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			controllerGVK := corev1.SchemeGroupVersion.WithKind("ConfigMap")

			client := fake.NewSimpleClientset(tc.existing...)
			ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ctxCancel()

			podCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, obj := range tc.existing {
				err := podCache.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}

			podLister := corev1listers.NewPodLister(podCache)

			control := ControlleeManagerGetObjectsFuncs[*corev1.ConfigMap, *corev1.Pod]{
				GetControllerUncachedFunc: client.CoreV1().ConfigMaps("ns").Get,
				ListObjectsFunc:           podLister.Pods("ns").List,
				PatchObjectFunc:           client.CoreV1().Pods("ns").Patch,
			}

			err := ReleaseObjects(ctx, tc.controller, controllerGVK, tc.selector, control)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			client.Tracker()

			gotPods, err := client.CoreV1().Pods("ns").List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("can't list pods: %v", err)
			}
			if !equality.Semantic.DeepEqual(tc.expectedPods, gotPods.Items) {
				t.Errorf("patched and expected pods differ:\n%s", cmp.Diff(tc.expectedPods, gotPods.Items))
			}
		})
	}
}
