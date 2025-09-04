package validation

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateScyllaDBMonitoring(sm *scyllav1alpha1.ScyllaDBMonitoring) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateScyllaDBMonitoringSpec(&sm.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBMonitoringUpdate(new, old *scyllav1alpha1.ScyllaDBMonitoring) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBMonitoring(new)...)
	allErrs = append(allErrs, validateScyllaDBMonitoringSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)

	return allErrs
}

func GetWarningsOnScyllaDBMonitoringCreate(_ *scyllav1alpha1.ScyllaDBMonitoring) []string {
	return nil
}

func GetWarningsOnScyllaDBMonitoringUpdate(_, _ *scyllav1alpha1.ScyllaDBMonitoring) []string {
	return nil
}

func validateScyllaDBMonitoringSpecUpdate(new, old *scyllav1alpha1.ScyllaDBMonitoringSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if new.Components != nil && old.Components != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringComponentsUpdate(new.Components, old.Components, fldPath.Child("components"))...)
	}

	return allErrs
}

func validateScyllaDBMonitoringComponentsUpdate(new, old *scyllav1alpha1.Components, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if new.Prometheus != nil && old.Prometheus != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringSpecComponentsPrometheusUpdate(new.Prometheus, old.Prometheus, fldPath.Child("prometheus"))...)
	}

	return allErrs
}

func validateScyllaDBMonitoringSpecComponentsPrometheusUpdate(new, old *scyllav1alpha1.PrometheusSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if new.Mode != old.Mode {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mode"), old.Mode, "is immutable and cannot be changed"))
	}

	return allErrs
}

func validateScyllaDBMonitoringSpec(sm *scyllav1alpha1.ScyllaDBMonitoringSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if sm.Components != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringComponents(sm.Components, fldPath.Child("components"))...)
	}

	return allErrs
}

func validateScyllaDBMonitoringComponents(components *scyllav1alpha1.Components, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if components.Prometheus != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringSpecComponentsPrometheus(components.Prometheus, fldPath.Child("prometheus"))...)
	}

	if components.Grafana != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringSpecComponentsGrafana(components.Grafana, fldPath.Child("grafana"))...)
	}

	allErrs = append(allErrs, validateScyllaDBMonitoringComponentsInterdependencies(components, fldPath)...)

	return allErrs
}

var allowedPrometheusModes = []string{
	string(scyllav1alpha1.PrometheusModeManaged),
	string(scyllav1alpha1.PrometheusModeExternal),
}

func validateScyllaDBMonitoringSpecComponentsPrometheus(ps *scyllav1alpha1.PrometheusSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !oslices.ContainsItem(allowedPrometheusModes, string(ps.Mode)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("mode"), string(ps.Mode), allowedPrometheusModes))
	}

	return allErrs
}

func validateScyllaDBMonitoringComponentsInterdependencies(components *scyllav1alpha1.Components, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if components.Prometheus != nil && components.Prometheus.Mode == scyllav1alpha1.PrometheusModeExternal {
		if components.Grafana == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("grafana"), "must be specified when Prometheus is in External mode"))
		} else {
			datasources := components.Grafana.Datasources
			if len(datasources) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("grafana").Child("datasources"), "exactly one datasource must be specified when Prometheus is in External mode"))
			} else if len(datasources) > 1 {
				allErrs = append(allErrs, field.TooMany(fldPath.Child("grafana").Child("datasources"), len(datasources), 1))
			}
		}
	}

	if components.Prometheus == nil || components.Prometheus.Mode == scyllav1alpha1.PrometheusModeManaged {
		if components.Grafana != nil && len(components.Grafana.Datasources) > 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("grafana").Child("datasources"), "must not be specified when Prometheus is in Managed mode"))
		}
	}

	return allErrs
}

func validateScyllaDBMonitoringSpecComponentsGrafana(gs *scyllav1alpha1.GrafanaSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, ds := range gs.Datasources {
		allErrs = append(allErrs, validateScyllaDBMonitoringGrafanaDatasource(ds, fldPath.Child("datasources").Index(i))...)
	}

	return allErrs
}

