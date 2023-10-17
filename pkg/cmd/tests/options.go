package tests

import (
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
)

type IngressControllerOptions struct {
	Address           string
	IngressClassName  string
	CustomAnnotations map[string]string
}

type ScyllaDBClusterExposureType string

var (
	ScyllaDBClusterExposureTypePodIP     ScyllaDBClusterExposureType = "PodIP"
	ScyllaDBClusterExposureTypeClusterIP ScyllaDBClusterExposureType = "ClusterIP"
)

type TestFrameworkOptions struct {
	ArtifactsDir                   string
	DeleteTestingNSPolicyUntyped   string
	DeleteTestingNSPolicy          framework.DeleteTestingNSPolicyType
	IngressController              *IngressControllerOptions
	ScyllaDBClusterExposureUntyped string
	ScyllaDBClusterExposure        ScyllaDBClusterExposureType
}

func NewTestFrameworkOptions() TestFrameworkOptions {
	return TestFrameworkOptions{
		ArtifactsDir:                   "",
		DeleteTestingNSPolicyUntyped:   string(framework.DeleteTestingNSPolicyAlways),
		IngressController:              &IngressControllerOptions{},
		ScyllaDBClusterExposureUntyped: string(ScyllaDBClusterExposureTypePodIP),
	}
}

func (o *TestFrameworkOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&o.ArtifactsDir, "artifacts-dir", "", o.ArtifactsDir, "A directory for storing test artifacts. No data is collected until set.")
	cmd.PersistentFlags().StringVarP(&o.DeleteTestingNSPolicyUntyped, "delete-namespace-policy", "", o.DeleteTestingNSPolicyUntyped, fmt.Sprintf("Namespace deletion policy. Allowed values are [%s].", strings.Join(
		[]string{
			string(framework.DeleteTestingNSPolicyAlways),
			string(framework.DeleteTestingNSPolicyNever),
			string(framework.DeleteTestingNSPolicyOnSuccess),
		},
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.IngressController.Address, "ingress-controller-address", "", o.IngressController.Address, "Overrides destination address when sending testing data to applications behind ingresses.")
	cmd.PersistentFlags().StringVarP(&o.IngressController.IngressClassName, "ingress-controller-ingress-class-name", "", o.IngressController.IngressClassName, "Ingress class name under which ingress controller is registered")
	cmd.PersistentFlags().StringToStringVarP(&o.IngressController.CustomAnnotations, "ingress-controller-custom-annotations", "", o.IngressController.CustomAnnotations, "Custom annotations required by the ingress controller")
	cmd.PersistentFlags().StringVarP(&o.ScyllaDBClusterExposureUntyped, "scylladb-cluster-exposure", "", o.ScyllaDBClusterExposureUntyped, fmt.Sprintf("ScyllaCluster exposure type. Allowed values are [%s].", strings.Join(
		[]string{
			string(ScyllaDBClusterExposureTypePodIP),
			string(ScyllaDBClusterExposureTypeClusterIP),
		},
		", ",
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

	switch t := ScyllaDBClusterExposureType(o.ScyllaDBClusterExposureUntyped); t {
	case ScyllaDBClusterExposureTypePodIP,
		ScyllaDBClusterExposureTypeClusterIP:
	default:
		errors = append(errors, fmt.Errorf("invalid ScyllaDBClusterExposure: %q", t))
	}

	return apierrors.NewAggregate(errors)
}

func (o *TestFrameworkOptions) Complete() error {
	o.DeleteTestingNSPolicy = framework.DeleteTestingNSPolicyType(o.DeleteTestingNSPolicyUntyped)

	// Trim spaces so we can reason later if the dir is set or not
	o.ArtifactsDir = strings.TrimSpace(o.ArtifactsDir)

	o.ScyllaDBClusterExposure = ScyllaDBClusterExposureType(o.ScyllaDBClusterExposureUntyped)

	return nil
}
