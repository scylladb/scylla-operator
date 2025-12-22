package configassests

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	"sigs.k8s.io/yaml"
)

var (
	//go:embed "config.yaml"
	projectConfigString string
	Project             = helpers.Must(func() (*ProjectConfig, error) {
		c := &ProjectConfig{}
		err := yaml.UnmarshalStrict([]byte(projectConfigString), c)
		return c, err
	}())
)

type OperatorConfig struct {
	ScyllaDBVersion string `json:"scyllaDBVersion"`
	// scyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride sets enterprise version
	// that requires consistent_cluster_management workaround for restore.
	// In the future, enterprise versions should be run as a different config instance in its own run.
	ScyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride string `json:"scyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride"`
	ScyllaDBUtilsImage                                                  string `json:"scyllaDBUtilsImage"`
	ScyllaDBManagerVersion                                              string `json:"scyllaDBManagerVersion"`
	ScyllaDBManagerAgentVersion                                         string `json:"scyllaDBManagerAgentVersion"`
	BashToolsImage                                                      string `json:"bashToolsImage"`
	GrafanaImage                                                        string `json:"grafanaImage"`
	GrafanaDefaultPlatformDashboard                                     string `json:"grafanaDefaultPlatformDashboard"`
	PrometheusVersion                                                   string `json:"prometheusVersion"`
}

type ScyllaDBTestVersions struct {
	UpdateFrom  string `json:"updateFrom"`
	UpgradeFrom string `json:"upgradeFrom"`
}

type OperatorTestsConfig struct {
	ScyllaDBVersions         ScyllaDBTestVersions `json:"scyllaDBVersions"`
	NodeSetupImage           string               `json:"nodeSetupImage"`
	EnvTestKubernetesVersion string               `json:"envTestKubernetesVersion"`
}

type ProjectConfig struct {
	Operator      OperatorConfig      `json:"operator"`
	OperatorTests OperatorTestsConfig `json:"operatorTests"`
}
