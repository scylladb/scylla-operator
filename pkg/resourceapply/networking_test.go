package resourceapply

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func TestApplyIngress(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newIngress := func() *networkingv1.Ingress {
		return &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				Labels:    map[string]string{},
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
			Spec: networkingv1.IngressSpec{
				DefaultBackend: &networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: "some-service",
						Port: networkingv1.ServiceBackendPort{
							Number: 443,
						},
					},
				},
			},
		}
	}

	newIngressWithHash := func() *networkingv1.Ingress {
		ingress := newIngress()
		utilruntime.Must(SetHashAnnotation(ingress))
		return ingress
	}

	tt := []struct {
		name            string
		existing        []runtime.Object
		cache           []runtime.Object // nil cache means autofill from the client
		required        *networkingv1.Ingress
		expectedIngress *networkingv1.Ingress
		expectedChanged bool
		expectedErr     error
		expectedEvents  []string
	}{
		{
			name:            "creates a new ingress when there is none",
			existing:        nil,
			required:        newIngress(),
			expectedIngress: newIngressWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal IngressCreated Ingress default/test created"},
		},
		{
			name: "does nothing if the same ingress already exists",
			existing: []runtime.Object{
				newIngressWithHash(),
			},
			required:        newIngress(),
			expectedIngress: newIngressWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "does nothing if the same ingress already exists and required one has the hash",
			existing: []runtime.Object{
				newIngressWithHash(),
			},
			required:        newIngressWithHash(),
			expectedIngress: newIngressWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "updates the ingress if it exists without the hash",
			existing: []runtime.Object{
				newIngress(),
			},
			required:        newIngress(),
			expectedIngress: newIngressWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal IngressUpdated Ingress default/test updated"},
		},
		{
			name:     "fails to create the ingress without a controllerRef",
			existing: nil,
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.OwnerReferences = nil
				return ingress
			}(),
			expectedIngress: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`networking.k8s.io/v1, Kind=Ingress "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the ingress default backend differs",
			existing: []runtime.Object{
				newIngress(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Spec.DefaultBackend.Service.Port.Number = 42
				return ingress
			}(),
			expectedIngress: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Spec.DefaultBackend.Service.Port.Number = 42
				utilruntime.Must(SetHashAnnotation(ingress))
				return ingress
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal IngressUpdated Ingress default/test updated"},
		},
		{
			name: "updates the ingress if labels differ",
			existing: []runtime.Object{
				newIngressWithHash(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Labels["foo"] = "bar"
				return ingress
			}(),
			expectedIngress: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(ingress))
				return ingress
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal IngressUpdated Ingress default/test updated"},
		},
		{
			name: "won't update the ingress if an admission changes the sts",
			existing: []runtime.Object{
				func() *networkingv1.Ingress {
					ingress := newIngressWithHash()
					// Simulate admission by changing a value after the hash is computed.
					ingress.Spec.DefaultBackend.Service.Port.Number = 42
					return ingress
				}(),
			},
			required: newIngress(),
			expectedIngress: func() *networkingv1.Ingress {
				ingress := newIngressWithHash()
				// Simulate admission by changing a value after the hash is computed.
				ingress.Spec.DefaultBackend.Service.Port.Number = 42
				return ingress
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *networkingv1.Ingress {
					ingress := newIngressWithHash()
					ingress.ResourceVersion = "21"
					return ingress
				}(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.ResourceVersion = ""
				ingress.Labels["foo"] = "bar"
				return ingress
			}(),
			expectedIngress: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.ResourceVersion = "21"
				ingress.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(ingress))
				return ingress
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal IngressUpdated Ingress default/test updated"},
		},
		{
			name:     "update fails if the ingress is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newIngressWithHash(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Labels["foo"] = "bar"
				return ingress
			}(),
			expectedIngress: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`can't update networking.k8s.io/v1, Kind=Ingress "default/test": %w`, apierrors.NewNotFound(networkingv1.Resource("ingresses"), "test")),
			expectedEvents:  []string{`Warning UpdateIngressFailed Failed to update Ingress default/test: ingresses.networking.k8s.io "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *networkingv1.Ingress {
					ingress := newIngress()
					ingress.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(ingress))
					return ingress
				}(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Labels["foo"] = "bar"
				return ingress
			}(),
			expectedIngress: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`networking.k8s.io/v1, Kind=Ingress "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateIngressFailed Failed to update Ingress default/test: networking.k8s.io/v1, Kind=Ingress "default/test" isn't controlled by us`},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *networkingv1.Ingress {
					ingress := newIngress()
					ingress.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(ingress))
					return ingress
				}(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Labels["foo"] = "bar"
				return ingress
			}(),
			expectedIngress: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`networking.k8s.io/v1, Kind=Ingress "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateIngressFailed Failed to update Ingress default/test: networking.k8s.io/v1, Kind=Ingress "default/test" isn't controlled by us`},
		},
		{
			name: "all label and annotation keys are kept when the hash matches",
			existing: []runtime.Object{
				func() *networkingv1.Ingress {
					ingress := newIngress()
					ingress.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "",
					}
					ingress.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "",
					}
					utilruntime.Must(SetHashAnnotation(ingress))
					ingress.Annotations["a-1"] = "a-alpha-changed"
					ingress.Annotations["a-3"] = "a-resurrected"
					ingress.Annotations["a-custom"] = "custom-value"
					ingress.Labels["l-1"] = "l-alpha-changed"
					ingress.Labels["l-3"] = "l-resurrected"
					ingress.Labels["l-custom"] = "custom-value"
					return ingress
				}(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				ingress.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				return ingress
			}(),
			expectedIngress: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Annotations = map[string]string{
					"a-1":  "a-alpha",
					"a-2":  "a-beta",
					"a-3-": "",
				}
				ingress.Labels = map[string]string{
					"l-1":  "l-alpha",
					"l-2":  "l-beta",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(ingress))
				ingress.Annotations["a-1"] = "a-alpha-changed"
				ingress.Annotations["a-3"] = "a-resurrected"
				ingress.Annotations["a-custom"] = "custom-value"
				ingress.Labels["l-1"] = "l-alpha-changed"
				ingress.Labels["l-3"] = "l-resurrected"
				ingress.Labels["l-custom"] = "custom-value"
				return ingress
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "only managed label and annotation keys are updated when the hash changes",
			existing: []runtime.Object{
				func() *networkingv1.Ingress {
					ingress := newIngress()
					ingress.Annotations = map[string]string{
						"a-1":  "a-alpha",
						"a-2":  "a-beta",
						"a-3-": "a-resurrected",
					}
					ingress.Labels = map[string]string{
						"l-1":  "l-alpha",
						"l-2":  "l-beta",
						"l-3-": "l-resurrected",
					}
					utilruntime.Must(SetHashAnnotation(ingress))
					ingress.Annotations["a-1"] = "a-alpha-changed"
					ingress.Annotations["a-custom"] = "a-custom-value"
					ingress.Labels["l-1"] = "l-alpha-changed"
					ingress.Labels["l-custom"] = "l-custom-value"
					return ingress
				}(),
			},
			required: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				ingress.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				return ingress
			}(),
			expectedIngress: func() *networkingv1.Ingress {
				ingress := newIngress()
				ingress.Annotations = map[string]string{
					"a-1":  "a-alpha-x",
					"a-2":  "a-beta-x",
					"a-3-": "",
				}
				ingress.Labels = map[string]string{
					"l-1":  "l-alpha-x",
					"l-2":  "l-beta-x",
					"l-3-": "",
				}
				utilruntime.Must(SetHashAnnotation(ingress))
				delete(ingress.Annotations, "a-3-")
				ingress.Annotations["a-custom"] = "a-custom-value"
				delete(ingress.Labels, "l-3-")
				ingress.Labels["l-custom"] = "l-custom-value"
				return ingress
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal IngressUpdated Ingress default/test updated"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyIngress needs to be reentrant so running it the second time should give the same results.
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

					ingressCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					ingressLister := networkingv1listers.NewIngressLister(ingressCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := ingressCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						ingressList, err := client.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range ingressList.Items {
							err := ingressCache.Add(&ingressList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotObj, gotChanged, gotErr := ApplyIngress(ctx, client.NetworkingV1(), ingressLister, recorder, tc.required, ApplyOptions{})
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotObj, tc.expectedIngress) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedIngress, gotObj, cmp.Diff(tc.expectedIngress, gotObj))
					}

					// Make sure such object was actually created.
					if gotObj != nil {
						createdSts, err := client.NetworkingV1().Ingresses(gotObj.Namespace).Get(ctx, gotObj.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSts, gotObj) {
							t.Errorf("created and returned pdbs differ:\n%s", cmp.Diff(createdSts, gotObj))
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
