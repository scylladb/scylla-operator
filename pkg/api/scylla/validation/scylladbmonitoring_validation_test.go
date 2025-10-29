package validation

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type validateScyllaDBMonitoringTestCase struct {
	name              string
	sm                *scyllav1alpha1.ScyllaDBMonitoring
	expectedErrorList field.ErrorList
}

func validScyllaDBMonitoringWithExternalPrometheus() *scyllav1alpha1.ScyllaDBMonitoring {
	return &scyllav1alpha1.ScyllaDBMonitoring{
		Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
			Components: &scyllav1alpha1.Components{
				Prometheus: &scyllav1alpha1.PrometheusSpec{
					Mode: scyllav1alpha1.PrometheusModeExternal,
				},
				Grafana: &scyllav1alpha1.GrafanaSpec{
					Datasources: []scyllav1alpha1.GrafanaDatasourceSpec{
						{
							Name: "prometheus",
							Type: scyllav1alpha1.GrafanaDatasourceTypePrometheus,
							URL:  "https://external-prom:9090",
							PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
								TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
									CACertConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
										Name: "ca-cert-configmap",
										Key:  "ca-cert.pem",
									},
									InsecureSkipVerify: false,
									ClientTLSKeyPairSecretRef: &scyllav1alpha1.LocalObjectReference{
										Name: "client-tls-secret",
									},
								},
								Auth: &scyllav1alpha1.GrafanaPrometheusDatasourceAuthSpec{
									Type: scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken,
									BearerTokenOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceBearerTokenAuthOptions{
										SecretRef: &scyllav1alpha1.LocalObjectKeySelector{
											Name: "bearer-token-secret",
											Key:  "token-key",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func validateScyllaDBMonitoringTestCases() []validateScyllaDBMonitoringTestCase {
	return []validateScyllaDBMonitoringTestCase{
		{
			name: "default monitoring",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Components: &scyllav1alpha1.Components{},
				},
			},
			expectedErrorList: nil,
		},
		{
			name:              "valid monitoring with external prometheus",
			sm:                validScyllaDBMonitoringWithExternalPrometheus(),
			expectedErrorList: nil,
		},
		{
			name: "invalid monitoring with datasources when prometheus is managed",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeManaged
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.components.grafana.datasources",
					BadValue: "",
					Detail:   "must not be specified when Prometheus is in Managed mode",
				},
			},
		},
		{
			name: "invalid monitoring with unsupported prometheus mode",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.Mode = "unsupported-mode"
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.components.prometheus.mode",
					BadValue: "unsupported-mode",
					Detail:   `supported values: "Managed", "External"`,
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus and missing grafana",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana = nil
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana",
					BadValue: "",
					Detail:   "must be specified when Prometheus is in External mode",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus and missing datasource",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources = nil
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources",
					BadValue: "",
					Detail:   "exactly one datasource must be specified when Prometheus is in External mode",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus and too many datasources",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources = append(sm.Spec.Components.Grafana.Datasources, scyllav1alpha1.GrafanaDatasourceSpec{
					Name:              "prometheus",
					Type:              scyllav1alpha1.GrafanaDatasourceTypePrometheus,
					URL:               "https://another-prom:9090",
					PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{},
				})
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeTooMany,
					Field:    "spec.components.grafana.datasources",
					BadValue: 2,
					Detail:   "must have at most 1 item",
				},
			},
		},
		{
			name: "invalid monitoring with unsupported datasource type",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions = nil
				sm.Spec.Components.Grafana.Datasources[0].Type = "unsupported-type"
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.components.grafana.datasources[0].type",
					BadValue: "unsupported-type",
					Detail:   `supported values: "Prometheus"`,
				},
			},
		},
		{
			name: "invalid monitoring with unsupported datasource name",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].Name = "unsupported-name"
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.components.grafana.datasources[0].name",
					BadValue: "unsupported-name",
					Detail:   `supported values: "prometheus"`,
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus and missing datasource URL",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].URL = ""
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].url",
					BadValue: "",
					Detail:   "must be specified",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus and unsupported auth type",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.Auth.Type = "unsupported-auth-type"
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.auth.type",
					BadValue: "unsupported-auth-type",
					Detail:   `supported values: "NoAuthentication", "BearerToken"`,
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus, BearerToken auth type and missing secret ref",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.Auth.BearerTokenOptions.SecretRef = nil
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.auth.bearerTokenOptions.secretRef",
					BadValue: "",
					Detail:   "must be specified for BearerToken auth",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus, BearerToken auth type and empty secret ref name",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.Auth.BearerTokenOptions.SecretRef.Name = ""
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.auth.bearerTokenOptions.secretRef.name",
					BadValue: "",
					Detail:   "must be specified",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus, BearerToken auth type and empty secret ref key",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.Auth.BearerTokenOptions.SecretRef.Key = ""
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.auth.bearerTokenOptions.secretRef.key",
					BadValue: "",
					Detail:   "must be specified",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus, TLS client cert/key secret ref and empty name",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.TLS.ClientTLSKeyPairSecretRef.Name = ""
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.tls.clientTLSKeyPairSecretRef.name",
					BadValue: "",
					Detail:   "must be specified",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus, TLS CA cert config map ref and empty name",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.TLS.CACertConfigMapRef.Name = ""
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.tls.caCertConfigMapRef.name",
					BadValue: "",
					Detail:   "must be specified",
				},
			},
		},
		{
			name: "invalid monitoring with external prometheus, TLS CA cert config map ref and empty key",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].PrometheusOptions.TLS.CACertConfigMapRef.Key = ""
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions.tls.caCertConfigMapRef.key",
					BadValue: "",
					Detail:   "must be specified",
				},
			},
		},
		{
			name: "invalid monitoring with unsupported datasource type and prometheus options specified",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.Datasources[0].Type = "other-type"
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeNotSupported,
					Field:    "spec.components.grafana.datasources[0].type",
					BadValue: "other-type",
					Detail:   `supported values: "Prometheus"`,
				},
				{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.components.grafana.datasources[0].prometheusOptions",
					BadValue: "",
					Detail:   "must not be specified when datasource type is not Prometheus",
				},
			},
		},
	}
}

