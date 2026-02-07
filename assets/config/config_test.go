package configassests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	prometheusoperator "github.com/scylladb/scylla-operator/pkg/thirdparty/github.com/prometheus-operator/prometheus-operator/pkg/operator"
	"github.com/scylladb/scylla-operator/pkg/util/images"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateRequired(v string) error {
	if len(strings.TrimSpace(v)) == 0 {
		return fmt.Errorf("value %q is empty", v)
	}

	return nil
}

// validateSemanticVersion checks if the provided string is a valid semantic version.
func validateSemanticVersion(v string) error {
	return validation.ValidateSemanticVersion(v, &field.Path{}).ToAggregate()
}

// validateMultiPlatformImage checks if the provided image is a multi-platform image.
func validateMultiPlatformImage(ctx context.Context) func(image string) error {
	return func(image string) error {
		return images.IsImageMultiPlatform(ctx, image)
	}
}

// validateMultiPlatformVersionWithRepo checks if the provided version is a valid multi-platform image for a given repository.
func validateMultiPlatformVersionWithRepo(ctx context.Context, repo string) func(tag string) error {
	return func(tag string) error {
		image := fmt.Sprintf("%s:%s", repo, tag)
		return validateMultiPlatformImage(ctx)(image)
	}
}

var (
	dashboardPathRegexFmt = `^[^ /]+/[^ /]+$`
	dashboardPathRegex    = regexp.MustCompile(dashboardPathRegexFmt)
)

func validateDashboardPath(p string) error {
	if dashboardPathRegex.MatchString(p) {
		return nil
	}

	return fmt.Errorf("path %q is invalid: doesn't match regex %q", p, dashboardPathRegexFmt)
}

func TestManagerAndAgentVersionsMatch(t *testing.T) {
	t.Parallel()

	// Extract tag from version string (ignore digest after @)
	extractTag := func(version string) string {
		parts := strings.Split(version, "@")
		return parts[0]
	}

	managerTag := extractTag(Project.Operator.ScyllaDBManagerVersion)
	agentTag := extractTag(Project.Operator.ScyllaDBManagerAgentVersion)

	if managerTag != agentTag {
		t.Errorf("scyllaDBManagerVersion tag %q does not match scyllaDBManagerAgentVersion tag %q", managerTag, agentTag)
	}
}

func TestProjectConfig(t *testing.T) {
	t.Parallel()

	composeValidators := func(validators ...func(string) error) func(string) error {
		return func(value string) error {
			var errs []error
			for _, validate := range validators {
				if err := validate(value); err != nil {
					errs = append(errs, err)
				}
			}
			return errors.Join(errs...)
		}
	}

	ctx := t.Context()

	testCases := []struct {
		name        string
		configField string
		testFn      func(string) error
	}{
		{
			name:        "scyllaDBVersion",
			configField: Project.Operator.ScyllaDBVersion,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformVersionWithRepo(ctx, ScyllaDBImageRepository),
			),
		},
		{
			name:        "scyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride",
			configField: Project.Operator.ScyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformVersionWithRepo(ctx, ScyllaDBEnterpriseImageRepository),
			),
		},
		{
			name:        "scyllaDBUtilsImage",
			configField: Project.Operator.ScyllaDBUtilsImage,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformImage(ctx),
			),
		},
		{
			name:        "scyllaDBManagerVersion",
			configField: Project.Operator.ScyllaDBManagerVersion,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformVersionWithRepo(ctx, ScyllaDBManagerImageRepository),
			),
		},
		{
			name:        "scyllaDBManagerAgentVersion",
			configField: Project.Operator.ScyllaDBManagerAgentVersion,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformVersionWithRepo(ctx, ScyllaDBManagerAgentImageRepository),
			),
		},
		{
			name:        "bashToolsImage",
			configField: Project.Operator.BashToolsImage,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformImage(ctx),
			),
		},
		{
			name:        "grafanaImage",
			configField: Project.Operator.GrafanaImage,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformImage(ctx),
			),
		},
		{
			name:        "grafanaDefaultPlatformDashboard",
			configField: Project.Operator.GrafanaDefaultPlatformDashboard,
			testFn: composeValidators(
				validateRequired,
				validateDashboardPath,
			),
		},
		{
			name:        "prometheusVersion",
			configField: Project.Operator.PrometheusVersion,
			testFn: composeValidators(
				validateRequired,
				validateSemanticVersion,
			),
		},
		{
			name:        "scyllaDBVersions.UpdateFrom",
			configField: Project.OperatorTests.ScyllaDBVersions.UpdateFrom,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformVersionWithRepo(ctx, ScyllaDBImageRepository),
			),
		},
		{
			name:        "scyllaDBVersions.UpgradeFrom",
			configField: Project.OperatorTests.ScyllaDBVersions.UpgradeFrom,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformVersionWithRepo(ctx, ScyllaDBImageRepository),
			),
		},
		{
			name:        "nodeSetupImage",
			configField: Project.OperatorTests.NodeSetupImage,
			testFn: composeValidators(
				validateRequired,
				validateMultiPlatformImage(ctx),
			),
		},
		{
			name:        "envTestKubernetesVersion",
			configField: Project.OperatorTests.EnvTestKubernetesVersion,
			testFn: composeValidators(
				validateRequired,
				validateSemanticVersion,
			),
		},
		{
			name:        "thirdParty.prometheusOperator.version",
			configField: Project.ThirdParty.PrometheusOperatorConfig.Version,
			testFn: composeValidators(
				validateRequired,
				validateSemanticVersion,
			),
		},
		{
			name:        "thirdParty.prometheusOperator.namespace",
			configField: Project.ThirdParty.PrometheusOperatorConfig.Namespace,
			testFn:      validateRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.testFn(tc.configField); err != nil {
				t.Errorf("validation failed for %s: %v", tc.name, err)
			}
		})
	}
}

