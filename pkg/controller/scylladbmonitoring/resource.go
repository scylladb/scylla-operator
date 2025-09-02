package scylladbmonitoring

import scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"

// grafanaPrometheusDatasourceSpec holds the spec of the Prometheus datasource used by Grafana.
type grafanaPrometheusDatasourceSpec struct {
	URL  string
	TLS  *grafanaPrometheusDatasourceTLSSpec
	Auth *grafanaPrometheusDatasourceAuthSpec
}

// grafanaPrometheusDatasourceTLSSpec holds TLS spec used by Grafana to connect to Prometheus.
// It's either user provided or managed by us, depending on the ScyllaDBMonitoring spec.
type grafanaPrometheusDatasourceTLSSpec struct {
	ClientTLSKeyPairSecretRef *scyllav1alpha1.LocalObjectReference
	ServingCAConfigMapRef     *scyllav1alpha1.LocalObjectKeySelector
	InsecureSkipVerify        bool
}

// grafanaPrometheusDatasourceAuthSpec holds authentication spec used by Grafana to connect to Prometheus.
type grafanaPrometheusDatasourceAuthSpec struct {
	BearerTokenSecretRef *scyllav1alpha1.LocalObjectKeySelector
}

// getGrafanaPrometheusDatasourceSpec returns the Prometheus datasource spec based on the ScyllaDBMonitoring spec.
// It handles both external (user-defined) and operator managed datasource.
func getGrafanaPrometheusDatasourceSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *grafanaPrometheusDatasourceSpec {
	return &grafanaPrometheusDatasourceSpec{
		URL:  getGrafanaDatasourceURL(sm),
		TLS:  getGrafanaPrometheusDatasourceTLSSpec(sm),
		Auth: getGrafanaPrometheusDatasourceAuthSpec(sm),
	}
}

func getGrafanaPrometheusDatasourceAuthSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *grafanaPrometheusDatasourceAuthSpec {
	ds := getPrometheusDatasourceSpec(sm)
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

func getGrafanaPrometheusDatasourceTLSSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *grafanaPrometheusDatasourceTLSSpec {
	ds := getPrometheusDatasourceSpec(sm)
	if ds != nil {
		// If the user provided a Prometheus datasource with TLS config, use it.
		if ds.PrometheusOptions.TLS != nil {
			tls := ds.PrometheusOptions.TLS
			return &grafanaPrometheusDatasourceTLSSpec{
				ClientTLSKeyPairSecretRef: tls.ClientTLSKeyPairSecretRef,
				ServingCAConfigMapRef:     tls.CACertConfigMapRef,
				InsecureSkipVerify:        tls.InsecureSkipVerify,
			}
		}

		// If the user provided a Prometheus datasource without TLS config, assume no TLS.
		return nil
	}

	// By default, use operator managed TLS certs.
	return &grafanaPrometheusDatasourceTLSSpec{
		ClientTLSKeyPairSecretRef: &scyllav1alpha1.LocalObjectReference{
			Name: sm.Name + "-prometheus-client-grafana", // TODO: use naming
		},
		ServingCAConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
			Name: sm.Name + "-prometheus-serving-ca", // TODO: use naming
			Key:  "ca-bundle.crt",
		},
		InsecureSkipVerify: false,
	}
}

func getGrafanaDatasourceURL(sm *scyllav1alpha1.ScyllaDBMonitoring) string {
	if spec := getGrafanaSpec(sm); spec != nil && len(spec.Datasources) > 0 {
		// We only support one datasource for now.
		ds := spec.Datasources[0]
		return ds.URL
	}

	// Default to managed Prometheus service.
	return "https://" + sm.Name + "-prometheus:9090"
}

// getPrometheusDatasourceSpec returns the Prometheus datasource spec if defined in the ScyllaDBMonitoring spec.
func getPrometheusDatasourceSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.GrafanaDatasourceSpec {
	if spec := getGrafanaSpec(sm); spec != nil && len(spec.Datasources) > 0 {
		// We only support one datasource for now.
		ds := spec.Datasources[0]
		if ds.Type == scyllav1alpha1.GrafanaDatasourceTypePrometheus {
			return &ds
		}
	}
	return nil
}
