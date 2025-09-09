package scylladbmonitoring

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
)

const (
	scyllaDBMonitoringBySecretIndexName    = "secret"
	scyllaDBMonitoringByConfigMapIndexName = "configmap"
)

// indexScyllaDBMonitoringBySecret indexes ScyllaDBMonitoring resources by the names of Secrets it references.
func indexScyllaDBMonitoringBySecret(obj interface{}) ([]string, error) {
	sdm, ok := obj.(*scyllav1alpha1.ScyllaDBMonitoring)
	if !ok {
		return nil, fmt.Errorf("expected *scyllav1alpha1.ScyllaDBMonitoring, got %T", obj)
	}

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

	return secretNames, nil
}

// indexScyllaDBMonitoringByConfigMap indexes ScyllaDBMonitoring resources by the names of ConfigMaps it references.
func indexScyllaDBMonitoringByConfigMap(obj interface{}) ([]string, error) {
	sdm, ok := obj.(*scyllav1alpha1.ScyllaDBMonitoring)
	if !ok {
		return nil, fmt.Errorf("expected *scyllav1alpha1.ScyllaDBMonitoring, got %T", obj)
	}

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

	return configMapNames, nil
}
