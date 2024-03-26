package resourceapply

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func TestApplyStatefulSet(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newSts := func() *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller:         pointer.Ptr(true),
						UID:                "abcdefgh",
						APIVersion:         "scylla.scylladb.com/v1",
						Kind:               "ScyllaCluster",
						Name:               "basic",
						BlockOwnerDeletion: pointer.Ptr(true),
					},
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointer.Ptr(int32(3)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "scylla",
								Image: "scylladb/scylla:latest",
							},
						},
					},
				},
			},
		}
	}

	newStsWithHash := func() *appsv1.StatefulSet {
		sts := newSts()
		utilruntime.Must(SetHashAnnotation(sts))
		return sts
	}

	tt := []struct {
		name            string
		existing        []runtime.Object
		cache           []runtime.Object // nil cache means autofill from the client
		required        *appsv1.StatefulSet
		forceOwnership  bool
		expectedSts     *appsv1.StatefulSet
		expectedChanged bool
		expectedErr     error
		expectedEvents  []string
	}{
		{
			name:            "creates a new sts when there is none",
			existing:        nil,
			required:        newSts(),
			expectedSts:     newStsWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetCreated StatefulSet default/test created"},
		},
		{
			name: "does nothing if the same sts already exists",
			existing: []runtime.Object{
				newStsWithHash(),
			},
			required:        newSts(),
			expectedSts:     newStsWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "does nothing if the same sts already exists and required one has the hash",
			existing: []runtime.Object{
				newStsWithHash(),
			},
			required:        newStsWithHash(),
			expectedSts:     newStsWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "updates the sts if it exists without the hash",
			existing: []runtime.Object{
				newSts(),
			},
			required:        newSts(),
			expectedSts:     newStsWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name:     "fails to create the sts without a controllerRef",
			existing: nil,
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.OwnerReferences = nil
				return sts
			}(),
			expectedSts:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`apps/v1, Kind=StatefulSet "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the sts if replicas differ",
			existing: []runtime.Object{
				newSts(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Replicas = pointer.Ptr(*sts.Spec.Replicas + 1)
				return sts
			}(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Replicas = pointer.Ptr(*sts.Spec.Replicas + 1)
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name: "updates the sts if labels differ",
			existing: []runtime.Object{
				newStsWithHash(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Labels["foo"] = "bar"
				return sts
			}(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name: "updates the sts if the an image differs",
			existing: []runtime.Object{
				newStsWithHash(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name: "won't update the sts if an admission changes the sts",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newStsWithHash()
					// Simulate admission by changing a value after the hash is computed.
					sts.Spec.Template.Spec.Containers[0].Image += "-admissionchange"
					return sts
				}(),
			},
			required: newSts(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newStsWithHash()
				// Simulate admission by changing a value after the hash is computed.
				sts.Spec.Template.Spec.Containers[0].Image += "-admissionchange"
				return sts
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newStsWithHash()
					sts.ResourceVersion = "21"
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.ResourceVersion = ""
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.ResourceVersion = "21"
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name:     "update fails if the sts is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newStsWithHash(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			expectedSts:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`can't update apps/v1, Kind=StatefulSet "default/test": %w`, apierrors.NewNotFound(appsv1.Resource("statefulsets"), "test")),
			expectedEvents:  []string{`Warning UpdateStatefulSetFailed Failed to update StatefulSet default/test: statefulsets.apps "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(sts))
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			expectedSts:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`apps/v1, Kind=StatefulSet "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateStatefulSetFailed Failed to update StatefulSet default/test: apps/v1, Kind=StatefulSet "default/test" isn't controlled by us`},
		},
		{
			name: "forced update succeeds if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(sts))
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			forceOwnership: true,
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(sts))
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				return sts
			}(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(sts))
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			expectedSts:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`apps/v1, Kind=StatefulSet "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateStatefulSetFailed Failed to update StatefulSet default/test: apps/v1, Kind=StatefulSet "default/test" isn't controlled by us`},
		},
		{
			name: "forced update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(sts))
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return sts
			}(),
			forceOwnership:  true,
			expectedSts:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`apps/v1, Kind=StatefulSet "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateStatefulSetFailed Failed to update StatefulSet default/test: apps/v1, Kind=StatefulSet "default/test" isn't controlled by us`},
		},
		{
			name: "all label and annotation keys are kept when the hash matches",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "",
					}
					sts.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "",
					}
					utilruntime.Must(SetHashAnnotation(sts))
					sts.Annotations["a-1"] = "a-alpha-changed"
					sts.Annotations["a-3"] = "a-resurrected"
					sts.Annotations["a-custom"] = "custom-value"
					sts.Labels["l-1"] = "l-alpha-changed"
					sts.Labels["l-3"] = "l-resurrected"
					sts.Labels["l-custom"] = "custom-value"
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				sts.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				return sts
			}(),
			forceOwnership: false,
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				sts.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(sts))
				sts.Annotations["a-1"] = "a-alpha-changed"
				sts.Annotations["a-3"] = "a-resurrected"
				sts.Annotations["a-custom"] = "custom-value"
				sts.Labels["l-1"] = "l-alpha-changed"
				sts.Labels["l-3"] = "l-resurrected"
				sts.Labels["l-custom"] = "custom-value"
				return sts
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "only managed label and annotation keys are updated when the hash changes",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "a-resurrected",
					}
					sts.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "l-resurrected",
					}
					utilruntime.Must(SetHashAnnotation(sts))
					sts.Annotations["a-1"] = "a-alpha-changed"
					sts.Annotations["a-custom"] = "a-custom-value"
					sts.Labels["l-1"] = "l-alpha-changed"
					sts.Labels["l-custom"] = "l-custom-value"
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				sts.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				return sts
			}(),
			forceOwnership: true,
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				sts.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(sts))
				delete(sts.Annotations, "a-3-")
				sts.Annotations["a-custom"] = "a-custom-value"
				delete(sts.Labels, "l-3-")
				sts.Labels["l-custom"] = "l-custom-value"
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal StatefulSetUpdated StatefulSet default/test updated"},
		},
		{
			name: "deletes and creates the StatefulSet when selector is changed and still matches the old pods",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.Spec.Template.Labels = map[string]string{
						"foo": "bar",
						"bar": "foo",
					}
					sts.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
							"bar": "foo",
						},
					}
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Labels = map[string]string{
					"foo": "bar",
					"bar": "foo",
				}
				sts.Spec.Selector = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				}
				return sts
			}(),
			expectedSts: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Labels = map[string]string{
					"foo": "bar",
					"bar": "foo",
				}
				sts.Spec.Selector = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				}
				utilruntime.Must(SetHashAnnotation(sts))
				return sts
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents: []string{
				"Normal StatefulSetDeleted StatefulSet default/test deleted",
				"Normal StatefulSetCreated StatefulSet default/test created",
			},
		},
		{
			name: "apply fails when StatefulSet selector differs and existing Pod labels doesn't match new selector",
			existing: []runtime.Object{
				func() *appsv1.StatefulSet {
					sts := newSts()
					sts.Spec.Template.Labels = map[string]string{
						"foo": "bar",
					}
					sts.Spec.Selector = &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					}
					return sts
				}(),
			},
			required: func() *appsv1.StatefulSet {
				sts := newSts()
				sts.Spec.Template.Labels = map[string]string{
					"foo": "bar",
					"bar": "foo",
				}
				sts.Spec.Selector = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
						"bar": "foo",
					},
				}
				return sts
			}(),
			expectedSts:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't get recreate reason: %w", fmt.Errorf(`required StatefulSet selector "bar=foo,foo=bar" doesn't match existing Pod Labels set map[foo:bar]`)),
			expectedEvents:  nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state, so it has to persist the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyStatefulSet needs to be reentrant so running it the second time should give the same results.
			// (One of the common mistakes is editing the object after computing the hash so it differs the second time.)
			iterations := 2
			if tc.expectedErr != nil {
				iterations = 1
			}
			for i := range iterations {
				t.Run("", func(t *testing.T) {
					ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer ctxCancel()

					recorder := record.NewFakeRecorder(10)

					stsCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					stsLister := appsv1listers.NewStatefulSetLister(stsCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := stsCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						stsList, err := client.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range stsList.Items {
							err := stsCache.Add(&stsList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotSts, gotChanged, gotErr := ApplyStatefulSet(ctx, client.AppsV1(), stsLister, recorder, tc.required, ApplyOptions{
						ForceOwnership: tc.forceOwnership,
					})
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotSts, tc.expectedSts) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedSts, gotSts, cmp.Diff(tc.expectedSts, gotSts))
					}

					// Make sure such object was actually created.
					if gotSts != nil {
						createdSts, err := client.AppsV1().StatefulSets(gotSts.Namespace).Get(ctx, gotSts.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSts, gotSts) {
							t.Errorf("created and returned statefulsets differ:\n%s", cmp.Diff(createdSts, gotSts))
						}
					}

					if i == 0 {
						if gotChanged != tc.expectedChanged {
							t.Errorf("expected %t, got %t", tc.expectedChanged, gotChanged)
						}
					} else {
						if gotChanged {
							t.Errorf("object changed in iteration %d", i)
						}
					}

					close(recorder.Events)
					var gotEvents []string
					for e := range recorder.Events {
						gotEvents = append(gotEvents, e)
					}
					if i == 0 {
						if !reflect.DeepEqual(gotEvents, tc.expectedEvents) {
							t.Errorf("expected %v, got %v, diff:\n%s", tc.expectedEvents, gotEvents, cmp.Diff(tc.expectedEvents, gotEvents))
						}
					} else {
						if len(gotEvents) > 0 {
							t.Errorf("unexpected events: %v", gotEvents)
						}
					}
				})
			}
		})
	}
}

func TestApplyDaemonSet(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newDS := func() *appsv1.DaemonSet {
		return &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller:         pointer.Ptr(true),
						UID:                "abcdefgh",
						APIVersion:         "scylla.scylladb.com/v1",
						Kind:               "ScyllaCluster",
						Name:               "basic",
						BlockOwnerDeletion: pointer.Ptr(true),
					},
				},
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: metav1.SetAsLabelSelector(map[string]string{}),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "scylla",
								Image: "scylladb/scylla:latest",
							},
						},
					},
				},
			},
		}
	}

	newDsWithHash := func() *appsv1.DaemonSet {
		ds := newDS()
		utilruntime.Must(SetHashAnnotation(ds))
		return ds
	}

	tt := []struct {
		name              string
		existing          []runtime.Object
		cache             []runtime.Object // nil cache means autofill from the client
		required          *appsv1.DaemonSet
		expectedDaemonSet *appsv1.DaemonSet
		expectedChanged   bool
		expectedErr       error
		expectedEvents    []string
	}{
		{
			name:              "creates a new ds when there is none",
			existing:          nil,
			required:          newDS(),
			expectedDaemonSet: newDsWithHash(),
			expectedChanged:   true,
			expectedErr:       nil,
			expectedEvents:    []string{"Normal DaemonSetCreated DaemonSet default/test created"},
		},
		{
			name: "does nothing if the same ds already exists",
			existing: []runtime.Object{
				newDsWithHash(),
			},
			required:          newDS(),
			expectedDaemonSet: newDsWithHash(),
			expectedChanged:   false,
			expectedErr:       nil,
			expectedEvents:    nil,
		},
		{
			name: "does nothing if the same ds already exists and required one has the hash",
			existing: []runtime.Object{
				newDsWithHash(),
			},
			required:          newDsWithHash(),
			expectedDaemonSet: newDsWithHash(),
			expectedChanged:   false,
			expectedErr:       nil,
			expectedEvents:    nil,
		},
		{
			name: "updates the ds if it exists without the hash",
			existing: []runtime.Object{
				newDS(),
			},
			required:          newDS(),
			expectedDaemonSet: newDsWithHash(),
			expectedChanged:   true,
			expectedErr:       nil,
			expectedEvents:    []string{"Normal DaemonSetUpdated DaemonSet default/test updated"},
		},
		{
			name:     "fails to create the ds without a controllerRef",
			existing: nil,
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.OwnerReferences = nil
				return ds
			}(),
			expectedDaemonSet: nil,
			expectedChanged:   false,
			expectedErr:       fmt.Errorf(`apps/v1, Kind=DaemonSet "default/test" is missing controllerRef`),
			expectedEvents:    nil,
		},
		{
			name: "updates the sts if template differ",
			existing: []runtime.Object{
				newDS(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Spec.Template.Spec.Containers[0].Image = "differentimage:latest"
				return ds
			}(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Spec.Template.Spec.Containers[0].Image = "differentimage:latest"
				utilruntime.Must(SetHashAnnotation(ds))
				return ds
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal DaemonSetUpdated DaemonSet default/test updated"},
		},
		{
			name: "updates the ds if labels differ",
			existing: []runtime.Object{
				newDsWithHash(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Labels["foo"] = "bar"
				return ds
			}(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(ds))
				return ds
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal DaemonSetUpdated DaemonSet default/test updated"},
		},
		{
			name: "won't update the ds if an admission changes the sts",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					ds := newDsWithHash()
					// Simulate admission by changing a value after the hash is computed.
					ds.Spec.Template.Spec.Containers[0].Image += "-admissionchange"
					return ds
				}(),
			},
			required: newDS(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDsWithHash()
				// Simulate admission by changing a value after the hash is computed.
				ds.Spec.Template.Spec.Containers[0].Image += "-admissionchange"
				return ds
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					ds := newDsWithHash()
					ds.ResourceVersion = "21"
					return ds
				}(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.ResourceVersion = ""
				ds.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return ds
			}(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.ResourceVersion = "21"
				ds.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				utilruntime.Must(SetHashAnnotation(ds))
				return ds
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal DaemonSetUpdated DaemonSet default/test updated"},
		},
		{
			name:     "update fails if the ds is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newDsWithHash(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return ds
			}(),
			expectedDaemonSet: nil,
			expectedChanged:   false,
			expectedErr:       fmt.Errorf(`can't update apps/v1, Kind=DaemonSet "default/test": %w`, apierrors.NewNotFound(appsv1.Resource("daemonsets"), "test")),
			expectedEvents:    []string{`Warning UpdateDaemonSetFailed Failed to update DaemonSet default/test: daemonsets.apps "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					sts := newDS()
					sts.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(sts))
					return sts
				}(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return ds
			}(),
			expectedDaemonSet: nil,
			expectedChanged:   false,
			expectedErr:       fmt.Errorf(`apps/v1, Kind=DaemonSet "default/test" isn't controlled by us`),
			expectedEvents:    []string{`Warning UpdateDaemonSetFailed Failed to update DaemonSet default/test: apps/v1, Kind=DaemonSet "default/test" isn't controlled by us`},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					ds := newDS()
					ds.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(ds))
					return ds
				}(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				return ds
			}(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDS()
				utilruntime.Must(SetHashAnnotation(ds))
				return ds
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal DaemonSetUpdated DaemonSet default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					ds := newDS()
					ds.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(ds))
					return ds
				}(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Spec.Template.Spec.Containers[0].Image += "-rc.0"
				return ds
			}(),
			expectedDaemonSet: nil,
			expectedChanged:   false,
			expectedErr:       fmt.Errorf(`apps/v1, Kind=DaemonSet "default/test" isn't controlled by us`),
			expectedEvents:    []string{`Warning UpdateDaemonSetFailed Failed to update DaemonSet default/test: apps/v1, Kind=DaemonSet "default/test" isn't controlled by us`},
		},
		{
			name: "all label and annotation keys are kept when the hash matches",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					ds := newDS()
					ds.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "",
					}
					ds.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "",
					}
					utilruntime.Must(SetHashAnnotation(ds))
					ds.Annotations["a-1"] = "a-alpha-changed"
					ds.Annotations["a-3"] = "a-resurrected"
					ds.Annotations["a-custom"] = "custom-value"
					ds.Labels["l-1"] = "l-alpha-changed"
					ds.Labels["l-3"] = "l-resurrected"
					ds.Labels["l-custom"] = "custom-value"
					return ds
				}(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				ds.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				return ds
			}(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				ds.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(ds))
				ds.Annotations["a-1"] = "a-alpha-changed"
				ds.Annotations["a-3"] = "a-resurrected"
				ds.Annotations["a-custom"] = "custom-value"
				ds.Labels["l-1"] = "l-alpha-changed"
				ds.Labels["l-3"] = "l-resurrected"
				ds.Labels["l-custom"] = "custom-value"
				return ds
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "only managed label and annotation keys are updated when the hash changes",
			existing: []runtime.Object{
				func() *appsv1.DaemonSet {
					ds := newDS()
					ds.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "a-resurrected",
					}
					ds.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "l-resurrected",
					}
					utilruntime.Must(SetHashAnnotation(ds))
					ds.Annotations["a-1"] = "a-alpha-changed"
					ds.Annotations["a-custom"] = "a-custom-value"
					ds.Labels["l-1"] = "l-alpha-changed"
					ds.Labels["l-custom"] = "l-custom-value"
					return ds
				}(),
			},
			required: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				ds.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				return ds
			}(),
			expectedDaemonSet: func() *appsv1.DaemonSet {
				ds := newDS()
				ds.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				ds.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(ds))
				delete(ds.Annotations, "a-3-")
				ds.Annotations["a-custom"] = "a-custom-value"
				delete(ds.Labels, "l-3-")
				ds.Labels["l-custom"] = "l-custom-value"
				return ds
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal DaemonSetUpdated DaemonSet default/test updated"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyStatefulSet needs to be reentrant so running it the second time should give the same results.
			// (One of the common mistakes is editing the object after computing the hash so it differs the second time.)
			iterations := 2
			if tc.expectedErr != nil {
				iterations = 1
			}
			for i := range iterations {
				t.Run("", func(t *testing.T) {
					ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer ctxCancel()

					recorder := record.NewFakeRecorder(10)

					dsCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					dsLister := appsv1listers.NewDaemonSetLister(dsCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := dsCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						dsList, err := client.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range dsList.Items {
							err := dsCache.Add(&dsList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotDs, gotChanged, gotErr := ApplyDaemonSet(ctx, client.AppsV1(), dsLister, recorder, tc.required, ApplyOptions{})
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotDs, tc.expectedDaemonSet) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedDaemonSet, gotDs, cmp.Diff(tc.expectedDaemonSet, gotDs))
					}

					// Make sure such object was actually created.
					if gotDs != nil {
						createdDs, err := client.AppsV1().DaemonSets(gotDs.Namespace).Get(ctx, gotDs.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdDs, gotDs) {
							t.Errorf("created and returned daemonset differ:\n%s", cmp.Diff(createdDs, gotDs))
						}
					}

					if i == 0 {
						if gotChanged != tc.expectedChanged {
							t.Errorf("expected %t, got %t", tc.expectedChanged, gotChanged)
						}
					} else {
						if gotChanged {
							t.Errorf("object changed in iteration %d", i)
						}
					}

					close(recorder.Events)
					var gotEvents []string
					for e := range recorder.Events {
						gotEvents = append(gotEvents, e)
					}
					if i == 0 {
						if !reflect.DeepEqual(gotEvents, tc.expectedEvents) {
							t.Errorf("expected %v, got %v, diff:\n%s", tc.expectedEvents, gotEvents, cmp.Diff(tc.expectedEvents, gotEvents))
						}
					} else {
						if len(gotEvents) > 0 {
							t.Errorf("unexpected events: %v", gotEvents)
						}
					}
				})
			}
		})
	}
}
