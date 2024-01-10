// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/klog/v2"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should claim preexisting member ServiceAccount and RoleBinding", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Name = names.SimpleNameGenerator.GenerateName(sc.GenerateName)
		sc.GenerateName = ""

		framework.By("Creating a ServiceAccount and a RoleBinding")
		sa, err := f.KubeClient().CoreV1().ServiceAccounts(f.Namespace()).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-member", sc.Name),
				Annotations: map[string]string{
					"user-annotation": "123",
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sa.OwnerReferences).To(o.HaveLen(0))

		rb, err := f.KubeClient().RbacV1().RoleBindings(f.Namespace()).Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: sa.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					APIGroup:  corev1.GroupName,
					Kind:      rbacv1.ServiceAccountKind,
					Namespace: sa.Namespace,
					Name:      sa.Name,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "scyllacluster-member",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sa.OwnerReferences).To(o.HaveLen(0))

		framework.By("Creating a ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ServiceAccount to be adopted")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sa, err = controllerhelpers.WaitForServiceAccountState(waitCtx1, f.KubeClient().CoreV1().ServiceAccounts(sa.Namespace), sa.Name, controllerhelpers.WaitForStateOptions{}, func(sa *corev1.ServiceAccount) (bool, error) {
			ref := metav1.GetControllerOfNoCopy(sa)
			if ref == nil {
				klog.V(2).InfoS("No controller ref")
				return false, nil
			}

			if ref.UID == sc.UID {
				return true, nil
			}

			klog.Error("Foreign controller has claimed the object", "ControllerRef", ref)

			return true, fmt.Errorf("foreign controller has claimed the object")
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sa.OwnerReferences).To(o.HaveLen(1))
		o.Expect(sa.Annotations).To(o.HaveKeyWithValue("user-annotation", "123"))

		framework.By("Waiting for the RoleBinding to be adopted")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		rb, err = controllerhelpers.WaitForRoleBindingState(waitCtx2, f.KubeClient().RbacV1().RoleBindings(rb.Namespace), rb.Name, controllerhelpers.WaitForStateOptions{}, func(sa *rbacv1.RoleBinding) (bool, error) {
			ref := metav1.GetControllerOfNoCopy(sa)
			if ref == nil {
				return false, nil
			}

			if ref.UID == sc.UID {
				return true, nil
			}

			klog.Error("Foreign controller has claimed the object", "ControllerRef", ref)

			return true, fmt.Errorf("foreign controller has claimed the object")
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rb.OwnerReferences).To(o.HaveLen(1))
	})
})
