package tests

import (
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
)

type TestFrameworkOptions struct {
	ArtifactsDir                 string
	DeleteTestingNSPolicyUntyped string
	DeleteTestingNSPolicy        framework.DeleteTestingNSPolicyType
}

func NewTestFrameworkOptions() TestFrameworkOptions {
	return TestFrameworkOptions{
		ArtifactsDir:                 "",
		DeleteTestingNSPolicyUntyped: string(framework.DeleteTestingNSPolicyAlways),
	}
}

func (o *TestFrameworkOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&o.ArtifactsDir, "artifacts-dir", "", o.ArtifactsDir, "A directory for storing test artifacts. No data is collected until set.")
	cmd.PersistentFlags().StringVarP(&o.DeleteTestingNSPolicyUntyped, "delete-namespace-policy", "", o.DeleteTestingNSPolicyUntyped, fmt.Sprintf("Namespace deletion policy. Allowed values are [%s]", strings.Join(
		[]string{
			string(framework.DeleteTestingNSPolicyAlways),
			string(framework.DeleteTestingNSPolicyNever),
			string(framework.DeleteTestingNSPolicyOnSuccess),
		},
		",",
	)))
}

func (o *TestFrameworkOptions) Validate() error {
	var errors []error

	switch p := framework.DeleteTestingNSPolicyType(o.DeleteTestingNSPolicyUntyped); p {
	case framework.DeleteTestingNSPolicyAlways,
		framework.DeleteTestingNSPolicyOnSuccess,
		framework.DeleteTestingNSPolicyNever:
	default:
		errors = append(errors, fmt.Errorf("invalid DeleteTestingNSPolicy: %q", p))
	}

	return apierrors.NewAggregate(errors)
}

func (o *TestFrameworkOptions) Complete() error {
	o.DeleteTestingNSPolicy = framework.DeleteTestingNSPolicyType(o.DeleteTestingNSPolicyUntyped)

	// Trim spaces so we can reason later if the dir is set or not
	o.ArtifactsDir = strings.TrimSpace(o.ArtifactsDir)

	return nil
}