var allowedDatasourceTypes = []string{
	string(scyllav1alpha1.GrafanaDatasourceTypePrometheus),
}

var allowedDatasourceNames = []string{
	"prometheus",
}

func validateScyllaDBMonitoringGrafanaDatasource(ds scyllav1alpha1.GrafanaDatasourceSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !oslices.ContainsItem(allowedDatasourceTypes, string(ds.Type)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), string(ds.Type), allowedDatasourceTypes))
	}
	if !oslices.ContainsItem(allowedDatasourceNames, ds.Name) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("name"), ds.Name, allowedDatasourceNames))
	}
	if ds.URL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), "must be specified"))
	}

	allErrs = append(allErrs, validateScyllaDBMonitoringGrafanaDatasourceOptions(&ds, fldPath)...)

	return allErrs
}

func validateScyllaDBMonitoringGrafanaDatasourceOptions(ds *scyllav1alpha1.GrafanaDatasourceSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if ds.Type == scyllav1alpha1.GrafanaDatasourceTypePrometheus {
		allErrs = append(allErrs, validateScyllaDBMonitoringGrafanaPrometheusDatasource(ds, fldPath)...)
	}
	if ds.Type != scyllav1alpha1.GrafanaDatasourceTypePrometheus && ds.PrometheusOptions != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("prometheusOptions"), "must not be specified when datasource type is not Prometheus"))
	}

	return allErrs
}

var supportedPrometheusAuthTypes = []string{
	string(scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeNoAuthentication),
	string(scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken),
}

func validateScyllaDBMonitoringGrafanaPrometheusDatasource(ds *scyllav1alpha1.GrafanaDatasourceSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	opts := ds.PrometheusOptions
	if opts == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("prometheusOptions"), "must be specified for Prometheus datasource"))
		return allErrs
	}

	if opts.Auth != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringPrometheusAuth(opts.Auth, fldPath.Child("prometheusOptions").Child("auth"))...)
	}

	if opts.TLS != nil {
		allErrs = append(allErrs, validateScyllaDBMonitoringGrafanaPrometheusDatasourceTLS(opts.TLS, fldPath.Child("prometheusOptions").Child("tls"))...)
	}

	return allErrs
}

func validateScyllaDBMonitoringPrometheusAuth(auth *scyllav1alpha1.GrafanaPrometheusDatasourceAuthSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !oslices.ContainsItem(supportedPrometheusAuthTypes, string(auth.Type)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), string(auth.Type), supportedPrometheusAuthTypes))
	}

	if auth.Type == scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken {
		allErrs = append(allErrs, validateScyllaDBMonitoringPrometheusAuthBearerToken(auth.BearerTokenOptions, fldPath.Child("bearerTokenOptions"))...)
	}

	return allErrs
}

func validateScyllaDBMonitoringPrometheusAuthBearerToken(opts *scyllav1alpha1.GrafanaPrometheusDatasourceBearerTokenAuthOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if opts.SecretRef == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretRef"), "must be specified for BearerToken auth"))
	} else {
		allErrs = append(allErrs, validateLocalObjectKeySelector(opts.SecretRef, fldPath.Child("secretRef"))...)
	}

	return allErrs
}

func validateLocalObjectKeySelector(ref *scyllav1alpha1.LocalObjectKeySelector, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if ref.Key == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), "must be specified"))
	}

	if ref.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must be specified"))
	}

	return allErrs
}

func validateScyllaDBMonitoringGrafanaPrometheusDatasourceTLS(tls *scyllav1alpha1.GrafanaDatasourceTLSSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if tls.ClientTLSKeyPairSecretRef != nil {
		allErrs = append(allErrs, validateLocalObjectReference(tls.ClientTLSKeyPairSecretRef, fldPath.Child("clientTLSKeyPairSecretRef"))...)
	}

	if tls.CACertConfigMapRef != nil {
		allErrs = append(allErrs, validateLocalObjectKeySelector(tls.CACertConfigMapRef, fldPath.Child("caCertConfigMapRef"))...)
	}

	return allErrs
}

func validateLocalObjectReference(ref *scyllav1alpha1.LocalObjectReference, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if ref.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must be specified"))
	}

	return allErrs
}
