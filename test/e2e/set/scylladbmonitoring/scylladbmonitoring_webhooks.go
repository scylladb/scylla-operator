package scylladbmonitoring

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
)

var _ = g.Describe("ScyllaDBMonitoring webhook", func() {
	f := framework.NewFramework("scylladbmonitoring")

	f.AdminClient.Config = verification.RestConfigWithWarningCaptureHandler(f.AdminClient.Config)

	type entry struct {
		modifierFuncs          []func(*scyllav1alpha1.ScyllaDBMonitoring)
		expectedErrMatcherFunc func(sm *scyllav1alpha1.ScyllaDBMonitoring) o.OmegaMatcher
		expectedWarning        string
	}

	validSDBM := &scyllav1alpha1.ScyllaDBMonitoring{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.SimpleNameGenerator.GenerateName("valid-"),
		},
		Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
			Components: &scyllav1alpha1.Components{
				Prometheus: &scyllav1alpha1.PrometheusSpec{
					Mode: scyllav1alpha1.PrometheusModeExternal,
				},
				Grafana: &scyllav1alpha1.GrafanaSpec{
					Resources: corev1.ResourceRequirements{},
					Datasources: []scyllav1alpha1.GrafanaDatasourceSpec{
						{
							URL:               "https://prometheus.example:9091",
							PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{},
						},
					},
				},
			},
		},
	}

	g.DescribeTableSubtree("should respond", func(e *entry) {
		g.It("is created", func(ctx g.SpecContext) {
			warningCtx := verification.NewWarningContext(ctx)

			sm := validSDBM.DeepCopy()
			sm.Name = names.SimpleNameGenerator.GenerateName("sm-")
			for _, f := range e.modifierFuncs {
				f(sm)
			}

			framework.By("Creating a ScyllaDBMonitoring")
			_, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBMonitorings(f.Namespace()).Create(warningCtx, sm, metav1.CreateOptions{})
			o.Expect(err).To(e.expectedErrMatcherFunc(sm))
			o.Expect(warningCtx.CapturedWarning()).To(o.Equal(e.expectedWarning))
		}, g.NodeTimeout(testTimeout))

		g.It("is updated", func(ctx g.SpecContext) {
			sm := validSDBM.DeepCopy()
			sm.Name = names.SimpleNameGenerator.GenerateName("sm-")
			framework.By("Creating a ScyllaDBMonitoring")
			createdSM, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBMonitorings(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
			o.Expect(err).To(o.Succeed())

			warningCtx := verification.NewWarningContext(ctx)
			smCopy := createdSM.DeepCopy()
			for _, f := range e.modifierFuncs {
				f(smCopy)
			}
			framework.By("Updating the ScyllaDBMonitoring")
			_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBMonitorings(f.Namespace()).Update(warningCtx, smCopy, metav1.UpdateOptions{})
			o.Expect(err).To(e.expectedErrMatcherFunc(smCopy))
			o.Expect(warningCtx.CapturedWarning()).To(o.Equal(e.expectedWarning))
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
		g.Entry("with acceptance and warning when a ScyllaDBMonitoring with deprecated field", &entry{
			modifierFuncs: []func(*scyllav1alpha1.ScyllaDBMonitoring){
				func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
					sm.Spec.Components.Grafana.ExposeOptions = &scyllav1alpha1.GrafanaExposeOptions{
						WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
							Ingress: &scyllav1alpha1.IngressOptions{
								IngressClassName: "haproxy",
							},
						},
					}
				},
			},
			expectedErrMatcherFunc: func(sm *scyllav1alpha1.ScyllaDBMonitoring) o.OmegaMatcher {
				return o.Succeed()
			},
			expectedWarning: "`spec.components.grafana.exposeOptions` field is deprecated and will be removed in future releases, please expose managed Grafana Service on your own (e.g., via Ingress or HTTPRoute).",
		}),
	)
})
