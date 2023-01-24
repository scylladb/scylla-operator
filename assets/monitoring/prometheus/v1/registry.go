package v1

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "prometheus.yaml"
	prometheusTemplateString string
	PrometheusTemplate       = ParseObjectTemplateOrDie[*monitoringv1.Prometheus]("prometheus", prometheusTemplateString)

	//go:embed "serviceaccount.yaml"
	prometheusSATemplateString string
	PrometheusSATemplate       = ParseObjectTemplateOrDie[*corev1.ServiceAccount]("prometheus-sa", prometheusSATemplateString)

	//go:embed "rolebinding.yaml"
	prometheusRoleBindingTemplateString string
	PrometheusRoleBindingTemplate       = ParseObjectTemplateOrDie[*rbacv1.RoleBinding]("prometheus-rolebinding", prometheusRoleBindingTemplateString)

	//go:embed "service.yaml"
	prometheusServiceTemplateString string
	PrometheusServiceTemplate       = ParseObjectTemplateOrDie[*corev1.Service]("prometheus-service", prometheusServiceTemplateString)

	//go:embed "scylladb.servicemonitor.yaml"
	scyllaDBServiceMonitorTemplateString string
	ScyllaDBServiceMonitorTemplate       = ParseObjectTemplateOrDie[*monitoringv1.ServiceMonitor]("scylladb-servicemonitor", scyllaDBServiceMonitorTemplateString)

	//go:embed "recording.prometheusrule.yaml"
	recordingPrometheusRuleTemplateString string
	RecordingPrometheusRuleTemplate       = ParseObjectTemplateOrDie[*monitoringv1.PrometheusRule]("recording-prometheus-rule", recordingPrometheusRuleTemplateString)

	//go:embed "alerts.prometheusrule.yaml"
	alertsPrometheusRuleTemplateString string
	AlertsPrometheusRuleTemplate       = ParseObjectTemplateOrDie[*monitoringv1.PrometheusRule]("alerts-prometheus-rule", alertsPrometheusRuleTemplateString)

	//go:embed "ingress.yaml"
	prometheusIngressTemplateString string
	PrometheusIngressTemplate       = ParseObjectTemplateOrDie[*networkingv1.Ingress]("prometheus-ingress", prometheusIngressTemplateString)
)
