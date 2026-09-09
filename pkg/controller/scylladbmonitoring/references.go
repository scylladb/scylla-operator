package scylladbmonitoring

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
)

// getScyllaDBMonitoringGrafanaSecretReferences returns the names of the Secrets the ScyllaDBMonitoring's Grafana datasources reference.
func getScyllaDBMonitoringGrafanaSecretReferences(sdm *scyllav1alpha1.ScyllaDBMonitoring) []string {
	var secretNames []string

	if sdm.Spec.Components != nil && sdm.Spec.Components.Grafana != nil {
		for _, ds := range sdm.Spec.Components.Grafana.Datasources {
			if ds.PrometheusOptions != nil {
				if ds.PrometheusOptions.Auth != nil && ds.PrometheusOptions.Auth.BearerTokenOptions != nil && ds.PrometheusOptions.Auth.BearerTokenOptions.SecretRef != nil {
					secretNames = append(secretNames, ds.PrometheusOptions.Auth.BearerTokenOptions.SecretRef.Name)
				}
				if ds.PrometheusOptions.TLS != nil {
					if ds.PrometheusOptions.TLS.ClientTLSKeyPairSecretRef != nil {
						secretNames = append(secretNames, ds.PrometheusOptions.TLS.ClientTLSKeyPairSecretRef.Name)
					}
				}
			}
		}
	}

	return secretNames
}

// getScyllaDBMonitoringGrafanaConfigMapReferences returns the names of the ConfigMaps the ScyllaDBMonitoring's Grafana datasources reference.
func getScyllaDBMonitoringGrafanaConfigMapReferences(sdm *scyllav1alpha1.ScyllaDBMonitoring) []string {
	var configMapNames []string

	if sdm.Spec.Components != nil && sdm.Spec.Components.Grafana != nil {
		for _, ds := range sdm.Spec.Components.Grafana.Datasources {
			if ds.PrometheusOptions != nil {
				if ds.PrometheusOptions.TLS != nil {
					if ds.PrometheusOptions.TLS.CACertConfigMapRef != nil {
						configMapNames = append(configMapNames, ds.PrometheusOptions.TLS.CACertConfigMapRef.Name)
					}
				}
			}
		}
	}

	return configMapNames
}
