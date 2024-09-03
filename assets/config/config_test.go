package configassests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateRequired(v string) error {
	if len(strings.TrimSpace(v)) == 0 {
		return fmt.Errorf("value %q is empty", v)
	}

	return nil
}

func validateImage(image string) error {
	return validation.ValidateImageRef(image, &field.Path{}).ToAggregate()
}

func validateSemanticVersion(v string) error {
	return validation.ValidateSemanticVersion(v, &field.Path{}).ToAggregate()
}

func TestProjectConfig(t *testing.T) {
	var err error

	err = validateRequired(Project.Operator.ScyllaDBVersion)
	if err != nil {
		t.Error(err)
	}

	err = validateRequired(Project.Operator.ScyllaDBEnterpriseVersionNeedingConsistentClusterManagementOverride)
	if err != nil {
		t.Error(err)
	}

	err = validateImage(Project.Operator.ScyllaDBUtilsImage)
	if err != nil {
		t.Error(err)
	}

	err = validateRequired(Project.Operator.ScyllaDBManagerVersion)
	if err != nil {
		t.Error(err)
	}

	err = validateRequired(Project.Operator.ScyllaDBManagerAgentVersion)
	if err != nil {
		t.Error(err)
	}

	err = validateImage(Project.Operator.BashToolsImage)
	if err != nil {
		t.Error(err)
	}

	err = validateImage(Project.Operator.GrafanaImage)
	if err != nil {
		t.Error(err)
	}

	err = validateSemanticVersion(Project.Operator.PrometheusVersion)
	if err != nil {
		t.Error(err)
	}

	err = validateRequired(Project.OperatorTests.ScyllaDBVersions.UpdateFrom)
	if err != nil {
		t.Error(err)
	}

	err = validateRequired(Project.OperatorTests.ScyllaDBVersions.UpgradeFrom)
	if err != nil {
		t.Error(err)
	}

	err = validateImage(Project.OperatorTests.NodeSetupImage)
	if err != nil {
		t.Error(err)
	}
}
