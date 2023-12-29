package tests

import (
	"fmt"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
)

type IngressControllerOptions struct {
	Address           string
	IngressClassName  string
	CustomAnnotations map[string]string
}

type ScyllaClusterOptions struct {
	NodeServiceType             string
	NodesBroadcastAddressType   string
	ClientsBroadcastAddressType string
}

var supportedNodeServiceTypes = []scyllav1.NodeServiceType{
	scyllav1.NodeServiceTypeHeadless,
	scyllav1.NodeServiceTypeClusterIP,
}

var supportedBroadcastAddressTypes = []scyllav1.BroadcastAddressType{
	scyllav1.BroadcastAddressTypePodIP,
	scyllav1.BroadcastAddressTypeServiceClusterIP,
}

type TestFrameworkOptions struct {
	ArtifactsDir                 string
	DeleteTestingNSPolicyUntyped string
	DeleteTestingNSPolicy        framework.DeleteTestingNSPolicyType
	IngressController            *IngressControllerOptions
	ScyllaClusterOptionsUntyped  *ScyllaClusterOptions
	scyllaClusterOptions         *framework.ScyllaClusterOptions
}

func NewTestFrameworkOptions() TestFrameworkOptions {
	return TestFrameworkOptions{
		ArtifactsDir:                 "",
		DeleteTestingNSPolicyUntyped: string(framework.DeleteTestingNSPolicyAlways),
		IngressController:            &IngressControllerOptions{},
		ScyllaClusterOptionsUntyped: &ScyllaClusterOptions{
			NodeServiceType:             string(scyllav1.NodeServiceTypeHeadless),
			NodesBroadcastAddressType:   string(scyllav1.BroadcastAddressTypePodIP),
			ClientsBroadcastAddressType: string(scyllav1.BroadcastAddressTypePodIP),
		},
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
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.NodeServiceType, "scyllacluster-node-service-type", "", o.ScyllaClusterOptionsUntyped.NodeServiceType, fmt.Sprintf("Kubernetes service type that the ScyllaCluster nodes are exposed with. Allowed values are [%s].", strings.Join(
		slices.ConvertSlice(supportedNodeServiceTypes, slices.ToString[scyllav1.NodeServiceType]),
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType, "scyllacluster-nodes-broadcast-address-type", "", o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType, fmt.Sprintf("Type of address that the ScyllaCluster nodes broadcast for communication with other nodes. Allowed values are [%s].", strings.Join(
		slices.ConvertSlice(supportedBroadcastAddressTypes, slices.ToString[scyllav1.BroadcastAddressType]),
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType, "scyllacluster-clients-broadcast-address-type", "", o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType, fmt.Sprintf("Type of address that the ScyllaCluster nodes broadcast for communication with clients. Allowed values are [%s].", strings.Join(
		slices.ConvertSlice(supportedBroadcastAddressTypes, slices.ToString[scyllav1.BroadcastAddressType]),
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

	if !slices.ContainsItem(supportedNodeServiceTypes, scyllav1.NodeServiceType(o.ScyllaClusterOptionsUntyped.NodeServiceType)) {
		errors = append(errors, fmt.Errorf("invalid scylla-cluster-node-service-type: %q", o.ScyllaClusterOptionsUntyped.NodeServiceType))
	}

	if !slices.ContainsItem(supportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType)) {
		errors = append(errors, fmt.Errorf("invalid scylla-cluster-nodes-broadcast-address-type: %q", o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType))
	}

	if !slices.ContainsItem(supportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType)) {
		errors = append(errors, fmt.Errorf("invalid scylla-cluster-clients-broadcast-address-type: %q", o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType))
	}

	return apierrors.NewAggregate(errors)
}

func (o *TestFrameworkOptions) Complete() error {
	o.DeleteTestingNSPolicy = framework.DeleteTestingNSPolicyType(o.DeleteTestingNSPolicyUntyped)

	// Trim spaces so we can reason later if the dir is set or not
	o.ArtifactsDir = strings.TrimSpace(o.ArtifactsDir)

	o.scyllaClusterOptions = &framework.ScyllaClusterOptions{
		ExposeOptions: framework.ExposeOptions{
			NodeServiceType:             scyllav1.NodeServiceType(o.ScyllaClusterOptionsUntyped.NodeServiceType),
			NodesBroadcastAddressType:   scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType),
			ClientsBroadcastAddressType: scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType),
		},
	}

	return nil
}
