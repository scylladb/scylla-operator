package scylladbmonitoring

import (
	"fmt"
	"reflect"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func Test_indexScyllaDBMonitoringBySecret(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name    string
		obj     interface{}
		want    []string
		wantErr error
	}{
		{
			name:    "unexpected object type",
			obj:     corev1.Pod{},
			want:    nil,
			wantErr: fmt.Errorf("expected *scyllav1alpha1.ScyllaDBMonitoring, got v1.Pod"),
		},
		{
			name:    "no secret references",
			obj:     &scyllav1alpha1.ScyllaDBMonitoring{},
			want:    nil,
			wantErr: nil,
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
			want:    []string{"bearer-token-secret", "client-tls-secret"},
			wantErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := indexScyllaDBMonitoringBySecret(tc.obj)
			if !reflect.DeepEqual(err, tc.wantErr) {
				t.Errorf("indexScyllaDBMonitoringBySecret() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("indexScyllaDBMonitoringBySecret() got = %v, want %v", got, tc.want)
			}
		})
	}
}

func Test_indexScyllaDBMonitoringByConfigMap(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name    string
		obj     interface{}
		want    []string
		wantErr error
	}{
		{
			name:    "unexpected object type",
			obj:     corev1.Pod{},
			want:    nil,
			wantErr: fmt.Errorf("expected *scyllav1alpha1.ScyllaDBMonitoring, got v1.Pod"),
		},
		{
			name:    "no configmap references",
			obj:     &scyllav1alpha1.ScyllaDBMonitoring{},
			want:    nil,
			wantErr: nil,
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
			want:    []string{"ca-cert-configmap"},
			wantErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := indexScyllaDBMonitoringByConfigMap(tc.obj)
			if !reflect.DeepEqual(err, tc.wantErr) {
				t.Errorf("indexScyllaDBMonitoringByConfigMap() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("indexScyllaDBMonitoringByConfigMap() got = %v, want %v", got, tc.want)
			}
		})
	}
}
