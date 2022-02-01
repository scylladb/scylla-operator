package resourceapply

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
)

func TestApplyService(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newService := func() *corev1.Service {
		return &corev1.Service{
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
			Spec: corev1.ServiceSpec{},
		}
	}

	newServiceWithHash := func() *corev1.Service {
		svc := newService()
		utilruntime.Must(SetHashAnnotation(svc))
		return svc
	}

	tt := []struct {
		name            string
		existing        []runtime.Object
		cache           []runtime.Object // nil cache means autofill from the client
		required        *corev1.Service
		forceOwnership  bool
		expectedService *corev1.Service
		expectedChanged bool
		expectedErr     error
		expectedEvents  []string
	}{
		{
			name:            "creates a new service when there is none",
			existing:        nil,
			required:        newService(),
			expectedService: newServiceWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceCreated Service default/test created"},
		},
		{
			name: "does nothing if the same service already exists",
			existing: []runtime.Object{
				newServiceWithHash(),
			},
			required:        newService(),
			expectedService: newServiceWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "does nothing if the same service already exists and required one has the hash",
			existing: []runtime.Object{
				newServiceWithHash(),
			},
			required:        newServiceWithHash(),
			expectedService: newServiceWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "updates the service if it exists without the hash",
			existing: []runtime.Object{
				newService(),
			},
			required:        newService(),
			expectedService: newServiceWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceUpdated Service default/test updated"},
		},
		{
			name:     "fails to create the service without a controllerRef",
			existing: nil,
			required: func() *corev1.Service {
				svc := newService()
				svc.OwnerReferences = nil
				return svc
			}(),
			expectedService: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`service "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the service if ports differ",
			existing: []runtime.Object{
				newService(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
					Name: "https",
				})
				return svc
			}(),
			expectedService: func() *corev1.Service {
				svc := newService()
				svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
					Name: "https",
				})
				utilruntime.Must(SetHashAnnotation(svc))
				return svc
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceUpdated Service default/test updated"},
		},
		{
			name: "updates the service if labels differ",
			existing: []runtime.Object{
				newServiceWithHash(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			expectedService: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(svc))
				return svc
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceUpdated Service default/test updated"},
		},
		{
			name: "won't update the service if an admission changes the sts",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newServiceWithHash()
					// Simulate admission by changing a value after the hash is computed.
					svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
						Name: "admissionchange",
					})
					return svc
				}(),
			},
			required: newService(),
			expectedService: func() *corev1.Service {
				svc := newServiceWithHash()
				// Simulate admission by changing a value after the hash is computed.
				svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
					Name: "admissionchange",
				})
				return svc
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newServiceWithHash()
					svc.ResourceVersion = "21"
					return svc
				}(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.ResourceVersion = ""
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			expectedService: func() *corev1.Service {
				svc := newService()
				svc.ResourceVersion = "21"
				svc.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(svc))
				return svc
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceUpdated Service default/test updated"},
		},
		{
			name:     "update fails if the service is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newServiceWithHash(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			expectedService: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update service: %w", apierrors.NewNotFound(corev1.Resource("services"), "test")),
			expectedEvents:  []string{`Warning UpdateServiceFailed Failed to update Service default/test: services "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newService()
					svc.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(svc))
					return svc
				}(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			expectedService: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`service "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateServiceFailed Failed to update Service default/test: service "default/test" isn't controlled by us`},
		},
		{
			name: "forced update succeeds if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newService()
					svc.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(svc))
					return svc
				}(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			forceOwnership: true,
			expectedService: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(svc))
				return svc
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceUpdated Service default/test updated"},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newService()
					svc.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(svc))
					return svc
				}(),
			},
			required: func() *corev1.Service {
				svc := newService()
				return svc
			}(),
			expectedService: func() *corev1.Service {
				svc := newService()
				utilruntime.Must(SetHashAnnotation(svc))
				return svc
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceUpdated Service default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newService()
					svc.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(svc))
					return svc
				}(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			expectedService: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`service "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateServiceFailed Failed to update Service default/test: service "default/test" isn't controlled by us`},
		},
		{
			name: "forced update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.Service {
					svc := newService()
					svc.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(svc))
					return svc
				}(),
			},
			required: func() *corev1.Service {
				svc := newService()
				svc.Labels["foo"] = "bar"
				return svc
			}(),
			forceOwnership:  true,
			expectedService: nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`service "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateServiceFailed Failed to update Service default/test: service "default/test" isn't controlled by us`},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyService needs to be reentrant so running it the second time should give the same results.
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

					serviceCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					svcLister := corev1listers.NewServiceLister(serviceCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := serviceCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						svcList, err := client.CoreV1().Services("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range svcList.Items {
							err := serviceCache.Add(&svcList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotSts, gotChanged, gotErr := ApplyService(ctx, client.CoreV1(), svcLister, recorder, tc.required, tc.forceOwnership)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotSts, tc.expectedService) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedService, gotSts, cmp.Diff(tc.expectedService, gotSts))
					}

					// Make sure such object was actually created.
					if gotSts != nil {
						createdSts, err := client.CoreV1().Services(gotSts.Namespace).Get(ctx, gotSts.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSts, gotSts) {
							t.Errorf("created and returned services differ:\n%s", cmp.Diff(createdSts, gotSts))
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

func TestApplySecret(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newSecret := func() *corev1.Secret {
		return &corev1.Secret{
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
			Data: map[string][]byte{},
		}
	}

	newSecretWithHash := func() *corev1.Secret {
		secret := newSecret()
		utilruntime.Must(SetHashAnnotation(secret))
		return secret
	}

	tt := []struct {
		name            string
		existing        []runtime.Object
		cache           []runtime.Object // nil cache means autofill from the client
		required        *corev1.Secret
		forceOwnership  bool
		expectedSecret  *corev1.Secret
		expectedChanged bool
		expectedErr     error
		expectedEvents  []string
	}{
		{
			name:            "creates a new secret when there is none",
			existing:        nil,
			required:        newSecret(),
			expectedSecret:  newSecretWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretCreated Secret default/test created"},
		},
		{
			name: "does nothing if the same secret already exists",
			existing: []runtime.Object{
				newSecretWithHash(),
			},
			required:        newSecret(),
			expectedSecret:  newSecretWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "does nothing if the same secret already exists and required one has the hash",
			existing: []runtime.Object{
				newSecretWithHash(),
			},
			required:        newSecretWithHash(),
			expectedSecret:  newSecretWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "updates the secret if it exists without the hash",
			existing: []runtime.Object{
				newSecret(),
			},
			required:        newSecret(),
			expectedSecret:  newSecretWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretUpdated Secret default/test updated"},
		},
		{
			name:     "fails to create the secret without a controllerRef",
			existing: nil,
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.OwnerReferences = nil
				return secret
			}(),
			expectedSecret:  nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`secret "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the secret if data differs",
			existing: []runtime.Object{
				newSecret(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Data["tls.key"] = []byte("foo")
				return secret
			}(),
			expectedSecret: func() *corev1.Secret {
				secret := newSecret()
				secret.Data["tls.key"] = []byte("foo")
				utilruntime.Must(SetHashAnnotation(secret))
				return secret
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretUpdated Secret default/test updated"},
		},
		{
			name: "updates the secret if labels differ",
			existing: []runtime.Object{
				newSecretWithHash(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			expectedSecret: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(secret))
				return secret
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretUpdated Secret default/test updated"},
		},
		{
			name: "won't update the secret if an admission changes the sts",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecretWithHash()
					// Simulate admission by changing a value after the hash is computed.
					secret.Data["tls.key"] = []byte("admissionchange")
					return secret
				}(),
			},
			required: newSecret(),
			expectedSecret: func() *corev1.Secret {
				secret := newSecretWithHash()
				// Simulate admission by changing a value after the hash is computed.
				secret.Data["tls.key"] = []byte("admissionchange")
				return secret
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecretWithHash()
					secret.ResourceVersion = "21"
					return secret
				}(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.ResourceVersion = ""
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			expectedSecret: func() *corev1.Secret {
				secret := newSecret()
				secret.ResourceVersion = "21"
				secret.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(secret))
				return secret
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretUpdated Secret default/test updated"},
		},
		{
			name:     "update fails if the secret is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newSecretWithHash(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			expectedSecret:  nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update secret: %w", apierrors.NewNotFound(corev1.Resource("secrets"), "test")),
			expectedEvents:  []string{`Warning UpdateSecretFailed Failed to update Secret default/test: secrets "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecret()
					secret.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(secret))
					return secret
				}(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			expectedSecret:  nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`secret "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateSecretFailed Failed to update Secret default/test: secret "default/test" isn't controlled by us`},
		},
		{
			name: "forced update succeeds if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecret()
					secret.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(secret))
					return secret
				}(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			forceOwnership: true,
			expectedSecret: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(secret))
				return secret
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretUpdated Secret default/test updated"},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecret()
					secret.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(secret))
					return secret
				}(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				return secret
			}(),
			expectedSecret: func() *corev1.Secret {
				secret := newSecret()
				utilruntime.Must(SetHashAnnotation(secret))
				return secret
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal SecretUpdated Secret default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecret()
					secret.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(secret))
					return secret
				}(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			expectedSecret:  nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`secret "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateSecretFailed Failed to update Secret default/test: secret "default/test" isn't controlled by us`},
		},
		{
			name: "forced update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.Secret {
					secret := newSecret()
					secret.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(secret))
					return secret
				}(),
			},
			required: func() *corev1.Secret {
				secret := newSecret()
				secret.Labels["foo"] = "bar"
				return secret
			}(),
			forceOwnership:  true,
			expectedSecret:  nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`secret "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateSecretFailed Failed to update Secret default/test: secret "default/test" isn't controlled by us`},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplySecret needs to be reentrant so running it the second time should give the same results.
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

					secretCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					secretLister := corev1listers.NewSecretLister(secretCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := secretCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						secretList, err := client.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range secretList.Items {
							err := secretCache.Add(&secretList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotSts, gotChanged, gotErr := ApplySecret(ctx, client.CoreV1(), secretLister, recorder, tc.required, tc.forceOwnership)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotSts, tc.expectedSecret) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedSecret, gotSts, cmp.Diff(tc.expectedSecret, gotSts))
					}

					// Make sure such object was actually created.
					if gotSts != nil {
						createdSts, err := client.CoreV1().Secrets(gotSts.Namespace).Get(ctx, gotSts.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSts, gotSts) {
							t.Errorf("created and returned secrets differ:\n%s", cmp.Diff(createdSts, gotSts))
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

func TestApplyServiceAccount(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newSA := func() *corev1.ServiceAccount {
		return &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
			},
		}
	}
	newSAWithControllerRef := func() *corev1.ServiceAccount {
		sa := newSA()
		sa.OwnerReferences = []metav1.OwnerReference{
			{
				Controller:         pointer.BoolPtr(true),
				UID:                "abcdefgh",
				APIVersion:         "scylla.scylladb.com/v1",
				Kind:               "ScyllaCluster",
				Name:               "basic",
				BlockOwnerDeletion: pointer.BoolPtr(true),
			},
		}
		return sa
	}

	newSAWithHash := func() *corev1.ServiceAccount {
		sa := newSA()
		utilruntime.Must(SetHashAnnotation(sa))
		return sa
	}

	tt := []struct {
		name                      string
		existing                  []runtime.Object
		cache                     []runtime.Object // nil cache means autofill from the client
		forceOwnership            bool
		allowMissingControllerRef bool
		required                  *corev1.ServiceAccount
		expectedSA                *corev1.ServiceAccount
		expectedChanged           bool
		expectedErr               error
		expectedEvents            []string
	}{
		{
			name:                      "creates a new SA when there is none",
			existing:                  nil,
			allowMissingControllerRef: true,
			required:                  newSA(),
			expectedSA:                newSAWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal ServiceAccountCreated ServiceAccount default/test created"},
		},
		{
			name: "does nothing if the same SA already exists",
			existing: []runtime.Object{
				newSAWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newSA(),
			expectedSA:                newSAWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "does nothing if the same SA already exists and required one has the hash",
			existing: []runtime.Object{
				newSAWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newSAWithHash(),
			expectedSA:                newSAWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "updates the SA if it exists without the hash",
			existing: []runtime.Object{
				newSA(),
			},
			allowMissingControllerRef: true,
			required:                  newSA(),
			expectedSA:                newSAWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
		{
			name:                      "fails to create the SA without a controllerRef",
			existing:                  nil,
			allowMissingControllerRef: false,
			required: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.OwnerReferences = nil
				return sa
			}(),
			expectedSA:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`ServiceAccount "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the SA when AutomountServiceAccountToken differ",
			existing: []runtime.Object{
				newSA(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
				return sa
			}(),
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
				utilruntime.Must(SetHashAnnotation(sa))
				return sa
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
		{
			name: "updates the SA if labels differ",
			existing: []runtime.Object{
				newSAWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.Labels["foo"] = "bar"
				return sa
			}(),
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(sa))
				return sa
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
		{
			name: "won't update the SA if an admission changes the crb",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithHash()
					// Simulate admission by changing a value after the hash is computed.
					sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
					return sa
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newSA(),
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSAWithHash()
				// Simulate admission by changing a value after the hash is computed.
				sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
				return sa
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other test.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					crb := newSAWithHash()
					crb.ResourceVersion = "21"
					return crb
				}(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.ResourceVersion = ""
				sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
				return sa
			}(),
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.ResourceVersion = "21"
				sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
				utilruntime.Must(SetHashAnnotation(sa))
				return sa
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
		{
			name:     "update fails if the SA is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newSAWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.ServiceAccount {
				sa := newSA()
				sa.AutomountServiceAccountToken = pointer.BoolPtr(true)
				return sa
			}(),
			expectedSA:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update serviceaccount: %w", apierrors.NewNotFound(corev1.Resource("serviceaccounts"), "test")),
			expectedEvents:  []string{`Warning UpdateServiceAccountFailed Failed to update ServiceAccount default/test: serviceaccounts "test" not found`},
		},
		{
			name: "update fails if the existing object has ownerRef and required hasn't",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithControllerRef()
					utilruntime.Must(SetHashAnnotation(sa))
					return sa
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newSA(),
			expectedSA:                nil,
			expectedChanged:           false,
			expectedErr:               fmt.Errorf(`serviceAccount "default/test" isn't controlled by us`),
			expectedEvents:            []string{`Warning UpdateServiceAccountFailed Failed to update ServiceAccount default/test: serviceAccount "default/test" isn't controlled by us`},
		},
		{
			name: "forced update succeeds if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSA()
					utilruntime.Must(SetHashAnnotation(sa))
					return sa
				}(),
			},
			required: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Labels["foo"] = "bar"
				return sa
			}(),
			forceOwnership: true,
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(sa))
				return sa
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithControllerRef()
					sa.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(sa))
					return sa
				}(),
			},
			required: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				return sa
			}(),
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				utilruntime.Must(SetHashAnnotation(sa))
				return sa
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithControllerRef()
					sa.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(sa))
					return sa
				}(),
			},
			required: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Labels["foo"] = "bar"
				return sa
			}(),
			expectedSA:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`serviceAccount "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateServiceAccountFailed Failed to update ServiceAccount default/test: serviceAccount "default/test" isn't controlled by us`},
		},
		{
			name: "forced update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithControllerRef()
					sa.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(sa))
					return sa
				}(),
			},
			required: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Labels["foo"] = "bar"
				return sa
			}(),
			forceOwnership:  true,
			expectedSA:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`serviceAccount "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateServiceAccountFailed Failed to update ServiceAccount default/test: serviceAccount "default/test" isn't controlled by us`},
		},
		{
			name: "annotations not starting with our prefix are ignored during hashing and kept in the object",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithControllerRef()
					utilruntime.Must(SetHashAnnotation(sa))
					sa.Annotations["custom-annotation"] = "custom-value"
					return sa
				}(),
			},
			required: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Annotations = map[string]string{
					"custom-annotation": "custom-value",
				}
				return sa
			}(),
			forceOwnership: true,
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				utilruntime.Must(SetHashAnnotation(sa))
				sa.Annotations["custom-annotation"] = "custom-value"
				return sa
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "annotations starting with our prefix are accounted during hash compute",
			existing: []runtime.Object{
				func() *corev1.ServiceAccount {
					sa := newSAWithControllerRef()
					utilruntime.Must(SetHashAnnotation(sa))
					sa.Annotations["custom-annotation"] = "custom-value"
					return sa
				}(),
			},
			required: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Annotations = map[string]string{
					"custom-annotation":                   "custom-value",
					"scylla-operator.scylladb.com/blabla": "123",
				}
				return sa
			}(),
			forceOwnership: true,
			expectedSA: func() *corev1.ServiceAccount {
				sa := newSAWithControllerRef()
				sa.Annotations = map[string]string{
					"scylla-operator.scylladb.com/blabla": "123",
				}
				utilruntime.Must(SetHashAnnotation(sa))
				sa.Annotations["custom-annotation"] = "custom-value"
				return sa
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ServiceAccountUpdated ServiceAccount default/test updated"},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state, so it has to persist the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyClusterRole needs to be reentrant so running it the second time should give the same results.
			// (One of the common mistakes is editing the object after computing the hash, so it differs the second time.)
			iterations := 2
			if tc.expectedErr != nil {
				iterations = 1
			}
			for i := 0; i < iterations; i++ {
				t.Run("", func(t *testing.T) {
					ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer ctxCancel()

					recorder := record.NewFakeRecorder(10)

					saCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					crbLister := corev1listers.NewServiceAccountLister(saCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := saCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						crList, err := client.CoreV1().ServiceAccounts(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range crList.Items {
							err := saCache.Add(&crList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotSA, gotChanged, gotErr := ApplyServiceAccount(ctx, client.CoreV1(), crbLister, recorder, tc.required, tc.forceOwnership, tc.allowMissingControllerRef)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotSA, tc.expectedSA) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedSA, gotSA, cmp.Diff(tc.expectedSA, gotSA))
					}

					// Make sure such object was actually created.
					if gotSA != nil {
						createdSA, err := client.CoreV1().ServiceAccounts(gotSA.Namespace).Get(ctx, gotSA.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSA, gotSA) {
							t.Errorf("created and returned ServiceAccounts differ:\n%s", cmp.Diff(createdSA, gotSA))
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

func TestApplyConfigMap(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newConfigMap := func() *corev1.ConfigMap {
		return &corev1.ConfigMap{
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
			Data: map[string]string{},
		}
	}

	newConfigMapWithHash := func() *corev1.ConfigMap {
		cm := newConfigMap()
		utilruntime.Must(SetHashAnnotation(cm))
		return cm
	}

	tt := []struct {
		name            string
		existing        []runtime.Object
		cache           []runtime.Object // nil cache means autofill from the client
		required        *corev1.ConfigMap
		expectedCM      *corev1.ConfigMap
		expectedChanged bool
		expectedErr     error
		expectedEvents  []string
	}{
		{
			name:            "creates a new configmap when there is none",
			existing:        nil,
			required:        newConfigMap(),
			expectedCM:      newConfigMapWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ConfigMapCreated ConfigMap default/test created"},
		},
		{
			name: "does nothing if the same configmap already exists",
			existing: []runtime.Object{
				newConfigMapWithHash(),
			},
			required:        newConfigMap(),
			expectedCM:      newConfigMapWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "does nothing if the same configmap already exists and required one has the hash",
			existing: []runtime.Object{
				newConfigMapWithHash(),
			},
			required:        newConfigMapWithHash(),
			expectedCM:      newConfigMapWithHash(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			name: "updates the configmap if it exists without the hash",
			existing: []runtime.Object{
				newConfigMap(),
			},
			required:        newConfigMap(),
			expectedCM:      newConfigMapWithHash(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ConfigMapUpdated ConfigMap default/test updated"},
		},
		{
			name:     "fails to create the configmap without a controllerRef",
			existing: nil,
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.OwnerReferences = nil
				return cm
			}(),
			expectedCM:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`ConfigMap "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the configmap if data differs",
			existing: []runtime.Object{
				newConfigMap(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Data["tls.key"] = "foo"
				return cm
			}(),
			expectedCM: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Data["tls.key"] = "foo"
				utilruntime.Must(SetHashAnnotation(cm))
				return cm
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ConfigMapUpdated ConfigMap default/test updated"},
		},
		{
			name: "updates the configmap if labels differ",
			existing: []runtime.Object{
				newConfigMapWithHash(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Labels["foo"] = "bar"
				return cm
			}(),
			expectedCM: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(cm))
				return cm
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ConfigMapUpdated ConfigMap default/test updated"},
		},
		{
			name: "won't update the configmap if an admission changes the sts",
			existing: []runtime.Object{
				func() *corev1.ConfigMap {
					cm := newConfigMapWithHash()
					// Simulate admission by changing a value after the hash is computed.
					cm.Data["tls.key"] = "admissionchange"
					return cm
				}(),
			},
			required: newConfigMap(),
			expectedCM: func() *corev1.ConfigMap {
				cm := newConfigMapWithHash()
				// Simulate admission by changing a value after the hash is computed.
				cm.Data["tls.key"] = "admissionchange"
				return cm
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tests.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *corev1.ConfigMap {
					cm := newConfigMapWithHash()
					cm.ResourceVersion = "21"
					return cm
				}(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.ResourceVersion = ""
				cm.Labels["foo"] = "bar"
				return cm
			}(),
			expectedCM: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.ResourceVersion = "21"
				cm.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(cm))
				return cm
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ConfigMapUpdated ConfigMap default/test updated"},
		},
		{
			name:     "update fails if the configmap is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newConfigMapWithHash(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Labels["foo"] = "bar"
				return cm
			}(),
			expectedCM:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update configmap: %w", apierrors.NewNotFound(corev1.Resource("configmaps"), "test")),
			expectedEvents:  []string{`Warning UpdateConfigMapFailed Failed to update ConfigMap default/test: configmaps "test" not found`},
		},
		{
			name: "update fails if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *corev1.ConfigMap {
					cm := newConfigMap()
					cm.OwnerReferences = nil
					utilruntime.Must(SetHashAnnotation(cm))
					return cm
				}(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Labels["foo"] = "bar"
				return cm
			}(),
			expectedCM:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`configmap "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateConfigMapFailed Failed to update ConfigMap default/test: configmap "default/test" isn't controlled by us`},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *corev1.ConfigMap {
					cm := newConfigMap()
					cm.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(cm))
					return cm
				}(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				return cm
			}(),
			expectedCM: func() *corev1.ConfigMap {
				cm := newConfigMap()
				utilruntime.Must(SetHashAnnotation(cm))
				return cm
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ConfigMapUpdated ConfigMap default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *corev1.ConfigMap {
					cm := newConfigMap()
					cm.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(cm))
					return cm
				}(),
			},
			required: func() *corev1.ConfigMap {
				cm := newConfigMap()
				cm.Labels["foo"] = "bar"
				return cm
			}(),
			expectedCM:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`configmap "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateConfigMapFailed Failed to update ConfigMap default/test: configmap "default/test" isn't controlled by us`},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persists the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyConfigMap needs to be reentrant so running it the second time should give the same results.
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

					configmapCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					configmapLister := corev1listers.NewConfigMapLister(configmapCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := configmapCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						configmapList, err := client.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range configmapList.Items {
							err := configmapCache.Add(&configmapList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotSts, gotChanged, gotErr := ApplyConfigMap(ctx, client.CoreV1(), configmapLister, recorder, tc.required)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotSts, tc.expectedCM) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedCM, gotSts, cmp.Diff(tc.expectedCM, gotSts))
					}

					// Make sure such object was actually created.
					if gotSts != nil {
						createdSts, err := client.CoreV1().ConfigMaps(gotSts.Namespace).Get(ctx, gotSts.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdSts, gotSts) {
							t.Errorf("created and returned configmaps differ:\n%s", cmp.Diff(createdSts, gotSts))
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

func TestApplyNamespace(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newNS := func() *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
			},
		}
	}

	newNSWithHash := func() *corev1.Namespace {
		ns := newNS()
		utilruntime.Must(SetHashAnnotation(ns))
		return ns
	}

	tt := []struct {
		name                      string
		existing                  []runtime.Object
		cache                     []runtime.Object // nil cache means autofill from the client
		allowMissingControllerRef bool
		required                  *corev1.Namespace
		expectedNS                *corev1.Namespace
		expectedChanged           bool
		expectedErr               error
		expectedEvents            []string
	}{
		{
			name:                      "creates a new Namespace when there is none",
			existing:                  nil,
			allowMissingControllerRef: true,
			required:                  newNS(),
			expectedNS:                newNSWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal NamespaceCreated Namespace test created"},
		},
		{
			name: "does nothing if the same Namespace already exists",
			existing: []runtime.Object{
				newNSWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newNS(),
			expectedNS:                newNSWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "does nothing if the same Namespace already exists and required one has the hash",
			existing: []runtime.Object{
				newNSWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newNSWithHash(),
			expectedNS:                newNSWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "updates the Namespace if it exists without the hash",
			existing: []runtime.Object{
				newNS(),
			},
			allowMissingControllerRef: true,
			required:                  newNS(),
			expectedNS:                newNSWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal NamespaceUpdated Namespace test updated"},
		},
		{
			name:                      "fails to create the Namespace without a controllerRef",
			existing:                  nil,
			allowMissingControllerRef: false,
			required: func() *corev1.Namespace {
				ns := newNS()
				ns.OwnerReferences = nil
				return ns
			}(),
			expectedNS:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`namespace "test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the Namespace when Finalizers differ",
			existing: []runtime.Object{
				newNS(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.Namespace {
				ns := newNS()
				ns.Finalizers = []string{"boop"}
				return ns
			}(),
			expectedNS: func() *corev1.Namespace {
				ns := newNS()
				ns.Finalizers = []string{"boop"}
				utilruntime.Must(SetHashAnnotation(ns))
				return ns
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal NamespaceUpdated Namespace test updated"},
		},
		{
			name: "updates the Namespace if labels differ",
			existing: []runtime.Object{
				newNSWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.Namespace {
				ns := newNS()
				ns.Labels["foo"] = "bar"
				return ns
			}(),
			expectedNS: func() *corev1.Namespace {
				ns := newNS()
				ns.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(ns))
				return ns
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal NamespaceUpdated Namespace test updated"},
		},
		{
			name: "won't update the Namespace if an admission changes the ns",
			existing: []runtime.Object{
				func() *corev1.Namespace {
					ns := newNSWithHash()
					// Simulate admission by changing a value after the hash is computed.
					ns.Finalizers = []string{"boop"}
					return ns
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newNS(),
			expectedNS: func() *corev1.Namespace {
				ns := newNSWithHash()
				// Simulate admission by changing a value after the hash is computed.
				ns.Finalizers = []string{"boop"}
				return ns
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other test.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *corev1.Namespace {
					ns := newNSWithHash()
					ns.ResourceVersion = "21"
					return ns
				}(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.Namespace {
				ns := newNS()
				ns.ResourceVersion = ""
				ns.Finalizers = []string{"boop"}
				return ns
			}(),
			expectedNS: func() *corev1.Namespace {
				ns := newNS()
				ns.ResourceVersion = "21"
				ns.Finalizers = []string{"boop"}
				utilruntime.Must(SetHashAnnotation(ns))
				return ns
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal NamespaceUpdated Namespace test updated"},
		},
		{
			name:     "update fails if the Namespace is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newNSWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *corev1.Namespace {
				ns := newNS()
				ns.Finalizers = []string{"boop"}
				return ns
			}(),
			expectedNS:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update namespace: %w", apierrors.NewNotFound(corev1.Resource("namespaces"), "test")),
			expectedEvents:  []string{`Warning UpdateNamespaceFailed Failed to update Namespace test: namespaces "test" not found`},
		},
		{
			name: "update fails if the existing object has ownerRef and required hasn't",
			existing: []runtime.Object{
				func() *corev1.Namespace {
					ns := newNS()
					ns.OwnerReferences = []metav1.OwnerReference{
						{
							Controller:         pointer.BoolPtr(true),
							UID:                "abcdefgh",
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "basic",
							BlockOwnerDeletion: pointer.BoolPtr(true),
						},
					}
					utilruntime.Must(SetHashAnnotation(ns))
					return ns
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newNS(),
			expectedNS:                nil,
			expectedChanged:           false,
			expectedErr:               fmt.Errorf(`namespace "test" isn't controlled by us`),
			expectedEvents:            []string{`Warning UpdateNamespaceFailed Failed to update Namespace test: namespace "test" isn't controlled by us`},
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Client holds the state so it has to persicr the iterations.
			client := fake.NewSimpleClientset(tc.existing...)

			// ApplyClusterRole needs to be reentrant so running it the second time should give the same results.
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

					nsCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					nsLister := corev1listers.NewNamespaceLister(nsCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := nsCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range nsList.Items {
							err := nsCache.Add(&nsList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotNS, gotChanged, gotErr := ApplyNamespace(ctx, client.CoreV1(), nsLister, recorder, tc.required, tc.allowMissingControllerRef)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotNS, tc.expectedNS) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedNS, gotNS, cmp.Diff(tc.expectedNS, gotNS))
					}

					// Make sure such object was actually created.
					if gotNS != nil {
						createdNS, err := client.CoreV1().Namespaces().Get(ctx, gotNS.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdNS, gotNS) {
							t.Errorf("created and returned Namespaces differ:\n%s", cmp.Diff(createdNS, gotNS))
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
