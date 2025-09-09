package scylladbmonitoring

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
)

var _ = g.Describe("ScyllaDBMonitoring webhook", func() {
	f := framework.NewFramework("scylladbmonitoring")

	type entry struct {
		modifierFuncs          []func(*scyllav1alpha1.ScyllaDBMonitoring)
		expectedErrMatcherFunc func(sm *scyllav1alpha1.ScyllaDBMonitoring) o.OmegaMatcher
	}

	validSDBM := &scyllav1alpha1.ScyllaDBMonitoring{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.SimpleNameGenerator.GenerateName("valid-"),
		},
		Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
			Components: &scyllav1alpha1.Components{
				Prometheus: &scyllav1alpha1.PrometheusSpec{
					Mode: scyllav1alpha1.PrometheusModeManaged,
				},
				Grafana: &scyllav1alpha1.GrafanaSpec{
					Resources: corev1.ResourceRequirements{},
				},
			},
		},
	}

	g.DescribeTableSubtree("should respond", func(e *entry) {
		g.It("is created", func(ctx g.SpecContext) {
			sm := validSDBM.DeepCopy()
			sm.Name = names.SimpleNameGenerator.GenerateName("sm-")
			for _, f := range e.modifierFuncs {
				f(sm)
			}

			framework.By("Creating a ScyllaDBMonitoring")
			_, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBMonitorings(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
			o.Expect(err).To(e.expectedErrMatcherFunc(sm))
		}, g.NodeTimeout(testTimeout))

		g.It("is updated", func(ctx g.SpecContext) {
			sm := validSDBM.DeepCopy()
			sm.Name = names.SimpleNameGenerator.GenerateName("sm-")
			framework.By("Creating a ScyllaDBMonitoring")
			createdSM, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBMonitorings(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
			o.Expect(err).To(o.Succeed())

			smCopy := createdSM.DeepCopy()
			for _, f := range e.modifierFuncs {
				f(smCopy)
			}
			framework.By("Updating the ScyllaDBMonitoring")
			_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBMonitorings(f.Namespace()).Update(ctx, smCopy, metav1.UpdateOptions{})
			o.Expect(err).To(e.expectedErrMatcherFunc(smCopy))
		}, g.NodeTimeout(testTimeout))
	},
		g.Entry("with acceptance when a valid ScyllaDBMonitoring", &entry{
			modifierFuncs:          nil,
			expectedErrMatcherFunc: func(sm *scyllav1alpha1.ScyllaDBMonitoring) o.OmegaMatcher { return o.Succeed() },
		}),
		g.Entry("with rejection when an invalid ScyllaDBMonitoring", &entry{
			modifierFuncs: []func(*scyllav1alpha1.ScyllaDBMonitoring){
				func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
					sm.Spec.Components.Prometheus.Mode = "invalid-mode"
				},
			},
			expectedErrMatcherFunc: func(sm *scyllav1alpha1.ScyllaDBMonitoring) o.OmegaMatcher {
				return o.Equal(&apierrors.StatusError{ErrStatus: metav1.Status{
					Status:  "Failure",
					Message: fmt.Sprintf(`ScyllaDBMonitoring.scylla.scylladb.com %q is invalid: spec.components.prometheus.mode: Unsupported value: "invalid-mode": supported values: "Managed", "External"`, sm.Name),
					Reason:  "Invalid",
					Details: &metav1.StatusDetails{
						Name:  sm.Name,
						Group: "scylla.scylladb.com",
						Kind:  "ScyllaDBMonitoring",
						UID:   "",
						Causes: []metav1.StatusCause{
							{
								Type:    "FieldValueNotSupported",
								Message: `Unsupported value: "invalid-mode": supported values: "Managed", "External"`,
								Field:   "spec.components.prometheus.mode",
							},
						},
					},
					Code: 422,
				}})
			},
		}),
	)
})