func TestValidateScyllaDBMonitoring(t *testing.T) {
	t.Parallel()

	for _, tc := range validateScyllaDBMonitoringTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualErrorList := ValidateScyllaDBMonitoring(tc.sm)
			if !reflect.DeepEqual(actualErrorList, tc.expectedErrorList) {
				t.Errorf("expected error list differs from actual: %s", cmp.Diff(tc.expectedErrorList, actualErrorList))
			}
		})
	}
}

func TestValidateScyllaDBMonitoringUpdate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		oldSm             *scyllav1alpha1.ScyllaDBMonitoring
		newSm             *scyllav1alpha1.ScyllaDBMonitoring
		expectedErrorList field.ErrorList
	}{
		{
			name:              "no changes",
			oldSm:             validScyllaDBMonitoringWithExternalPrometheus(),
			newSm:             validScyllaDBMonitoringWithExternalPrometheus(),
			expectedErrorList: nil,
		},
		{
			name:  "change prometheus mode",
			oldSm: validScyllaDBMonitoringWithExternalPrometheus(),
			newSm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana = nil // Grafana must be nil in managed mode.
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeManaged
				return sm
			}(),
			expectedErrorList: field.ErrorList{
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.components.prometheus.mode",
					BadValue: scyllav1alpha1.PrometheusModeExternal,
					Detail:   "is immutable and cannot be changed",
				},
			},
		},
	}

	// Map regular test cases to the update test cases to ensure validation rules are wired up correctly.
	regularTestCases := validateScyllaDBMonitoringTestCases()
	for _, rtc := range regularTestCases {
		tt = append(tt, struct {
			name              string
			oldSm             *scyllav1alpha1.ScyllaDBMonitoring
			newSm             *scyllav1alpha1.ScyllaDBMonitoring
			expectedErrorList field.ErrorList
		}{
			name:              "update - " + rtc.name,
			oldSm:             rtc.sm,
			newSm:             rtc.sm,
			expectedErrorList: rtc.expectedErrorList,
		})
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualErrorList := ValidateScyllaDBMonitoringUpdate(tc.newSm, tc.oldSm)
			if !reflect.DeepEqual(actualErrorList, tc.expectedErrorList) {
				t.Errorf("expected error list differs from actual: %s", cmp.Diff(tc.expectedErrorList, actualErrorList))
			}
		})
	}
}

func TestGetWarningsOnScyllaDBMonitoringCreate(t *testing.T) {
	t.Parallel()
	sm := validScyllaDBMonitoringWithExternalPrometheus()

	testCases := []struct {
		name            string
		sm              *scyllav1alpha1.ScyllaDBMonitoring
		expectedWarning []string
	}{
		{
			name:            "no warnings on valid monitoring",
			sm:              sm,
			expectedWarning: nil,
		},
		{
			name: "warning on spec.components.grafana.exposeOptions",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.ExposeOptions = &scyllav1alpha1.GrafanaExposeOptions{
					WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
						Ingress: &scyllav1alpha1.IngressOptions{
							IngressClassName: "haproxy",
						},
					},
				}
				return sm
			}(),
			expectedWarning: []string{
				"`spec.components.grafana.exposeOptions` field is deprecated and will be removed in future releases, please expose managed Grafana Service on your own (e.g., via Ingress or HTTPRoute).",
			},
		},
		{
			name: "warning on spec.components.prometheus.mode = Managed",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeManaged
				return sm
			}(),
			expectedWarning: []string{
				"`spec.components.prometheus.mode` = `Managed` is deprecated and will be removed in future releases, please use `External` instead.",
			},
		},
		{
			name: "warning on spec.components.prometheus.exposeOptions",
			sm: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.ExposeOptions = &scyllav1alpha1.PrometheusExposeOptions{
					WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
						Ingress: &scyllav1alpha1.IngressOptions{
							IngressClassName: "haproxy",
						},
					},
				}
				return sm
			}(),
			expectedWarning: []string{
				"`spec.components.prometheus.exposeOptions` field is deprecated and will be removed in future releases, please expose managed Prometheus Service on your own (e.g., via Ingress or HTTPRoute).",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualWarnings := GetWarningsOnScyllaDBMonitoringCreate(tc.sm)
			if !reflect.DeepEqual(actualWarnings, tc.expectedWarning) {
				t.Errorf("expected warnings differ from actual: %s", cmp.Diff(tc.expectedWarning, actualWarnings))
			}
		})
	}
}

