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

					gotSts, gotChanged, gotErr := ApplyService(ctx, client.CoreV1(), svcLister, recorder, tc.required)
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

					gotSts, gotChanged, gotErr := ApplySecret(ctx, client.CoreV1(), secretLister, recorder, tc.required)
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
