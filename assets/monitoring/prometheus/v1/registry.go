package v1

import (
	"embed"
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/util/lazy"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) *assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "prometheus.yaml"
	prometheusTemplateString string
	PrometheusTemplate       = lazy.New(func() *assets.ObjectTemplate[*monitoringv1.Prometheus] {
		return ParseObjectTemplateOrDie[*monitoringv1.Prometheus]("prometheus", prometheusTemplateString)
	})

	//go:embed "serviceaccount.yaml"
	prometheusSATemplateString string
	PrometheusSATemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ServiceAccount] {
		return ParseObjectTemplateOrDie[*corev1.ServiceAccount]("prometheus-sa", prometheusSATemplateString)
	})

	//go:embed "rolebinding.yaml"
	prometheusRoleBindingTemplateString string
	PrometheusRoleBindingTemplate       = lazy.New(func() *assets.ObjectTemplate[*rbacv1.RoleBinding] {
		return ParseObjectTemplateOrDie[*rbacv1.RoleBinding]("prometheus-rolebinding", prometheusRoleBindingTemplateString)
	})

	//go:embed "service.yaml"
	prometheusServiceTemplateString string
	PrometheusServiceTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.Service] {
		return ParseObjectTemplateOrDie[*corev1.Service]("prometheus-service", prometheusServiceTemplateString)
	})

	//go:embed "scylladb.servicemonitor.yaml"
	scyllaDBServiceMonitorTemplateString string
	ScyllaDBServiceMonitorTemplate       = lazy.New(func() *assets.ObjectTemplate[*monitoringv1.ServiceMonitor] {
		return ParseObjectTemplateOrDie[*monitoringv1.ServiceMonitor]("scylladb-servicemonitor", scyllaDBServiceMonitorTemplateString)
	})

	//go:embed "rules/**"
	prometheusRulesFS embed.FS
	PrometheusRules   = lazy.New(func() PrometheusRulesMap {
		return helpers.Must(NewPrometheusRulesFromFS(prometheusRulesFS))
	})

	//go:embed "latency.prometheusrule.yaml"
	latencyPrometheusRuleTemplateString string
	LatencyPrometheusRuleTemplate       = lazy.New(func() *assets.ObjectTemplate[*monitoringv1.PrometheusRule] {
		return ParseObjectTemplateOrDie[*monitoringv1.PrometheusRule]("latency-prometheus-rule", latencyPrometheusRuleTemplateString)
	})

	//go:embed "alerts.prometheusrule.yaml"
	alertsPrometheusRuleTemplateString string
	AlertsPrometheusRuleTemplate       = lazy.New(func() *assets.ObjectTemplate[*monitoringv1.PrometheusRule] {
		return ParseObjectTemplateOrDie[*monitoringv1.PrometheusRule]("alerts-prometheus-rule", alertsPrometheusRuleTemplateString)
	})

	//go:embed "table.prometheusrule.yaml"
	tablePrometheusRuleTemplateString string
	TablePrometheusRuleTemplate       = lazy.New(func() *assets.ObjectTemplate[*monitoringv1.PrometheusRule] {
		return ParseObjectTemplateOrDie[*monitoringv1.PrometheusRule]("table-prometheus-rule", tablePrometheusRuleTemplateString)
	})

	//go:embed "ingress.yaml"
	prometheusIngressTemplateString string
	PrometheusIngressTemplate       = lazy.New(func() *assets.ObjectTemplate[*networkingv1.Ingress] {
		return ParseObjectTemplateOrDie[*networkingv1.Ingress]("prometheus-ingress", prometheusIngressTemplateString)
	})
)
