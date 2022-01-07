package resourceapply

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
)

func TestApplyClusterRole(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newCr := func() *rbacv1.ClusterRole {
		return &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch", "update"},
				},
			},
		}
	}

	newCrWithHash := func() *rbacv1.ClusterRole {
		cr := newCr()
		utilruntime.Must(SetHashAnnotation(cr))
		return cr
	}

	tt := []struct {
		name                      string
		existing                  []runtime.Object
		cache                     []runtime.Object // nil cache means autofill from the client
		allowMissingControllerRef bool
		required                  *rbacv1.ClusterRole
		expectedCr                *rbacv1.ClusterRole
		expectedChanged           bool
		expectedErr               error
		expectedEvents            []string
	}{
		{
			name:                      "creates a new cr when there is none",
			existing:                  nil,
			allowMissingControllerRef: true,
			required:                  newCr(),
			expectedCr:                newCrWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal ClusterRoleCreated ClusterRole test created"},
		},
		{
			name: "does nothing if the same cr already exists",
			existing: []runtime.Object{
				newCrWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newCr(),
			expectedCr:                newCrWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "does nothing if the same cr already exists and required one has the hash",
			existing: []runtime.Object{
				newCrWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newCrWithHash(),
			expectedCr:                newCrWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "updates the cr if it exists without the hash",
			existing: []runtime.Object{
				newCr(),
			},
			allowMissingControllerRef: true,
			required:                  newCr(),
			expectedCr:                newCrWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal ClusterRoleUpdated ClusterRole test updated"},
		},
		{
			name:                      "fails to create the cr without a controllerRef",
			existing:                  nil,
			allowMissingControllerRef: false,
			required: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.OwnerReferences = nil
				return cr
			}(),
			expectedCr:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`ClusterRole "test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the cr rules differ",
			existing: []runtime.Object{
				newCr(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Rules[0].Verbs = []string{"update"}
				return cr
			}(),
			expectedCr: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Rules[0].Verbs = []string{"update"}
				utilruntime.Must(SetHashAnnotation(cr))
				return cr
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleUpdated ClusterRole test updated"},
		},
		{
			name: "updates the cr if labels differ",
			existing: []runtime.Object{
				newCrWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Labels["foo"] = "bar"
				return cr
			}(),
			expectedCr: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(cr))
				return cr
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleUpdated ClusterRole test updated"},
		},
		{
			name: "updates the cr if new rule is added",
			existing: []runtime.Object{
				newCrWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Rules = append(cr.Rules, rbacv1.PolicyRule{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets"},
					Verbs:     []string{"get", "list", "watch"},
				})
				return cr
			}(),
			expectedCr: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Rules = append(cr.Rules, rbacv1.PolicyRule{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets"},
					Verbs:     []string{"get", "list", "watch"},
				})
				utilruntime.Must(SetHashAnnotation(cr))
				return cr
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleUpdated ClusterRole test updated"},
		},
		{
			name: "won't update the cr if an admission changes the cr",
			existing: []runtime.Object{
				func() *rbacv1.ClusterRole {
					cr := newCrWithHash()
					// Simulate admission by changing a value after the hash is computed.
					cr.Rules[0].Verbs = []string{"update"}
					return cr
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newCr(),
			expectedCr: func() *rbacv1.ClusterRole {
				cr := newCrWithHash()
				// Simulate admission by changing a value after the hash is computed.
				cr.Rules[0].Verbs = []string{"update"}
				return cr
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tecr.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *rbacv1.ClusterRole {
					cr := newCrWithHash()
					cr.ResourceVersion = "21"
					return cr
				}(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.ResourceVersion = ""
				cr.Rules[0].Verbs = []string{"update"}
				return cr
			}(),
			expectedCr: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.ResourceVersion = "21"
				cr.Rules[0].Verbs = []string{"update"}
				utilruntime.Must(SetHashAnnotation(cr))
				return cr
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleUpdated ClusterRole test updated"},
		},
		{
			name:     "update fails if the cr is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newCrWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRole {
				cr := newCr()
				cr.Rules[0].Verbs = []string{"update"}
				return cr
			}(),
			expectedCr:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update clusterrole: %w", apierrors.NewNotFound(rbacv1.Resource("clusterroles"), "test")),
			expectedEvents:  []string{`Warning UpdateClusterRoleFailed Failed to update ClusterRole test: clusterroles.rbac.authorization.k8s.io "test" not found`},
		},
		{
			name: "update fails if the existing object has ownerRef and required not",
			existing: []runtime.Object{
				func() *rbacv1.ClusterRole {
					cr := newCr()
					cr.OwnerReferences = []metav1.OwnerReference{{
						Controller:         pointer.BoolPtr(true),
						UID:                "abcdefgh",
						APIVersion:         "scylla.scylladb.com/v1",
						Kind:               "ScyllaCluster",
						Name:               "basic",
						BlockOwnerDeletion: pointer.BoolPtr(true),
					}}
					utilruntime.Must(SetHashAnnotation(cr))
					return cr
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newCr(),
			expectedCr:                nil,
			expectedChanged:           false,
			expectedErr:               fmt.Errorf(`clusterrole "test" is controlled by someone else`),
			expectedEvents:            []string{`Warning UpdateClusterRoleFailed Failed to update ClusterRole test: clusterrole "test" is controlled by someone else`},
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

					crCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					crLister := rbacv1listers.NewClusterRoleLister(crCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := crCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						crList, err := client.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range crList.Items {
							err := crCache.Add(&crList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotCr, gotChanged, gotErr := ApplyClusterRole(ctx, client.RbacV1(), crLister, recorder, tc.required, tc.allowMissingControllerRef)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotCr, tc.expectedCr) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedCr, gotCr, cmp.Diff(tc.expectedCr, gotCr))
					}

					// Make sure such object was actually created.
					if gotCr != nil {
						createdCr, err := client.RbacV1().ClusterRoles().Get(ctx, gotCr.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdCr, gotCr) {
							t.Errorf("created and returned clusterroles differ:\n%s", cmp.Diff(createdCr, gotCr))
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

func TestApplyClusterRoleBinding(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newCrb := func() *rbacv1.ClusterRoleBinding {
		return &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: "default",
					Name:      "sa",
				},
			},
		}
	}

	newCrbWithHash := func() *rbacv1.ClusterRoleBinding {
		cr := newCrb()
		utilruntime.Must(SetHashAnnotation(cr))
		return cr
	}

	tt := []struct {
		name                      string
		existing                  []runtime.Object
		cache                     []runtime.Object // nil cache means autofill from the client
		allowMissingControllerRef bool
		required                  *rbacv1.ClusterRoleBinding
		expectedCrb               *rbacv1.ClusterRoleBinding
		expectedChanged           bool
		expectedErr               error
		expectedEvents            []string
	}{
		{
			name:                      "creates a new crb when there is none",
			existing:                  nil,
			allowMissingControllerRef: true,
			required:                  newCrb(),
			expectedCrb:               newCrbWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal ClusterRoleBindingCreated ClusterRoleBinding test created"},
		},
		{
			name: "does nothing if the same crb already exists",
			existing: []runtime.Object{
				newCrbWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newCrb(),
			expectedCrb:               newCrbWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "does nothing if the same crb already exists and required one has the hash",
			existing: []runtime.Object{
				newCrbWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newCrbWithHash(),
			expectedCrb:               newCrbWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "updates the cr if it exists without the hash",
			existing: []runtime.Object{
				newCrb(),
			},
			allowMissingControllerRef: true,
			required:                  newCrb(),
			expectedCrb:               newCrbWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal ClusterRoleBindingUpdated ClusterRoleBinding test updated"},
		},
		{
			name:                      "fails to create the crb without a controllerRef ",
			existing:                  nil,
			allowMissingControllerRef: false,
			required: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.OwnerReferences = nil
				return crb
			}(),
			expectedCrb:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`ClusterRoleBinding "test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the crb when subjects differ",
			existing: []runtime.Object{
				newCrb(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.Subjects[0].Name = "different-name"
				return crb
			}(),
			expectedCrb: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.Subjects[0].Name = "different-name"
				utilruntime.Must(SetHashAnnotation(crb))
				return crb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleBindingUpdated ClusterRoleBinding test updated"},
		},
		{
			name: "deletes and creates the crb when roleref differ",
			existing: []runtime.Object{
				newCrb(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.RoleRef = rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "different-name",
				}
				return crb
			}(),
			expectedCrb: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.RoleRef = rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "different-name",
				}
				utilruntime.Must(SetHashAnnotation(crb))
				return crb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents: []string{
				"Normal ClusterRoleBindingDeleted ClusterRoleBinding test deleted",
				"Normal ClusterRoleBindingCreated ClusterRoleBinding test created",
			},
		},
		{
			name: "updates the crb if labels differ",
			existing: []runtime.Object{
				newCrbWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.Labels["foo"] = "bar"
				return crb
			}(),
			expectedCrb: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(crb))
				return crb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleBindingUpdated ClusterRoleBinding test updated"},
		},
		{
			name: "won't update the crb if an admission changes the crb",
			existing: []runtime.Object{
				func() *rbacv1.ClusterRoleBinding {
					crb := newCrbWithHash()
					// Simulate admission by changing a value after the hash is computed.
					crb.Subjects[0].Name = "different-name"
					return crb
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newCrb(),
			expectedCrb: func() *rbacv1.ClusterRoleBinding {
				crb := newCrbWithHash()
				// Simulate admission by changing a value after the hash is computed.
				crb.Subjects[0].Name = "different-name"
				return crb
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other tecr.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *rbacv1.ClusterRoleBinding {
					crb := newCrbWithHash()
					crb.ResourceVersion = "21"
					return crb
				}(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.ResourceVersion = ""
				crb.Subjects[0].Name = "different-name"
				return crb
			}(),
			expectedCrb: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.ResourceVersion = "21"
				crb.Subjects[0].Name = "different-name"
				utilruntime.Must(SetHashAnnotation(crb))
				return crb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal ClusterRoleBindingUpdated ClusterRoleBinding test updated"},
		},
		{
			name:     "update fails if the crb is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newCrbWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.ClusterRoleBinding {
				crb := newCrb()
				crb.Subjects[0].Name = "different-name"
				return crb
			}(),
			expectedCrb:     nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update clusterrolebinding: %w", apierrors.NewNotFound(rbacv1.Resource("clusterrolebindings"), "test")),
			expectedEvents:  []string{`Warning UpdateClusterRoleBindingFailed Failed to update ClusterRoleBinding test: clusterrolebindings.rbac.authorization.k8s.io "test" not found`},
		},
		{
			name: "update fails if the existing object has ownerRef and required hasn't",
			existing: []runtime.Object{
				func() *rbacv1.ClusterRoleBinding {
					crb := newCrb()
					crb.OwnerReferences = []metav1.OwnerReference{
						{
							Controller:         pointer.BoolPtr(true),
							UID:                "abcdefgh",
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "basic",
							BlockOwnerDeletion: pointer.BoolPtr(true),
						},
					}
					utilruntime.Must(SetHashAnnotation(crb))
					return crb
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newCrb(),
			expectedCrb:               nil,
			expectedChanged:           false,
			expectedErr:               fmt.Errorf(`clusterrolebinding "test" is controlled by someone else`),
			expectedEvents:            []string{`Warning UpdateClusterRoleBindingFailed Failed to update ClusterRoleBinding test: clusterrolebinding "test" is controlled by someone else`},
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

					crbCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					crbLister := rbacv1listers.NewClusterRoleBindingLister(crbCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := crbCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						crList, err := client.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range crList.Items {
							err := crbCache.Add(&crList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotCrb, gotChanged, gotErr := ApplyClusterRoleBinding(ctx, client.RbacV1(), crbLister, recorder, tc.required, tc.allowMissingControllerRef)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotCrb, tc.expectedCrb) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedCrb, gotCrb, cmp.Diff(tc.expectedCrb, gotCrb))
					}

					// Make sure such object was actually created.
					if gotCrb != nil {
						createdCrb, err := client.RbacV1().ClusterRoleBindings().Get(ctx, gotCrb.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdCrb, gotCrb) {
							t.Errorf("created and returned clusterrolebindings differ:\n%s", cmp.Diff(createdCrb, gotCrb))
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

func TestApplyRoleBinding(t *testing.T) {
	// Using a generating function prevents unwanted mutations.
	newRB := func() *rbacv1.RoleBinding {
		return &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
				// Setting a RV make sure it's propagated to update calls for optimistic concurrency.
				ResourceVersion: "42",
				Labels:          map[string]string{},
			},
		}
	}
	newRBWithControllerRef := func() *rbacv1.RoleBinding {
		rb := newRB()
		rb.OwnerReferences = []metav1.OwnerReference{
			{
				Controller:         pointer.BoolPtr(true),
				UID:                "abcdefgh",
				APIVersion:         "scylla.scylladb.com/v1",
				Kind:               "ScyllaCluster",
				Name:               "basic",
				BlockOwnerDeletion: pointer.BoolPtr(true),
			},
		}
		return rb
	}

	newRBWithHash := func() *rbacv1.RoleBinding {
		rb := newRB()
		utilruntime.Must(SetHashAnnotation(rb))
		return rb
	}

	tt := []struct {
		name                      string
		existing                  []runtime.Object
		cache                     []runtime.Object // nil cache means autofill from the client
		forceOwnership            bool
		allowMissingControllerRef bool
		required                  *rbacv1.RoleBinding
		expectedRB                *rbacv1.RoleBinding
		expectedChanged           bool
		expectedErr               error
		expectedEvents            []string
	}{
		{
			name:                      "creates a new RB when there is none",
			existing:                  nil,
			allowMissingControllerRef: true,
			required:                  newRB(),
			expectedRB:                newRBWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal RoleBindingCreated RoleBinding default/test created"},
		},
		{
			name: "does nothing if the same RB already exists",
			existing: []runtime.Object{
				newRBWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newRB(),
			expectedRB:                newRBWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "does nothing if the same RB already exists and required one has the hash",
			existing: []runtime.Object{
				newRBWithHash(),
			},
			allowMissingControllerRef: true,
			required:                  newRBWithHash(),
			expectedRB:                newRBWithHash(),
			expectedChanged:           false,
			expectedErr:               nil,
			expectedEvents:            nil,
		},
		{
			name: "updates the RB if it exists without the hash",
			existing: []runtime.Object{
				newRB(),
			},
			allowMissingControllerRef: true,
			required:                  newRB(),
			expectedRB:                newRBWithHash(),
			expectedChanged:           true,
			expectedErr:               nil,
			expectedEvents:            []string{"Normal RoleBindingUpdated RoleBinding default/test updated"},
		},
		{
			name:                      "fails to create the RB without a controllerRef",
			existing:                  nil,
			allowMissingControllerRef: false,
			required: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.OwnerReferences = nil
				return rb
			}(),
			expectedRB:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`RoleBinding "default/test" is missing controllerRef`),
			expectedEvents:  nil,
		},
		{
			name: "updates the RB when AutomountRoleBindingToken differ",
			existing: []runtime.Object{
				newRB(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.RoleRef.Name = "foo"
				return rb
			}(),
			expectedRB: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.RoleRef.Name = "foo"
				utilruntime.Must(SetHashAnnotation(rb))
				return rb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal RoleBindingUpdated RoleBinding default/test updated"},
		},
		{
			name: "updates the RB if labels differ",
			existing: []runtime.Object{
				newRBWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.Labels["foo"] = "bar"
				return rb
			}(),
			expectedRB: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(rb))
				return rb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal RoleBindingUpdated RoleBinding default/test updated"},
		},
		{
			name: "won't update the RB if an admission changes the RB",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRBWithHash()
					// Simulate admission by changing a value after the hash is computed.
					rb.RoleRef.Name = "foo"
					return rb
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newRB(),
			expectedRB: func() *rbacv1.RoleBinding {
				rb := newRBWithHash()
				// Simulate admission by changing a value after the hash is computed.
				rb.RoleRef.Name = "foo"
				return rb
			}(),
			expectedChanged: false,
			expectedErr:     nil,
			expectedEvents:  nil,
		},
		{
			// We test propagating the RV from required in all the other test.
			name: "specifying no RV will use the one from the existing object",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRBWithHash()
					rb.ResourceVersion = "21"
					return rb
				}(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.ResourceVersion = ""
				rb.RoleRef.Name = "foo"
				return rb
			}(),
			expectedRB: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.ResourceVersion = "21"
				rb.RoleRef.Name = "foo"
				utilruntime.Must(SetHashAnnotation(rb))
				return rb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal RoleBindingUpdated RoleBinding default/test updated"},
		},
		{
			name:     "update fails if the RB is missing but we still see it in the cache",
			existing: nil,
			cache: []runtime.Object{
				newRBWithHash(),
			},
			allowMissingControllerRef: true,
			required: func() *rbacv1.RoleBinding {
				rb := newRB()
				rb.RoleRef.Name = "foo"
				return rb
			}(),
			expectedRB:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf("can't update roleBinding: %w", apierrors.NewNotFound(rbacv1.Resource("rolebindings"), "test")),
			expectedEvents:  []string{`Warning UpdateRoleBindingFailed Failed to update RoleBinding default/test: rolebindings.rbac.authorization.k8s.io "test" not found`},
		},
		{
			name: "update fails if the existing object has ownerRef and required hasn't",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRBWithControllerRef()
					utilruntime.Must(SetHashAnnotation(rb))
					return rb
				}(),
			},
			allowMissingControllerRef: true,
			required:                  newRB(),
			expectedRB:                nil,
			expectedChanged:           false,
			expectedErr:               fmt.Errorf(`roleBinding "default/test" isn't controlled by us`),
			expectedEvents:            []string{`Warning UpdateRoleBindingFailed Failed to update RoleBinding default/test: roleBinding "default/test" isn't controlled by us`},
		},
		{
			name: "forced update succeeds if the existing object has no ownerRef",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRB()
					utilruntime.Must(SetHashAnnotation(rb))
					return rb
				}(),
			},
			required: func() *rbacv1.RoleBinding {
				rb := newRBWithControllerRef()
				rb.Labels["foo"] = "bar"
				return rb
			}(),
			forceOwnership: true,
			expectedRB: func() *rbacv1.RoleBinding {
				rb := newRBWithControllerRef()
				rb.Labels["foo"] = "bar"
				utilruntime.Must(SetHashAnnotation(rb))
				return rb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal RoleBindingUpdated RoleBinding default/test updated"},
		},
		{
			name: "update succeeds to replace ownerRef kind",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRBWithControllerRef()
					rb.OwnerReferences[0].Kind = "WrongKind"
					utilruntime.Must(SetHashAnnotation(rb))
					return rb
				}(),
			},
			required: func() *rbacv1.RoleBinding {
				rb := newRBWithControllerRef()
				return rb
			}(),
			expectedRB: func() *rbacv1.RoleBinding {
				rb := newRBWithControllerRef()
				utilruntime.Must(SetHashAnnotation(rb))
				return rb
			}(),
			expectedChanged: true,
			expectedErr:     nil,
			expectedEvents:  []string{"Normal RoleBindingUpdated RoleBinding default/test updated"},
		},
		{
			name: "update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRBWithControllerRef()
					rb.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(rb))
					return rb
				}(),
			},
			required: func() *rbacv1.RoleBinding {
				rb := newRBWithControllerRef()
				rb.Labels["foo"] = "bar"
				return rb
			}(),
			expectedRB:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`roleBinding "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateRoleBindingFailed Failed to update RoleBinding default/test: roleBinding "default/test" isn't controlled by us`},
		},
		{
			name: "forced update fails if the existing object is owned by someone else",
			existing: []runtime.Object{
				func() *rbacv1.RoleBinding {
					rb := newRBWithControllerRef()
					rb.OwnerReferences[0].UID = "42"
					utilruntime.Must(SetHashAnnotation(rb))
					return rb
				}(),
			},
			required: func() *rbacv1.RoleBinding {
				rb := newRBWithControllerRef()
				rb.Labels["foo"] = "bar"
				return rb
			}(),
			forceOwnership:  true,
			expectedRB:      nil,
			expectedChanged: false,
			expectedErr:     fmt.Errorf(`roleBinding "default/test" isn't controlled by us`),
			expectedEvents:  []string{`Warning UpdateRoleBindingFailed Failed to update RoleBinding default/test: roleBinding "default/test" isn't controlled by us`},
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

					rbCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
					rbLister := rbacv1listers.NewRoleBindingLister(rbCache)

					if tc.cache != nil {
						for _, obj := range tc.cache {
							err := rbCache.Add(obj)
							if err != nil {
								t.Fatal(err)
							}
						}
					} else {
						rbList, err := client.RbacV1().RoleBindings(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
							LabelSelector: labels.Everything().String(),
						})
						if err != nil {
							t.Fatal(err)
						}

						for i := range rbList.Items {
							err := rbCache.Add(&rbList.Items[i])
							if err != nil {
								t.Fatal(err)
							}
						}
					}

					gotRB, gotChanged, gotErr := ApplyRoleBinding(ctx, client.RbacV1(), rbLister, recorder, tc.required, tc.forceOwnership, tc.allowMissingControllerRef)
					if !reflect.DeepEqual(gotErr, tc.expectedErr) {
						t.Fatalf("expected %v, got %v", tc.expectedErr, gotErr)
					}

					if !equality.Semantic.DeepEqual(gotRB, tc.expectedRB) {
						t.Errorf("expected %#v, got %#v, diff:\n%s", tc.expectedRB, gotRB, cmp.Diff(tc.expectedRB, gotRB))
					}

					// Make sure such object was actually created.
					if gotRB != nil {
						createdRB, err := client.RbacV1().RoleBindings(gotRB.Namespace).Get(ctx, gotRB.Name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						if !equality.Semantic.DeepEqual(createdRB, gotRB) {
							t.Errorf("created and returned RoleBindings differ:\n%s", cmp.Diff(createdRB, gotRB))
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