func TestScyllaDBMonitoringAndGrafanaDefaultPlatformDashboardCompatibility(t *testing.T) {
	t.Parallel()

	// Verify that grafanaDefaultPlatformDashboard matches scyllaDBVersion (up to minor).
	t.Run("grafanaDefaultPlatformDashboard matches scyllaDBVersion", func(t *testing.T) {
		scyllaDBVersion, err := semver.Parse(Project.Operator.ScyllaDBVersion)
		if err != nil {
			t.Fatalf("failed to parse scyllaDBVersion: %v", err)
		}

		expectedMajorMinor := fmt.Sprintf("%d.%d", scyllaDBVersion.Major, scyllaDBVersion.Minor)
		expectedDefaultPlatformDashboard := fmt.Sprintf("scylladb-%s/scylla-overview.%s.json", expectedMajorMinor, expectedMajorMinor)

		if Project.Operator.GrafanaDefaultPlatformDashboard != expectedDefaultPlatformDashboard {
			t.Errorf("grafanaDefaultPlatformDashboard %q does not match expected %q based on scyllaDBVersion %q", Project.Operator.GrafanaDefaultPlatformDashboard, expectedDefaultPlatformDashboard, Project.Operator.ScyllaDBVersion)
		}
	})

	t.Run("grafanaDefaultPlatformDashboard exists", func(t *testing.T) {
		dashboardPath := "../../assets/monitoring/grafana/v1alpha1/dashboards/platform/" + Project.Operator.GrafanaDefaultPlatformDashboard
		if _, err := os.Stat(dashboardPath); os.IsNotExist(err) {
			t.Errorf("grafanaDefaultPlatformDashboard %q does not exist at path %q", Project.Operator.GrafanaDefaultPlatformDashboard, dashboardPath)
		}
	})
}

func TestPrometheusOperatorCompatibility(t *testing.T) {
	t.Parallel()

	t.Run("prometheus operator version supports prometheus version", func(t *testing.T) {
		t.Parallel()

		prometheusVersion := Project.Operator.PrometheusVersion
		if !slices.Contains(prometheusoperator.PrometheusCompatibilityMatrix, prometheusVersion) {
			t.Errorf("prometheus operator version %q compatibility matrix does not contain prometheus version %q", Project.ThirdParty.PrometheusOperatorConfig.Version, prometheusVersion)
		}
	})
}
