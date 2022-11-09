// Copyright (c) 2022 ScyllaDB.

package resourceapply

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
)

func TestApplyGenericObject(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newScyllaDatacenter := func() *scyllav1alpha1.ScyllaDatacenter {
		return &scyllav1alpha1.ScyllaDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				Labels:    map[string]string{},
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller:         pointer.Bool(true),
						UID:                "abcdefgh",
						APIVersion:         "scylla.scylladb.com/v1",
						Kind:               "ScyllaCluster",
						Name:               "basic",
						BlockOwnerDeletion: pointer.Bool(true),
					},
				},
			},
			Spec: scyllav1alpha1.ScyllaDatacenterSpec{
				NodesPerRack: pointer.Int32(1),
			},
		}
	}

	newScyllaDatacenterWithHash := func() *scyllav1alpha1.ScyllaDatacenter {
		sd := newScyllaDatacenter()
		utilruntime.Must(SetHashAnnotation(sd))
		return sd
	}

	tt := []struct {
		name                     string
		existing                 []runtime.Object
		cache                    []runtime.Object // nil cache means autofill from the client
		required                 *scyllav1alpha1.ScyllaDatacenter
		expectedScyllaDatacenter *scyllav1alpha1.ScyllaDatacenter
		expectedChanged          bool
		expectedErr              error
		expectedEvents           []string
	}{
		{
			name:                     "creates a new scylladatacenter when there is none",
			existing:                 nil,
			required:                 newScyllaDatacenter(),
			expectedScyllaDatacenter: newScyllaDatacenterWithHash(),
			expectedChanged:          true,
			expectedErr:              nil,
			expectedEvents:           []string{"Normal ScyllaDatacenterCreated ScyllaDatacenter default/test created"},
		},
		{
			name: "does nothing if the same scylladatacenter already exists",
			existing: []runtime.Object{
				newScyllaDatacenterWithHash(),
			},
			required:                 newScyllaDatacenter(),
			expectedScyllaDatacenter: newScyllaDatacenterWithHash(),
			expectedChanged:          false,
			expectedErr:              nil,
			expectedEvents:           nil,
		},
		{
			name: "does nothing if the same scylladatacenter already exists and required one has the hash",
			existing: []runtime.Object{
				newScyllaDatacenterWithHash(),
			},
			required:                 newScyllaDatacenterWithHash(),
			expectedScyllaDatacenter: newScyllaDatacenterWithHash(),
			expectedChanged:          false,
			expectedErr:              nil,
			expectedEvents:           nil,
		},
		{
			name: "updates the scylladatacenter if it exists without the hash",
			existing: []runtime.Object{
				newScyllaDatacenter(),
			},
			required:                 newScyllaDatacenter(),
			expectedScyllaDatacenter: newScyllaDatacenterWithHash(),
			expectedChanged:          true,
			expectedErr:              nil,
			expectedEvents:           []string{"Normal ScyllaDatacenterUpdated ScyllaDatacenter default/test updated"},
		},
		{
			name: "updates the scylladatacenter number of nodes differ",
			existing: []runtime.Object{
				newScyllaDatacenter(),
			},
			required: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Spec.NodesPerRack = pointer.Int32(42)
				return sd
			}(),
			expectedScyllaDatacenter: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Spec.NodesPerRack = pointer.Int32(42)
				utilruntime.Must(SetHashAnnotation(sd))
				return sd
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ScyllaDatacenterUpdated ScyllaDatacenter default/test updated"},
		},
		{
			name: "updates the scylladatacenter if labels differ",
			existing: []runtime.Object{
				newScyllaDatacenterWithHash(),
			},
			required: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Labels["foo"] = "bar"
				return sd
			}(),
			expectedScyllaDatacenter: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(sd))
				return sd
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ScyllaDatacenterUpdated ScyllaDatacenter default/test updated"},
		},
		{
			name: "won't update the scylladatacenter if an admission changes the object",
			existing: []runtime.Object{
				func() *scyllav1alpha1.ScyllaDatacenter {
					sd := newScyllaDatacenterWithHash()
					// Simulate admission by changing a value after the hash is computed.
					sd.Spec.NodesPerRack = pointer.Int32(42)
					return sd
				}(),
			},
			required: newScyllaDatacenter(),
			expectedScyllaDatacenter: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenterWithHash()
				// Simulate admission by changing a value after the hash is computed.
				sd.Spec.NodesPerRack = pointer.Int32(42)
				return sd
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *scyllav1alpha1.ScyllaDatacenter {
					sd := newScyllaDatacenterWithHash()
					sd.ResourceVersion = "21"
					return sd
				}(),
			},
			required: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.ResourceVersion = ""
				sd.Labels["foo"] = "bar"
				return sd
			}(),
			expectedScyllaDatacenter: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.ResourceVersion = "21"
				sd.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(sd))
				return sd
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ScyllaDatacenterUpdated ScyllaDatacenter default/test updated"},
		},
		{
			name:     "update fails if the scylladatacenter is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newScyllaDatacenterWithHash(),
			},
			required: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Labels["foo"] = "bar"
				return sd
			}(),
			expectedScyllaDatacenter: nil,
			expectedChanged:          false,
			expectedErr:              fmt.Errorf("can't update *v1alpha1.ScyllaDatacenter: %w", apierrors.NewNotFound(scyllav1alpha1.Resource("scylladatacenters"), "test")),
			expectedEvents:           []string{`Warning UpdateScyllaDatacenterFailed Failed to update ScyllaDatacenter default/test: scylladatacenters.scylla.scylladb.com "test" not found`},
		},
		{
			name: "all label and annotation keys are kept when the hash matches",
			existing: []runtime.Object{
				func() *scyllav1alpha1.ScyllaDatacenter {
					sd := newScyllaDatacenter()
					sd.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "",
					}
					sd.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "",
					}
					utilruntime.Must(SetHashAnnotation(sd))
					sd.Annotations["a-1"] = "a-alpha-changed"
					sd.Annotations["a-3"] = "a-resurrected"
					sd.Annotations["a-custom"] = "custom-value"
					sd.Labels["l-1"] = "l-alpha-changed"
					sd.Labels["l-3"] = "l-resurrected"
					sd.Labels["l-custom"] = "custom-value"
					return sd
				}(),
			},
			required: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				sd.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				return sd
			}(),
			expectedScyllaDatacenter: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				sd.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(sd))
				sd.Annotations["a-1"] = "a-alpha-changed"
				sd.Annotations["a-3"] = "a-resurrected"
				sd.Annotations["a-custom"] = "custom-value"
				sd.Labels["l-1"] = "l-alpha-changed"
				sd.Labels["l-3"] = "l-resurrected"
				sd.Labels["l-custom"] = "custom-value"
				return sd
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "only managed label and annotation keys are updated when the hash changes",
			existing: []runtime.Object{
				func() *scyllav1alpha1.ScyllaDatacenter {
					sd := newScyllaDatacenter()
					sd.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "a-resurrected",
					}
					sd.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "l-resurrected",
					}
					utilruntime.Must(SetHashAnnotation(sd))
					sd.Annotations["a-1"] = "a-alpha-changed"
					sd.Annotations["a-custom"] = "a-custom-value"
					sd.Labels["l-1"] = "l-alpha-changed"
					sd.Labels["l-custom"] = "l-custom-value"
					return sd
				}(),
			},
			required: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				sd.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				return sd
			}(),
			expectedScyllaDatacenter: func() *scyllav1alpha1.ScyllaDatacenter {
				sd := newScyllaDatacenter()
				sd.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				sd.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(sd))
				delete(sd.Annotations, "a-3-")
				sd.Annotations["a-custom"] = "a-custom-value"
				delete(sd.Labels, "l-3-")
				sd.Labels["l-custom"] = "l-custom-value"
				return sd
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ScyllaDatacenterUpdated ScyllaDatacenter default/test updated"},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			existingObjects := make([]runtime.Object, 0, len(tc.existing))
			for _, obj := range tc.existing {
				asUnstructured := &unstructured.Unstructured{}
				if err := scheme.Scheme.Convert(obj, asUnstructured, nil); err != nil {
					t.Fatal(err)
				}
				existingObjects = append(existingObjects, asUnstructured)
			}
			client := fake.NewSimpleDynamicClient(scheme.Scheme, existingObjects...)
			// ApplyScyllaDatacenter needs to be reentrant so running it the second time should give the same results.
			// (One of the common mistakes is editing the object after computing the hash so it differs the second time.)
			iterations := 2
			if tc.expectedErr != nil {
				iterations = 1
			}
			for i := 0; i < iterations; i++ {
				t.Run("", func(t *testing.T) {
					ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer ctxCancel()

					recorder := record.NewFakeRecorder(10)

					objCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					gvr := schema.GroupVersionResource{Group: "scylla.scylladb.com", Version: "v1alpha1", Resource: "scylladatacenters"}
					scylladatacenterLister := dynamiclister.NewRuntimeObjectShim(dynamiclister.New(objCache, gvr))

					if tc.cache != nil {
						for _, obj := range tc.cache {
							asUnstructured := &unstructured.Unstructured{}
							if err := scheme.Scheme.Convert(obj, asUnstructured, nil); err != nil {
								t.Fatal(err)
							}
							err := objCache.Add(asUnstructured)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						scylladatacenterList, err := client.Resource(gvr).List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range scylladatacenterList.Items {
							err := objCache.Add(&scylladatacenterList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotObj, gotChanged, gotErr := ApplyGenericObject(ctx, client.Resource(gvr), scylladatacenterLister, recorder, tc.required)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotObj, tc.expectedScyllaDatacenter) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedScyllaDatacenter, gotObj, cmp.Diff(tc.expectedScyllaDatacenter, gotObj))
					}

					// Make sure such object was actually created.
					if gotObj != nil {
						createdObjRaw, err := client.Resource(gvr).Namespace(gotObj.Namespace).Get(ctx, gotObj.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						createdObj := &scyllav1alpha1.ScyllaDatacenter{}
						err = scheme.Scheme.Convert(createdObjRaw, createdObj, nil)
						if err != nil {
							t.Fatal(err)
						}
						if !equality.Semantic.DeepEqual(createdObj, gotObj) {
							t.Errorf("created and returned objs differ:\n%s", cmp.Diff(createdObj, gotObj))
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