func TestGetWarningsOnScyllaDBMonitoringUpdate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		old             *scyllav1alpha1.ScyllaDBMonitoring
		new             *scyllav1alpha1.ScyllaDBMonitoring
		expectedWarning []string
	}{
		{
			name:            "no warnings on valid monitoring",
			old:             validScyllaDBMonitoringWithExternalPrometheus(),
			new:             validScyllaDBMonitoringWithExternalPrometheus(),
			expectedWarning: nil,
		},
		{
			name: "no warning on non-nil spec.components.grafana.exposeOptions in old monitoring",
			old: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.ExposeOptions = &scyllav1alpha1.GrafanaExposeOptions{
					WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
						Ingress: &scyllav1alpha1.IngressOptions{
							IngressClassName: "haproxy",
						},
					},
				}
				return sm
			}(),
			new:             validScyllaDBMonitoringWithExternalPrometheus(),
			expectedWarning: nil,
		},
		{
			name: "warning on non-nil spec.components.grafana.exposeOptions in new monitoring",
			old:  validScyllaDBMonitoringWithExternalPrometheus(),
			new: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Grafana.ExposeOptions = &scyllav1alpha1.GrafanaExposeOptions{
					WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
						Ingress: &scyllav1alpha1.IngressOptions{
							IngressClassName: "haproxy",
						},
					},
				}
				return sm
			}(),
			expectedWarning: []string{
				"`spec.components.grafana.exposeOptions` field is deprecated and will be removed in future releases, please expose managed Grafana Service on your own (e.g., via Ingress or HTTPRoute).",
			},
		},
		{
			name: "no warning on spec.components.prometheus.mode Managed in old monitoring",
			old: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeManaged
				return sm
			}(),
			new:             validScyllaDBMonitoringWithExternalPrometheus(),
			expectedWarning: nil,
		},
		{
			name: "warning on spec.components.prometheus.mode Managed in new monitoring",
			old:  validScyllaDBMonitoringWithExternalPrometheus(),
			new: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeManaged
				return sm
			}(),
			expectedWarning: []string{
				"`spec.components.prometheus.mode` = `Managed` is deprecated and will be removed in future releases, please use `External` instead.",
			},
		},
		{
			name: "no warning on non-nil spec.components.prometheus.exposeOptions in old monitoring",
			old: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.ExposeOptions = &scyllav1alpha1.PrometheusExposeOptions{
					WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
						Ingress: &scyllav1alpha1.IngressOptions{
							IngressClassName: "haproxy",
						},
					},
				}
				return sm
			}(),
			new:             validScyllaDBMonitoringWithExternalPrometheus(),
			expectedWarning: nil,
		},
		{
			name: "warning on non-nil spec.components.prometheus.exposeOptions in new monitoring",
			old:  validScyllaDBMonitoringWithExternalPrometheus(),
			new: func() *scyllav1alpha1.ScyllaDBMonitoring {
				sm := validScyllaDBMonitoringWithExternalPrometheus()
				sm.Spec.Components.Prometheus.ExposeOptions = &scyllav1alpha1.PrometheusExposeOptions{
					WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
						Ingress: &scyllav1alpha1.IngressOptions{
							IngressClassName: "haproxy",
						},
					},
				}
				return sm
			}(),
			expectedWarning: []string{
				"`spec.components.prometheus.exposeOptions` field is deprecated and will be removed in future releases, please expose managed Prometheus Service on your own (e.g., via Ingress or HTTPRoute).",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualWarnings := GetWarningsOnScyllaDBMonitoringUpdate(tc.new, tc.old)
			if !reflect.DeepEqual(actualWarnings, tc.expectedWarning) {
				t.Errorf("expected warnings differ from actual: %s", cmp.Diff(tc.expectedWarning, actualWarnings))
			}
		})
	}
}
