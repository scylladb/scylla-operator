package v1alpha1

import (
	"embed"
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "deployment.yaml"
	grafanaDeploymentTemplateString string
	GrafanaDeploymentTemplate       = ParseObjectTemplateOrDie[*appsv1.Deployment]("grafana-deployment", grafanaDeploymentTemplateString)

	//go:embed "serviceaccount.yaml"
	grafanaSATemplateString string
	GrafanaSATemplate       = ParseObjectTemplateOrDie[*corev1.ServiceAccount]("grafana-sa", grafanaSATemplateString)

	//go:embed "configs.cm.yaml"
	grafanaConfigsTemplateString string
	GrafanaConfigsTemplate       = ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-configs-cm", grafanaConfigsTemplateString)

	//go:embed "admin-credentials.secret.yaml"
	grafanaAdminCredentialsSecretTemplateString string
	GrafanaAdminCredentialsSecretTemplate       = ParseObjectTemplateOrDie[*corev1.Secret]("grafana-access-credentials-secret", grafanaAdminCredentialsSecretTemplateString)

	//go:embed "provisioning.cm.yaml"
	grafanaProvisioningConfigMapTemplateString string
	GrafanaProvisioningConfigMapTemplate       = ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-provisioning-cm", grafanaProvisioningConfigMapTemplateString)

	//go:embed "dashboards.cm.yaml"
	grafanaDashboardsConfigMapTemplateString string
	GrafanaDashboardsConfigMapTemplate       = ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-dashboards-cm", grafanaDashboardsConfigMapTemplateString)

	//go:embed "dashboards/platform/*/*.json"
	grafanaDashboardsPlatformFS embed.FS
	GrafanaDashboardsPlatform   = helpers.Must(NewGrafanaDashboardsFromFS(grafanaDashboardsPlatformFS, "dashboards/platform"))

	//go:embed "dashboards/saas/*/*.json"
	grafanaDashboardsSAASFS embed.FS
	GrafanaDashboardsSAAS   = helpers.Must(NewGrafanaDashboardsFromFS(grafanaDashboardsSAASFS, "dashboards/saas"))

	//go:embed "service.yaml"
	grafanaServiceTemplateString string
	GrafanaServiceTemplate       = ParseObjectTemplateOrDie[*corev1.Service]("grafana-service", grafanaServiceTemplateString)

	//go:embed "ingress.yaml"
	grafanaIngressTemplateString string
	GrafanaIngressTemplate       = ParseObjectTemplateOrDie[*networkingv1.Ingress]("grafana-ingress", grafanaIngressTemplateString)
)
