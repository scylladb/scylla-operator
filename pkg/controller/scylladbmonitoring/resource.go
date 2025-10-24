package scylladbmonitoring

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
)

// grafanaPrometheusDatasourceSpec holds the spec of the Prometheus datasource used by Grafana.
// It describes both external (user-defined) and operator managed Prometheus.
// This struct is used as a template data structure to render the actual deployment and configmap.
type grafanaPrometheusDatasourceSpec struct {
	URL  string
	TLS  *grafanaPrometheusDatasourceTLSSpec
	Auth *grafanaPrometheusDatasourceAuthSpec
}

// grafanaPrometheusDatasourceTLSSpec holds TLS spec used by Grafana to connect to Prometheus.
// It describes both external (user-defined) and operator managed TLS.
// This struct is used as a template data structure to render the actual deployment and configmap.
type grafanaPrometheusDatasourceTLSSpec struct {
	ClientTLSKeyPairSecretRef *scyllav1alpha1.LocalObjectReference
	ServingCAConfigMapRef     *scyllav1alpha1.LocalObjectKeySelector
	InsecureSkipVerify        bool
}

// grafanaPrometheusDatasourceAuthSpec holds authentication spec used by Grafana to connect to Prometheus.
// This struct is used as a template data structure to render the actual deployment and configmap.
type grafanaPrometheusDatasourceAuthSpec struct {
	BearerTokenSecretRef *scyllav1alpha1.LocalObjectKeySelector
}

func makeGrafanaPrometheusDatasourceSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) (*grafanaPrometheusDatasourceSpec, error) {
	tls, err := makeGrafanaPrometheusDatasourceTLSSpec(sm)
	if err != nil {
		return nil, fmt.Errorf("can't make Grafana Prometheus datasource TLS spec: %w", err)
	}

	return &grafanaPrometheusDatasourceSpec{
		URL:  makeGrafanaDatasourceURL(sm),
		TLS:  tls,
		Auth: makeGrafanaPrometheusDatasourceAuthSpec(sm),
	}, nil
}

func makeGrafanaPrometheusDatasourceAuthSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *grafanaPrometheusDatasourceAuthSpec {
	ds := makePrometheusDatasourceSpec(sm)
	if ds != nil {
		if ds.PrometheusOptions.Auth != nil && ds.PrometheusOptions.Auth.Type == scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken {
			return &grafanaPrometheusDatasourceAuthSpec{
				BearerTokenSecretRef: ds.PrometheusOptions.Auth.BearerTokenOptions.SecretRef,
			}
		}
	}

	// By default, both for managed and external Prometheus, no auth is used.
	return nil
}

func makeGrafanaPrometheusDatasourceTLSSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) (*grafanaPrometheusDatasourceTLSSpec, error) {
	ds := makePrometheusDatasourceSpec(sm)
	if ds != nil {
		// If the user provided a Prometheus datasource with TLS config, use it.
		if ds.PrometheusOptions.TLS != nil {
			tls := ds.PrometheusOptions.TLS
			return &grafanaPrometheusDatasourceTLSSpec{
				ClientTLSKeyPairSecretRef: tls.ClientTLSKeyPairSecretRef,
				ServingCAConfigMapRef:     tls.CACertConfigMapRef,
				InsecureSkipVerify:        tls.InsecureSkipVerify,
			}, nil
		}

		// If the user provided a Prometheus datasource without TLS config, assume no TLS.
		return nil, nil
	}

	managedPrometheusClientGrafanaSecretName, err := naming.ManagedPrometheusClientGrafanaSecretName(sm.Name)
	if err != nil {
		return nil, fmt.Errorf("can't get managed Prometheus client Grafana secret name: %w", err)
	}

	managedPrometheusServiceCAConfigMapName, err := naming.ManagedPrometheusServingCAConfigMapName(sm.Name)
	if err != nil {
		return nil, fmt.Errorf("can't get managed Prometheus serving CA config map name: %w", err)
	}

	// By default, use operator managed TLS certs.
	return &grafanaPrometheusDatasourceTLSSpec{
		ClientTLSKeyPairSecretRef: &scyllav1alpha1.LocalObjectReference{
			Name: managedPrometheusClientGrafanaSecretName,
		},
		ServingCAConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
			Name: managedPrometheusServiceCAConfigMapName,
			Key:  "ca-bundle.crt",
		},
		InsecureSkipVerify: false,
	}, nil
}

func makeGrafanaDatasourceURL(sm *scyllav1alpha1.ScyllaDBMonitoring) string {
	if spec := getGrafanaSpec(sm); spec != nil && len(spec.Datasources) > 0 {
		// We only support one datasource for now.
		ds := spec.Datasources[0]
		return ds.URL
	}

	// Default to managed Prometheus service.
	return "https://" + sm.Name + "-prometheus:9090"
}

func makePrometheusDatasourceSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.GrafanaDatasourceSpec {
	if spec := getGrafanaSpec(sm); spec != nil && len(spec.Datasources) > 0 {
		// We only support one datasource for now.
		ds := spec.Datasources[0]
		if ds.Type == scyllav1alpha1.GrafanaDatasourceTypePrometheus {
			return &ds
		}
	}
	return nil
}
