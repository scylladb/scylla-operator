// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import (
	"bytes"
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllav2alpha1fixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v2alpha1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdapilatest "k8s.io/client-go/tools/clientcmd/api/latest"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaCluster scaling", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.DescribeTable("should support horizontal rack scaling",
		func(makeScyllaCluster func() (*v2alpha1.ScyllaCluster, func())) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			sc, cleanup := makeScyllaCluster()
			defer cleanup()

			framework.By("Creating a ScyllaCluster")
			sc, err := f.ScyllaClient().ScyllaV2alpha1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// TODO: Remove once we have a multi region GC controller
			defer func() {
				err = f.ScyllaClient().ScyllaV2alpha1().ScyllaClusters(sc.Namespace).Delete(ctx, sc.Name, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				for _, dc := range sc.Spec.Datacenters {
					if dc.RemoteKubeClusterConfigRef == nil {
						continue
					}
					remoteNamespace := naming.RemoteNamespace(sc, dc)
					err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDatacenters(remoteNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(map[string]string{
							naming.ParentClusterNameLabel:           sc.Name,
							naming.ParentClusterNamespaceLabel:      sc.Namespace,
							naming.ParentClusterDatacenterNameLabel: dc.Name,
						}).String(),
					})
					o.Expect(err).NotTo(o.HaveOccurred())

					err = f.KubeAdminClient().CoreV1().Namespaces().Delete(ctx, remoteNamespace, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				err = framework.WaitForObjectDeletion(ctx, f.DynamicAdminClient(), v2alpha1.GroupVersion.WithResource("scyllaclusters"), f.Namespace(), sc.Name, &sc.UID)
				o.Expect(err).NotTo(o.HaveOccurred())
			}()

			framework.By("Waiting for the ScyllaCluster to rollout")
			waitCtx1, waitCtx1Cancel := v2alpha1utils.ContextForScyllaClusterRollout(ctx, sc)
			defer waitCtx1Cancel()
			sc, err = v2alpha1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV2alpha1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v2alpha1utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Grant e2e user permissions in remote namespaces
			for _, dc := range sc.Spec.Datacenters {
				if dc.RemoteKubeClusterConfigRef == nil {
					continue
				}
				remoteNamespace := naming.RemoteNamespace(sc, dc)
				_, err = f.KubeAdminClient().RbacV1().RoleBindings(remoteNamespace).Create(ctx, &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: remoteNamespace,
					},
					Subjects: []rbacv1.Subject{
						{
							APIGroup:  corev1.GroupName,
							Kind:      rbacv1.ServiceAccountKind,
							Namespace: f.Namespace(),
							Name:      framework.ServiceAccountName,
						},
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "admin",
					},
				}, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			verifyScyllaCluster(ctx, f.ScyllaClient(), sc)

			framework.By("Scaling first datacenter to 2 nodes per rack")
			sc.Spec.Datacenters[0].NodesPerRack = pointer.Int32(2)
			sc, err = f.ScyllaClient().ScyllaV2alpha1().ScyllaClusters(f.Namespace()).Patch(
				ctx,
				sc.Name,
				types.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/datacenters/0/nodesPerRack", "value": 2}]`),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sc.Spec.Datacenters[0].NodesPerRack).ToNot(o.BeNil())
			o.Expect(*sc.Spec.Datacenters[0].NodesPerRack).To(o.Equal(int32(2)))

			framework.By("Waiting for the ScyllaCluster to rollout")
			waitCtx2, waitCtx2Cancel := v2alpha1utils.ContextForScyllaClusterRollout(ctx, sc)
			defer waitCtx2Cancel()
			sc, err = v2alpha1utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV2alpha1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v2alpha1utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaCluster(ctx, f.ScyllaClient(), sc)
		},
		g.Entry("remote cluster", func() (*v2alpha1.ScyllaCluster, func()) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			sa := newServiceAccountToSelf(ctx, f.KubeAdminClient(), "default")
			kubeConfigSecret := newSecretWithKubeconfig(ctx, f.KubeAdminClient(), sa)

			rkcc := &v1alpha1.RemoteKubeClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "self-",
				},
				Spec: v1alpha1.RemoteKubeClusterConfigSpec{
					KubeConfigSecretRef: &corev1.SecretReference{
						Name:      kubeConfigSecret.Name,
						Namespace: kubeConfigSecret.Namespace,
					},
				},
			}

			rkcc, err := f.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubeClusterConfigs().Create(ctx, rkcc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			sc := scyllav2alpha1fixture.SingleDCScyllaCluster.ReadOrFail()
			sc.Spec.Datacenters[0].RemoteKubeClusterConfigRef = &v2alpha1.RemoteKubeClusterConfigRef{Name: rkcc.Name}
			sc.Spec.Datacenters[0].NodesPerRack = pointer.Int32(1)

			cleanup := func() {
				err := f.KubeAdminClient().CoreV1().Secrets("default").Delete(context.TODO(), kubeConfigSecret.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &kubeConfigSecret.UID,
					},
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				err = f.KubeAdminClient().CoreV1().ServiceAccounts("default").Delete(context.TODO(), sa.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &sa.UID,
					},
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				err = f.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubeClusterConfigs().Delete(context.TODO(), rkcc.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &rkcc.UID,
					},
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			return sc, cleanup
		}),
		g.Entry("local cluster", func() (*v2alpha1.ScyllaCluster, func()) {
			sc := scyllav2alpha1fixture.SingleDCScyllaCluster.ReadOrFail()
			return sc, func() {}
		}),
	)
})

