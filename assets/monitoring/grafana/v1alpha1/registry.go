package v1alpha1

import (
	"embed"
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/util/lazy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) *assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "deployment.yaml"
	grafanaDeploymentTemplateString string
	GrafanaDeploymentTemplate       = lazy.New(func() *assets.ObjectTemplate[*appsv1.Deployment] {
		return ParseObjectTemplateOrDie[*appsv1.Deployment]("grafana-deployment", grafanaDeploymentTemplateString)
	})

	//go:embed "serviceaccount.yaml"
	grafanaSATemplateString string
	GrafanaSATemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ServiceAccount] {
		return ParseObjectTemplateOrDie[*corev1.ServiceAccount]("grafana-sa", grafanaSATemplateString)
	})

	//go:embed "rolebinding.yaml"
	grafanaRoleBindingTemplateString string
	GrafanaRoleBindingTemplate       = lazy.New(func() *assets.ObjectTemplate[*rbacv1.RoleBinding] {
		return ParseObjectTemplateOrDie[*rbacv1.RoleBinding]("grafana-rolebinding", grafanaRoleBindingTemplateString)
	})

	//go:embed "configs.cm.yaml"
	grafanaConfigsTemplateString string
	GrafanaConfigsTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ConfigMap] {
		return ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-configs-cm", grafanaConfigsTemplateString)
	})

	//go:embed "admin-credentials.secret.yaml"
	grafanaAdminCredentialsSecretTemplateString string
	GrafanaAdminCredentialsSecretTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.Secret] {
		return ParseObjectTemplateOrDie[*corev1.Secret]("grafana-access-credentials-secret", grafanaAdminCredentialsSecretTemplateString)
	})

	//go:embed "provisioning.cm.yaml"
	grafanaProvisioningConfigMapTemplateString string
	GrafanaProvisioningConfigMapTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ConfigMap] {
		return ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-provisioning-cm", grafanaProvisioningConfigMapTemplateString)
	})

	//go:embed "dashboards.cm.yaml"
	grafanaDashboardsConfigMapTemplateString string
	GrafanaDashboardsConfigMapTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ConfigMap] {
		return ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-dashboards-cm", grafanaDashboardsConfigMapTemplateString)
	})

	//go:embed "dashboards/platform/*/*.json"
	grafanaDashboardsPlatformFS embed.FS
	GrafanaDashboardsPlatform   = lazy.New(func() GrafanaDashboardsFoldersMap {
		return helpers.Must(NewGrafanaDashboardsFromFS(grafanaDashboardsPlatformFS, "dashboards/platform"))
	})

	//go:embed "dashboards/saas/*/*.json"
	grafanaDashboardsSAASFS embed.FS
	GrafanaDashboardsSAAS   = lazy.New(func() GrafanaDashboardsFoldersMap {
		return helpers.Must(NewGrafanaDashboardsFromFS(grafanaDashboardsSAASFS, "dashboards/saas"))
	})

	//go:embed "service.yaml"
	grafanaServiceTemplateString string
	GrafanaServiceTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.Service] {
		return ParseObjectTemplateOrDie[*corev1.Service]("grafana-service", grafanaServiceTemplateString)
	})

	//go:embed "ingress.yaml"
	grafanaIngressTemplateString string
	GrafanaIngressTemplate       = lazy.New(func() *assets.ObjectTemplate[*networkingv1.Ingress] {
		return ParseObjectTemplateOrDie[*networkingv1.Ingress]("grafana-ingress", grafanaIngressTemplateString)
	})
)
