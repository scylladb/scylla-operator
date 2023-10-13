package v1alpha1

import (
	"bytes"
	"compress/gzip"
	"embed"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"path/filepath"

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

	//go:embed "dashboards/platform/*.json"
	grafanaDashboardsPlatformFS embed.FS
	GrafanaDashboardsPlatform   = helpers.Must(gzipMapData(helpers.Must(parseDashboardsFromFS(grafanaDashboardsPlatformFS, "dashboards/platform"))))

	//go:embed "dashboards/saas/*.json"
	grafanaDashboardsSAASFS embed.FS
	GrafanaDashboardsSAAS   = helpers.Must(gzipMapData(helpers.Must(parseDashboardsFromFS(grafanaDashboardsSAASFS, "dashboards/saas"))))

	//go:embed "service.yaml"
	grafanaServiceTemplateString string
	GrafanaServiceTemplate       = ParseObjectTemplateOrDie[*corev1.Service]("grafana-service", grafanaServiceTemplateString)

	//go:embed "ingress.yaml"
	grafanaIngressTemplateString string
	GrafanaIngressTemplate       = ParseObjectTemplateOrDie[*networkingv1.Ingress]("grafana-ingress", grafanaIngressTemplateString)
)

func parseDashboardsFromFS(files embed.FS, root string) (map[string]string, error) {
	res := map[string]string{}

	err := fs.WalkDir(files, root, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if p == root || de.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(files, p)
		if err != nil {
			return fmt.Errorf("can't read file %q: %w", p, err)
		}
		relPath, err := filepath.Rel(root, p)
		if err != nil {
			return fmt.Errorf("can't compute relative path to file %q: %w", p, err)
		}
		res[relPath] = string(data)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("can't walk fs at root %q: %w", root, err)
	}

	return res, nil
}

func gzipMapData(uncompressedMap map[string]string) (map[string]string, error) {
	res := make(map[string]string, len(uncompressedMap))
	for k, v := range uncompressedMap {
		var buf bytes.Buffer
		b64Writer := base64.NewEncoder(base64.StdEncoding, &buf)

		zw, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
		if err != nil {
			return nil, fmt.Errorf("can't create gzip writer: %w", err)
		}

		_, err = zw.Write([]byte(v))
		if err != nil {
			return nil, fmt.Errorf("can't write value of key %q into gzip writer: %w", k, err)
		}
		err = zw.Close()
		if err != nil {
			return nil, fmt.Errorf("can't close gzip writer for key %q: %w", k, err)
		}

		err = b64Writer.Close()
		if err != nil {
			return nil, fmt.Errorf("can't close base64 writer for key %q: %w", k, err)
		}

		res[fmt.Sprintf("%s.gz.base64", k)] = buf.String()
	}

	return res, nil
}