func newServiceAccountToSelf(ctx context.Context, adminClient kubernetes.Interface, namespace string) *corev1.ServiceAccount {
	const (
		serviceAccountWaitTimeout = time.Minute
	)

	userSA, err := adminClient.CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "remote-client-",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	_, err = adminClient.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: userSA.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: userSA.Namespace,
				Name:      userSA.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "scylladb:controller:operator-multiregion",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	ctxUserSa, ctxUserSaCancel := watchtools.ContextWithOptionalTimeout(ctx, serviceAccountWaitTimeout)
	defer ctxUserSaCancel()
	userSA, err = framework.WaitForServiceAccount(ctxUserSa, adminClient.CoreV1(), userSA.Namespace, userSA.Name)
	o.Expect(err).NotTo(o.HaveOccurred())

	return userSA
}

func newSecretWithKubeconfig(ctx context.Context, adminClient kubernetes.Interface, sa *corev1.ServiceAccount) *corev1.Secret {
	var token, ca []byte
	for _, secretName := range sa.Secrets {
		secret, err := adminClient.CoreV1().Secrets(sa.Namespace).Get(ctx, secretName.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if secret.Type == corev1.SecretTypeServiceAccountToken {
			name := secret.Annotations[corev1.ServiceAccountNameKey]
			uid := secret.Annotations[corev1.ServiceAccountUIDKey]
			if name == sa.Name && uid == string(sa.UID) {
				token = secret.Data[corev1.ServiceAccountTokenKey]
				ca = secret.Data[corev1.ServiceAccountRootCAKey]
				break
			}
		}
	}
	o.Expect(token).NotTo(o.HaveLen(0))
	o.Expect(ca).NotTo(o.HaveLen(0))

	kubeConfig := &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"default": {
				Server:                   framework.TestContext.RestConfig.Host,
				CertificateAuthorityData: ca,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"default": {
				Token: string(token),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:   "default",
				AuthInfo:  "default",
				Namespace: sa.Namespace,
			},
		},
		CurrentContext: "default",
	}
	buf := &bytes.Buffer{}
	err := clientcmdapilatest.Codec.Encode(kubeConfig, buf)
	o.Expect(err).ToNot(o.HaveOccurred())

	secret, err := adminClient.CoreV1().Secrets(sa.Namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
		Data: map[string][]byte{
			naming.KubeConfigSecretKey: buf.Bytes(),
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return secret
}
