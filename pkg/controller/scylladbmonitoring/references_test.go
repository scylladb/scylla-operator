package scylladbmonitoring

import (
	"reflect"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
)

func Test_getScyllaDBMonitoringGrafanaSecretReferences(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		obj  *scyllav1alpha1.ScyllaDBMonitoring
		want []string
	}{
		{
			name: "no secret references",
			obj:  &scyllav1alpha1.ScyllaDBMonitoring{},
			want: nil,
		},
		{
			name: "all possible references",
			obj: &scyllav1alpha1.ScyllaDBMonitoring{
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Components: &scyllav1alpha1.Components{
						Grafana: &scyllav1alpha1.GrafanaSpec{
							Datasources: []scyllav1alpha1.GrafanaDatasourceSpec{
								{
									Type: scyllav1alpha1.GrafanaDatasourceTypePrometheus,
									PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
										TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
											ClientTLSKeyPairSecretRef: &scyllav1alpha1.LocalObjectReference{
												Name: "client-tls-secret",
											},
										},
										Auth: &scyllav1alpha1.GrafanaPrometheusDatasourceAuthSpec{
											Type: scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken,
											BearerTokenOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceBearerTokenAuthOptions{
												SecretRef: &scyllav1alpha1.LocalObjectKeySelector{
													Name: "bearer-token-secret",
													Key:  "token",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: []string{"bearer-token-secret", "client-tls-secret"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := getScyllaDBMonitoringGrafanaSecretReferences(tc.obj)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("getScyllaDBMonitoringGrafanaSecretReferences() got = %v, want %v", got, tc.want)
			}
		})
	}
}

func Test_getScyllaDBMonitoringGrafanaConfigMapReferences(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		obj  *scyllav1alpha1.ScyllaDBMonitoring
		want []string
	}{
		{
			name: "no configmap references",
			obj:  &scyllav1alpha1.ScyllaDBMonitoring{},
			want: nil,
		},
		{
			name: "all possible references",
			obj: &scyllav1alpha1.ScyllaDBMonitoring{
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Components: &scyllav1alpha1.Components{
						Grafana: &scyllav1alpha1.GrafanaSpec{
							Datasources: []scyllav1alpha1.GrafanaDatasourceSpec{
								{
									Type: scyllav1alpha1.GrafanaDatasourceTypePrometheus,
									PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
										TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
											CACertConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
												Name: "ca-cert-configmap",
												Key:  "ca.crt",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: []string{"ca-cert-configmap"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := getScyllaDBMonitoringGrafanaConfigMapReferences(tc.obj)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("getScyllaDBMonitoringGrafanaConfigMapReferences() got = %v, want %v", got, tc.want)
			}
		})
	}
}
