package resourceapply

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
)

func TestApplyPodDisruptionBudget(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newPDB := func() *policyv1.PodDisruptionBudget {
		return &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				Labels:    map[string]string{},
				OwnerReferences: []metav1.OwnerReference{
					{
						Controller:         pointer.BoolPtr(true),
						UID:                "abcdefgh",
						APIVersion:         "scylla.scylladb.com/v1",
						Kind:               "ScyllaCluster",
						Name:               "basic",
						BlockOwnerDeletion: pointer.BoolPtr(true),
					},
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{},
				},
			},
		}
	}

	newPDBWithHash := func() *policyv1.PodDisruptionBudget {
		pdb := newPDB()
		utilruntime.Must(SetHashAnnotation(pdb))
		return pdb
	}

	tt := []struct {
		name            string
		existing        []runtime.Object
		cache           []runtime.Object // nil cache means autofill from the client
		required        *policyv1.PodDisruptionBudget
		forceOwnership  bool
		expectedPDB     *policyv1.PodDisruptionBudget
		expectedChanged bool
		expectedErr     error
		expectedEvents  []string
	}{
		{
			name:            "creates a new pdb when there is none",
			existing:        nil,
			required:        newPDB(),
			expectedPDB:     newPDBWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetCreated PodDisruptionBudget default/test created"},
		},
		{
			name: "does nothing if the same pdb already exists",
			existing: []runtime.Object{
				newPDBWithHash(),
			},
			required:        newPDB(),
			expectedPDB:     newPDBWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "does nothing if the same pdb already exists and required one has the hash",
			existing: []runtime.Object{
				newPDBWithHash(),
			},
			required:        newPDBWithHash(),
			expectedPDB:     newPDBWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "updates the pdb if it exists without the hash",
			existing: []runtime.Object{
				newPDB(),
			},
			required:        newPDB(),
			expectedPDB:     newPDBWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetUpdated PodDisruptionBudget default/test updated"},
		},
		{
			name:     "fails to create the pdb without a controllerRef",
			existing: nil,
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.OwnerReferences = nil
				return pdb
			}(),
			expectedPDB:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`poddisruptionbudget "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the pdb selector differs",
			existing: []runtime.Object{
				newPDB(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Spec.Selector.MatchLabels["foo"] = "bar"
				return pdb
			}(),
			expectedPDB: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Spec.Selector.MatchLabels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(pdb))
				return pdb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetUpdated PodDisruptionBudget default/test updated"},
		},
		{
			name: "updates the pdb if labels differ",
			existing: []runtime.Object{
				newPDBWithHash(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			expectedPDB: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(pdb))
				return pdb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetUpdated PodDisruptionBudget default/test updated"},
		},
		{
			name: "won't update the pdb if an admission changes the sts",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDBWithHash()
					// Simulate admission by changing a value after the hash is computed.
					pdb.Spec.Selector.MatchLabels["foo"] = "admissionchange"
					return pdb
				}(),
			},
			required: newPDB(),
			expectedPDB: func() *policyv1.PodDisruptionBudget {
				pdb := newPDBWithHash()
				// Simulate admission by changing a value after the hash is computed.
				pdb.Spec.Selector.MatchLabels["foo"] = "admissionchange"
				return pdb
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDBWithHash()
					pdb.ResourceVersion = "21"
					return pdb
				}(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.ResourceVersion = ""
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			expectedPDB: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.ResourceVersion = "21"
				pdb.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(pdb))
				return pdb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetUpdated PodDisruptionBudget default/test updated"},
		},
		{
			name:     "update fails if the pdb is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newPDBWithHash(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			expectedPDB:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update pdb: %w", apierrors.NewNotFound(policyv1.Resource("poddisruptionbudgets"), "test")),
			expectedEvents:  []string{`Warning UpdatePodDisruptionBudgetFailed Failed to update PodDisruptionBudget default/test: poddisruptionbudgets.policy "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDB()
					pdb.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(pdb))
					return pdb
				}(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			expectedPDB:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`poddisruptionbudget "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdatePodDisruptionBudgetFailed Failed to update PodDisruptionBudget default/test: poddisruptionbudget "default/test" isn't controlled by us`},
		},
		{
			name: "forced update succeeds if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDB()
					pdb.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(pdb))
					return pdb
				}(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			forceOwnership: true,
			expectedPDB: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(pdb))
				return pdb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetUpdated PodDisruptionBudget default/test updated"},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDB()
					pdb.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(pdb))
					return pdb
				}(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				return pdb
			}(),
			expectedPDB: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				utilruntime.Must(SetHashAnnotation(pdb))
				return pdb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal PodDisruptionBudgetUpdated PodDisruptionBudget default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDB()
					pdb.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(pdb))
					return pdb
				}(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			expectedPDB:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`poddisruptionbudget "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdatePodDisruptionBudgetFailed Failed to update PodDisruptionBudget default/test: poddisruptionbudget "default/test" isn't controlled by us`},
		},
		{
			name: "forced update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *policyv1.PodDisruptionBudget {
					pdb := newPDB()
					pdb.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(pdb))
					return pdb
				}(),
			},
			required: func() *policyv1.PodDisruptionBudget {
				pdb := newPDB()
				pdb.Labels["foo"] = "bar"
				return pdb
			}(),
			forceOwnership:  true,
			expectedPDB:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`poddisruptionbudget "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdatePodDisruptionBudgetFailed Failed to update PodDisruptionBudget default/test: poddisruptionbudget "default/test" isn't controlled by us`},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyPodDisruptionBudget needs to be reentrant so running it the second time should give the same results.
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

					pdbCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					pdbLister := policyv1listers.NewPodDisruptionBudgetLister(pdbCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := pdbCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						pdbList, err := client.PolicyV1().PodDisruptionBudgets("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range pdbList.Items {
							err := pdbCache.Add(&pdbList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotSts, gotChanged, gotErr := ApplyPodDisruptionBudget(ctx, client.PolicyV1(), pdbLister, recorder, tc.required, tc.forceOwnership)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotSts, tc.expectedPDB) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedPDB, gotSts, cmp.Diff(tc.expectedPDB, gotSts))
					}

					// Make sure such object was actually created.
					if gotSts != nil {
						createdSts, err := client.PolicyV1().PodDisruptionBudgets(gotSts.Namespace).Get(ctx, gotSts.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSts, gotSts) {
							t.Errorf("created and returned pdbs differ:\n%s", cmp.Diff(createdSts, gotSts))
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
