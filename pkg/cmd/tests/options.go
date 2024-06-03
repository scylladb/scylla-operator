package tests

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/onsi/ginkgo/v2"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
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
	genericclioptions.ClientConfigSet

	ArtifactsDir                 string
	DeleteTestingNSPolicyUntyped string
	DeleteTestingNSPolicy        framework.DeleteTestingNSPolicyType
	IngressController            *IngressControllerOptions
	ScyllaClusterOptionsUntyped  *ScyllaClusterOptions
	scyllaClusterOptions         *framework.ScyllaClusterOptions
	ObjectStorageBucket          string
	GCSServiceAccountKeyPath     string
	objectStorageType            framework.ObjectStorageType
	gcsServiceAccountKey         []byte
}

func NewTestFrameworkOptions(streams genericclioptions.IOStreams, userAgent string) *TestFrameworkOptions {
	return &TestFrameworkOptions{
		ClientConfigSet:              genericclioptions.NewClientConfigSet(userAgent),
		ArtifactsDir:                 "",
		DeleteTestingNSPolicyUntyped: string(framework.DeleteTestingNSPolicyAlways),
		IngressController:            &IngressControllerOptions{},
		ScyllaClusterOptionsUntyped: &ScyllaClusterOptions{
			NodeServiceType:             string(scyllav1.NodeServiceTypeHeadless),
			NodesBroadcastAddressType:   string(scyllav1.BroadcastAddressTypePodIP),
			ClientsBroadcastAddressType: string(scyllav1.BroadcastAddressTypePodIP),
		},
		ObjectStorageBucket:      "",
		GCSServiceAccountKeyPath: "",
		objectStorageType:        framework.ObjectStorageTypeNone,
		gcsServiceAccountKey:     []byte{},
	}
}

func (o *TestFrameworkOptions) AddFlags(cmd *cobra.Command) {
	o.ClientConfigSet.AddFlags(cmd)

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
	cmd.PersistentFlags().StringVarP(&o.ObjectStorageBucket, "object-storage-bucket", "", o.ObjectStorageBucket, "Name of the object storage bucket.")
	cmd.PersistentFlags().StringVarP(&o.GCSServiceAccountKeyPath, "gcs-service-account-key-path", "", o.GCSServiceAccountKeyPath, "Path to a file containing a GCS service account key.")
}

func (o *TestFrameworkOptions) Validate(args []string) error {
	var errors []error

	err := o.ClientConfigSet.Validate()
	if err != nil {
		errors = append(errors, err)
	}

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

	if len(o.GCSServiceAccountKeyPath) > 0 && len(o.ObjectStorageBucket) == 0 {
		errors = append(errors, fmt.Errorf("object-storage-bucket can't be empty when gcs-service-account-key-path is provided"))
	}

	if len(o.ObjectStorageBucket) > 0 && len(o.GCSServiceAccountKeyPath) == 0 {
		errors = append(errors, fmt.Errorf("gcs-service-account-key-path can't be empty when object-storage-bucket is provided"))
	}

	return apierrors.NewAggregate(errors)
}

func (o *TestFrameworkOptions) Complete(args []string) error {
	err := o.ClientConfigSet.Complete()
	if err != nil {
		return err
	}

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

	if len(o.GCSServiceAccountKeyPath) > 0 {
		o.objectStorageType = framework.ObjectStorageTypeGCS
		gcsServiceAccountKey, err := os.ReadFile(o.GCSServiceAccountKeyPath)
		if err != nil {
			return fmt.Errorf("can't read gcs service account key file %q: %w", o.GCSServiceAccountKeyPath, err)
		}
		if len(gcsServiceAccountKey) == 0 {
			return fmt.Errorf("gcs service account key file %q can't be empty", o.GCSServiceAccountKeyPath)
		}
		o.gcsServiceAccountKey = gcsServiceAccountKey
	}

	framework.TestContext = &framework.TestContextType{
		RestConfigs: slices.ConvertSlice(o.ClientConfigs, func(cc genericclioptions.ClientConfig) *rest.Config {
			return cc.RestConfig
		}),
		ArtifactsDir:          o.ArtifactsDir,
		DeleteTestingNSPolicy: o.DeleteTestingNSPolicy,
		ScyllaClusterOptions:  o.scyllaClusterOptions,
		ObjectStorageType:     o.objectStorageType,
		ObjectStorageBucket:   o.ObjectStorageBucket,
		GCSServiceAccountKey:  o.gcsServiceAccountKey,
	}

	if o.IngressController != nil {
		framework.TestContext.IngressController = &framework.IngressController{
			Address:           o.IngressController.Address,
			IngressClassName:  o.IngressController.IngressClassName,
			CustomAnnotations: o.IngressController.CustomAnnotations,
		}
	}

	if len(o.ArtifactsDir) != 0 {
		_, reporterConfig := ginkgo.GinkgoConfiguration()
		reporterConfig.JUnitReport = path.Join(o.ArtifactsDir, "e2e.junit.xml")
		reporterConfig.JSONReport = path.Join(o.ArtifactsDir, "e2e.json")
	}

	return nil
}
