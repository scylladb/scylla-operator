// Copyright (c) 2022 ScyllaDB.

package migration

import (
	"context"
	"testing"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllafake "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/fake"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestMigrationController(t *testing.T) {
	tt := []struct {
		name       string
		createFunc func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error)
		updateFunc func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error)
		getFunc    func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error)
	}{
		{
			name: "service with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.CoreV1().Services(sc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				svc := obj.(*corev1.Service)
				return client.CoreV1().Services(svc.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "secret with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.CoreV1().Secrets(sc.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				svc := obj.(*corev1.Secret)
				return client.CoreV1().Secrets(svc.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "secret with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.CoreV1().Secrets(sc.Namespace).Create(ctx, secret, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				secret := obj.(*corev1.Secret)
				return client.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "configMap with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.CoreV1().ConfigMaps(sc.Namespace).Create(ctx, cm, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				cm := obj.(*corev1.ConfigMap)
				return client.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "serviceAccount with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.CoreV1().ServiceAccounts(sc.Namespace).Create(ctx, sa, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				sa := obj.(*corev1.ServiceAccount)
				return client.CoreV1().ServiceAccounts(sa.Namespace).Update(ctx, sa, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "roleBinding with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				rb := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.RbacV1().RoleBindings(sc.Namespace).Create(ctx, rb, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				rb := obj.(*rbacv1.RoleBinding)
				return client.RbacV1().RoleBindings(rb.Namespace).Update(ctx, rb, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.RbacV1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "statefulSet with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.AppsV1().StatefulSets(sc.Namespace).Create(ctx, sts, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				sts := obj.(*appsv1.StatefulSet)
				return client.AppsV1().StatefulSets(sts.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "podDisruptionBudget with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				pdb := &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.PolicyV1().PodDisruptionBudgets(sc.Namespace).Create(ctx, pdb, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				pdb := obj.(*policyv1.PodDisruptionBudget)
				return client.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Update(ctx, pdb, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
		{
			name: "ingress with matching label selector is orphaned",
			createFunc: func(ctx context.Context, client kubernetes.Interface, sc *scyllav1.ScyllaCluster) (metav1.Object, error) {
				ingress := &networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name: sc.Name,
						Labels: map[string]string{
							"scylla/cluster": sc.Name,
						},
					},
				}

				return client.NetworkingV1().Ingresses(sc.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
			},
			updateFunc: func(ctx context.Context, client kubernetes.Interface, obj metav1.Object) (metav1.Object, error) {
				ingress := obj.(*networkingv1.Ingress)
				return client.NetworkingV1().Ingresses(ingress.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
			},
			getFunc: func(ctx context.Context, client kubernetes.Interface, namespace, name string) (metav1.Object, error) {
				return client.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
			},
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sc := &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sc",
					UID:       types.UID("cluster-uuid"),
					Namespace: "ns",
				},
				Spec:   scyllav1.ScyllaClusterSpec{},
				Status: scyllav1.ScyllaClusterStatus{},
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			kubeClient := kubefake.NewSimpleClientset()
			scyllaClient := scyllafake.NewSimpleClientset()

			kubeInformer := informers.NewSharedInformerFactory(kubeClient, 0)
			scyllaInformer := scyllainformers.NewSharedInformerFactory(scyllaClient, 0)

			controller, err := NewController(
				kubeClient,
				scyllaClient.ScyllaV1(),
				kubeInformer.Core().V1().Services(),
				kubeInformer.Core().V1().Secrets(),
				kubeInformer.Core().V1().ConfigMaps(),
				kubeInformer.Core().V1().ServiceAccounts(),
				kubeInformer.Rbac().V1().RoleBindings(),
				kubeInformer.Apps().V1().StatefulSets(),
				kubeInformer.Policy().V1().PodDisruptionBudgets(),
				kubeInformer.Networking().V1().Ingresses(),
				scyllaInformer.Scylla().V1().ScyllaClusters(),
			)

			if err != nil {
				t.Fatal(err)
			}

			go scyllaInformer.Start(ctx.Done())
			go kubeInformer.Start(ctx.Done())
			go controller.Run(ctx, 1)

			sc, err = scyllaClient.ScyllaV1().ScyllaClusters(sc.Namespace).Create(ctx, sc, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}

			obj, err := tc.createFunc(ctx, kubeClient, sc)
			if err != nil {
				t.Fatal(err)
			}

			obj.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(sc, scyllav1.GroupVersion.WithKind("ScyllaCluster"))})
			updatedObj, err := tc.updateFunc(ctx, kubeClient, obj)
			if err != nil {
				t.Fatal(err)
			}
			if len(updatedObj.GetOwnerReferences()) != 1 {
				t.Fatalf("expected 1 owner reference in object, got %d", len(updatedObj.GetOwnerReferences()))
			}
			if !metav1.IsControlledBy(updatedObj, sc) {
				t.Fatalf("expected object to be owned by scyllav1.ScyllaCluster, but it's by: %v", metav1.GetControllerOfNoCopy(updatedObj))
			}

			err = wait.PollImmediate(10*time.Millisecond, time.Second, func() (done bool, err error) {
				obj, err := tc.getFunc(ctx, kubeClient, obj.GetNamespace(), obj.GetName())
				if err != nil {
					t.Fatal(err)
				}
				return len(obj.GetOwnerReferences()) == 0, nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
