package scylladbdatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaDBDatacenter webhook", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladbdatacenter")

	g.It("should forbid invalid requests", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		validSDC := f.GetDefaultScyllaDBDatacenter()

		framework.By("Accepting a creation of a valid ScyllaDBDatacenter")

		validSDC, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Create(ctx, validSDC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func(name string) {
			err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Delete(ctx, name, metav1.DeleteOptions{})
			if !apierrors.IsNotFound(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}(validSDC.Name)

		framework.By("Accepting a deletion of a valid ScyllaDBDatacenter")
		err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Delete(ctx, validSDC.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &validSDC.UID,
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Rejecting a creation of ScyllaDBDatacenter with invalid min ready seconds")
		invalidSDC := validSDC.DeepCopy()
		invalidSDC.Spec.MinReadySeconds = pointer.Ptr[int32](-123)
		_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(f.Namespace()).Create(ctx, invalidSDC, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&apierrors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDBDatacenter.scylla.scylladb.com %q is invalid: spec.minReadySeconds: Invalid value: -123: must be greater than or equal to 0`, invalidSDC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  invalidSDC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaDBDatacenter",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueInvalid",
						Message: `Invalid value: -123: must be greater than or equal to 0`,
						Field:   "spec.minReadySeconds",
					},
				},
			},
			Code: 422,
		}}))
	})
})
