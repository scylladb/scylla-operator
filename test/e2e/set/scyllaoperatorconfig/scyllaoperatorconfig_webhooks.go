package scyllaoperatorconfig

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/klog/v2"
)

var _ = g.Describe("ScyllaOperatorConfig webhook", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllaoperatorconfig")

	g.It("should forbid invalid requests", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		validSOC := &scyllav1alpha1.ScyllaOperatorConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.SimpleNameGenerator.GenerateName("valid-"),
			},
		}

		framework.By("Accepting a creation of a valid ScyllaOperatorConfig")
		klog.InfoS("ScyllaOperatorConfigs", "Name", validSOC.GetName())
		createdSOC, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Create(ctx, validSOC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func(name string) {
			err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Delete(ctx, name, metav1.DeleteOptions{})
			if !apierrors.IsNotFound(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}(createdSOC.Name)

		framework.By("Accepting a deletion of a valid ScyllaCluster")
		err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Delete(ctx, createdSOC.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &createdSOC.UID,
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Rejecting a creation of ScyllaOperatorConfig with invalid image reference")
		invalidSOC := validSOC.DeepCopy()
		invalidSOC.Spec.UnsupportedGrafanaImageOverride = pointer.Ptr("this is not a container image reference")
		_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Create(ctx, invalidSOC, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&apierrors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaOperatorConfig.scylla.scylladb.com %q is invalid: spec.unsupportedGrafanaImageOverride: Invalid value: "this is not a container image reference": unable to parse image: invalid reference format`, invalidSOC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  invalidSOC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaOperatorConfig",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueInvalid",
						Message: `Invalid value: "this is not a container image reference": unable to parse image: invalid reference format`,
						Field:   "spec.unsupportedGrafanaImageOverride",
					},
				},
			},
			Code: 422,
		}}))
	})
})
