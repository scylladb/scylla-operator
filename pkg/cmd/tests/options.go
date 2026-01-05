package tests

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
)

const (
	// https://github.com/distribution/reference/blob/8c942b0459dfdcc5b6685581dd0a5a470f615bff/regexp.go#L68
	referenceTagRegexp = `[\w][\w.-]{0,127}`

	// https://github.com/distribution/reference/blob/8c942b0459dfdcc5b6685581dd0a5a470f615bff/regexp.go#L81
	digestRegexp = `[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`
)

var tagWithOptionalDigestRegexp = regexp.MustCompile("^" + referenceTagRegexp + "(?:@" + digestRegexp + ")?$")

type IngressControllerOptions struct {
	Address           string
	IngressClassName  string
	CustomAnnotations map[string]string
}

type ScyllaClusterOptions struct {
	NodeServiceType             string
	NodesBroadcastAddressType   string
	ClientsBroadcastAddressType string
	StorageClassName            string
	ReactorBackend              string
}

var supportedNodeServiceTypes = []scyllav1.NodeServiceType{
	scyllav1.NodeServiceTypeHeadless,
	scyllav1.NodeServiceTypeClusterIP,
}

var supportedBroadcastAddressTypes = []scyllav1.BroadcastAddressType{
	scyllav1.BroadcastAddressTypePodIP,
	scyllav1.BroadcastAddressTypeServiceClusterIP,
}

var supportedReactorBackends = []string{
	"io_uring",
	"linux-aio",
}

type TestFrameworkOptions struct {
	genericclioptions.MultiDatacenterClientConfig
	ObjectStorageOptions

	ArtifactsDir                string
	CleanupPolicyUntyped        string
	CleanupPolicy               framework.CleanupPolicyType
	IngressController           *IngressControllerOptions
	ScyllaClusterOptionsUntyped *ScyllaClusterOptions
	scyllaClusterOptions        *framework.ScyllaClusterOptions
	ScyllaDBVersion             string
	ScyllaDBManagerVersion      string
	ScyllaDBManagerAgentVersion string
	ScyllaDBUpdateFrom          string
	ScyllaDBUpgradeFrom         string
}

func NewTestFrameworkOptions(streams genericclioptions.IOStreams, userAgent string) *TestFrameworkOptions {
	return &TestFrameworkOptions{
		MultiDatacenterClientConfig: genericclioptions.NewMultiDatacenterClientConfig(userAgent),
		ArtifactsDir:                "",
		CleanupPolicyUntyped:        string(framework.CleanupPolicyAlways),
		IngressController:           &IngressControllerOptions{},
		ScyllaClusterOptionsUntyped: &ScyllaClusterOptions{
			NodeServiceType:             string(scyllav1.NodeServiceTypeHeadless),
			NodesBroadcastAddressType:   string(scyllav1.BroadcastAddressTypePodIP),
			ClientsBroadcastAddressType: string(scyllav1.BroadcastAddressTypePodIP),
			StorageClassName:            "",
			ReactorBackend:              "", // Leaving empty to rely on ScyllaDB default.
		},
		ObjectStorageOptions:        NewObjectStorageOptions(),
		ScyllaDBVersion:             configassets.Project.Operator.ScyllaDBVersion,
		ScyllaDBManagerVersion:      configassets.Project.Operator.ScyllaDBManagerVersion,
		ScyllaDBManagerAgentVersion: configassets.Project.Operator.ScyllaDBManagerAgentVersion,
		ScyllaDBUpdateFrom:          configassets.Project.OperatorTests.ScyllaDBVersions.UpdateFrom,
		ScyllaDBUpgradeFrom:         configassets.Project.OperatorTests.ScyllaDBVersions.UpgradeFrom,
	}
}

func (o *TestFrameworkOptions) AddFlags(cmd *cobra.Command) {
	o.MultiDatacenterClientConfig.AddFlags(cmd)
	o.ObjectStorageOptions.AddFlags(cmd)

	cmd.PersistentFlags().StringVarP(&o.ArtifactsDir, "artifacts-dir", "", o.ArtifactsDir, "A directory for storing test artifacts. No data is collected until set.")
	cmd.PersistentFlags().StringVarP(&o.CleanupPolicyUntyped, "delete-namespace-policy", "", o.CleanupPolicyUntyped, fmt.Sprintf("Namespace deletion policy. Allowed values are [%s].", strings.Join(
		[]string{
			string(framework.CleanupPolicyAlways),
			string(framework.CleanupPolicyNever),
			string(framework.CleanupPolicyOnSuccess),
		},
		", ",
	)))
	apimachineryutilruntime.Must(cmd.PersistentFlags().MarkDeprecated("delete-namespace-policy", "--delete-namespace-policy is deprecated - please use --cleanup-policy instead"))
	cmd.PersistentFlags().StringVarP(&o.CleanupPolicyUntyped, "cleanup-policy", "", o.CleanupPolicyUntyped, fmt.Sprintf("Cleanup policy. Allowed values are [%s].", strings.Join(
		[]string{
			string(framework.CleanupPolicyAlways),
			string(framework.CleanupPolicyNever),
			string(framework.CleanupPolicyOnSuccess),
		},
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.IngressController.Address, "ingress-controller-address", "", o.IngressController.Address, "Overrides destination address when sending testing data to applications behind ingresses.")
	cmd.PersistentFlags().StringVarP(&o.IngressController.IngressClassName, "ingress-controller-ingress-class-name", "", o.IngressController.IngressClassName, "Ingress class name under which ingress controller is registered")
	cmd.PersistentFlags().StringToStringVarP(&o.IngressController.CustomAnnotations, "ingress-controller-custom-annotations", "", o.IngressController.CustomAnnotations, "Custom annotations required by the ingress controller")
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.NodeServiceType, "scyllacluster-node-service-type", "", o.ScyllaClusterOptionsUntyped.NodeServiceType, fmt.Sprintf("Kubernetes service type that the ScyllaCluster nodes are exposed with. Allowed values are [%s].", strings.Join(
		oslices.ConvertSlice(supportedNodeServiceTypes, oslices.ToString[scyllav1.NodeServiceType]),
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType, "scyllacluster-nodes-broadcast-address-type", "", o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType, fmt.Sprintf("Type of address that the ScyllaCluster nodes broadcast for communication with other nodes. Allowed values are [%s].", strings.Join(
		oslices.ConvertSlice(supportedBroadcastAddressTypes, oslices.ToString[scyllav1.BroadcastAddressType]),
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType, "scyllacluster-clients-broadcast-address-type", "", o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType, fmt.Sprintf("Type of address that the ScyllaCluster nodes broadcast for communication with clients. Allowed values are [%s].", strings.Join(
		oslices.ConvertSlice(supportedBroadcastAddressTypes, oslices.ToString[scyllav1.BroadcastAddressType]),
		", ",
	)))
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.StorageClassName, "scyllacluster-storageclass-name", "", o.ScyllaClusterOptionsUntyped.StorageClassName, fmt.Sprintf("Name of the StorageClass to request for ScyllaCluster storage."))
	cmd.PersistentFlags().StringVarP(&o.ScyllaClusterOptionsUntyped.ReactorBackend, "scyllacluster-reactor-backend", "", o.ScyllaClusterOptionsUntyped.ReactorBackend, fmt.Sprintf("Name of the reactor backend to use for ScyllaCluster."))
	cmd.PersistentFlags().StringVarP(&o.ScyllaDBVersion, "scylladb-version", "", o.ScyllaDBVersion, "Version of ScyllaDB to use.")
	cmd.PersistentFlags().StringVarP(&o.ScyllaDBManagerVersion, "scylladb-manager-version", "", o.ScyllaDBManagerVersion, "Version of Scylla Manager to use.")
	cmd.PersistentFlags().StringVarP(&o.ScyllaDBManagerAgentVersion, "scylladb-manager-agent-version", "", o.ScyllaDBManagerAgentVersion, "Version of Scylla Manager Agent to use.")
	cmd.PersistentFlags().StringVarP(&o.ScyllaDBUpdateFrom, "scylladb-update-from-version", "", o.ScyllaDBUpdateFrom, "Version of ScyllaDB to update from.")
	cmd.PersistentFlags().StringVarP(&o.ScyllaDBUpgradeFrom, "scylladb-upgrade-from-version", "", o.ScyllaDBUpgradeFrom, "Version of ScyllaDB to upgrade from.")
}

func (o *TestFrameworkOptions) Validate(args []string) error {
	var errors []error

	err := o.MultiDatacenterClientConfig.Validate()
	if err != nil {
		errors = append(errors, err)
	}

	switch p := framework.CleanupPolicyType(o.CleanupPolicyUntyped); p {
	case framework.CleanupPolicyAlways,
		framework.CleanupPolicyOnSuccess,
		framework.CleanupPolicyNever:
	default:
		errors = append(errors, fmt.Errorf("invalid DeleteTestingNSPolicy: %q", p))
	}

	if !oslices.ContainsItem(supportedNodeServiceTypes, scyllav1.NodeServiceType(o.ScyllaClusterOptionsUntyped.NodeServiceType)) {
		errors = append(errors, fmt.Errorf("invalid scyllacluster-node-service-type: %q", o.ScyllaClusterOptionsUntyped.NodeServiceType))
	}

	if !oslices.ContainsItem(supportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType)) {
		errors = append(errors, fmt.Errorf("invalid scyllacluster-nodes-broadcast-address-type: %q", o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType))
	}

	if !oslices.ContainsItem(supportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType)) {
		errors = append(errors, fmt.Errorf("invalid scyllacluster-clients-broadcast-address-type: %q", o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType))
	}

	if backend := o.ScyllaClusterOptionsUntyped.ReactorBackend; backend != "" && !oslices.ContainsItem(supportedReactorBackends, backend) {
		errors = append(errors, fmt.Errorf("invalid scyllacluster-reactor-backend: %q", o.ScyllaClusterOptionsUntyped.ReactorBackend))
	}

	if !tagWithOptionalDigestRegexp.MatchString(o.ScyllaDBVersion) {
		errors = append(errors, fmt.Errorf(
			"invalid scylladb-version format: %q. Expected format: <tag>[@<digest>]",
			o.ScyllaDBVersion,
		))
	}

	if !tagWithOptionalDigestRegexp.MatchString(o.ScyllaDBUpdateFrom) {
		errors = append(errors, fmt.Errorf(
			"invalid scylladb-update-from-version format: %q. Expected format: <tag>[@<digest>]",
			o.ScyllaDBUpdateFrom,
		))
	}

	if !tagWithOptionalDigestRegexp.MatchString(o.ScyllaDBUpgradeFrom) {
		errors = append(errors, fmt.Errorf(
			"invalid scylladb-upgrade-from-version format: %q. Expected format: <tag>[@<digest>]",
			o.ScyllaDBUpgradeFrom,
		))
	}

	if !tagWithOptionalDigestRegexp.MatchString(o.ScyllaDBManagerVersion) {
		errors = append(errors, fmt.Errorf(
			"invalid scylladb-manager-version format: %q. Expected format: <tag>[@<digest>]",
			o.ScyllaDBManagerVersion,
		))
	}

	if !tagWithOptionalDigestRegexp.MatchString(o.ScyllaDBManagerAgentVersion) {
		errors = append(errors, fmt.Errorf(
			"invalid scylladb-manager-agent-version format: %q. Expected format: <tag>[@<digest>]",
			o.ScyllaDBManagerAgentVersion,
		))
	}

	if len(o.ArtifactsDir) > 0 {
		_, err = os.Stat(o.ArtifactsDir)
		if err != nil {
			if os.IsNotExist(err) {
				errors = append(errors, fmt.Errorf("artifacts directory %q does not exist", o.ArtifactsDir))
			} else {
				errors = append(errors, fmt.Errorf("can't inspect artifacts directory %q", o.ArtifactsDir))
			}
		}
	}

	return apimachineryutilerrors.NewAggregate(errors)
}

func (o *TestFrameworkOptions) Complete(args []string) error {
	if err := o.MultiDatacenterClientConfig.Complete(); err != nil {
		return fmt.Errorf("can't complete multi-datacenter client config: %w", err)
	}
	if err := o.ObjectStorageOptions.Complete(); err != nil {
		return fmt.Errorf("can't complete object storage options: %w", err)
	}

	o.CleanupPolicy = framework.CleanupPolicyType(o.CleanupPolicyUntyped)

	// Trim spaces so we can reason later if the dir is set or not
	o.ArtifactsDir = strings.TrimSpace(o.ArtifactsDir)

	o.scyllaClusterOptions = &framework.ScyllaClusterOptions{
		ExposeOptions: framework.ExposeOptions{
			NodeServiceType:             scyllav1.NodeServiceType(o.ScyllaClusterOptionsUntyped.NodeServiceType),
			NodesBroadcastAddressType:   scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.NodesBroadcastAddressType),
			ClientsBroadcastAddressType: scyllav1.BroadcastAddressType(o.ScyllaClusterOptionsUntyped.ClientsBroadcastAddressType),
		},
		StorageClassName: o.ScyllaClusterOptionsUntyped.StorageClassName,
		ReactorBackend:   o.ScyllaClusterOptionsUntyped.ReactorBackend,
	}

	workerRestConfigs := make(map[string]*rest.Config, len(o.WorkerClientConfigs))
	for worker, config := range o.WorkerClientConfigs {
		workerRestConfigs[worker] = config.RestConfig
	}

	framework.TestContext = &framework.TestContextType{
		RestConfig:                         o.ClientConfig.RestConfig,
		WorkerRestConfigs:                  workerRestConfigs,
		ArtifactsDir:                       o.ArtifactsDir,
		CleanupPolicy:                      o.CleanupPolicy,
		ScyllaClusterOptions:               o.scyllaClusterOptions,
		ScyllaDBVersion:                    o.ScyllaDBVersion,
		ScyllaDBManagerVersion:             o.ScyllaDBManagerVersion,
		ScyllaDBManagerAgentVersion:        o.ScyllaDBManagerAgentVersion,
		ScyllaDBUpdateFrom:                 o.ScyllaDBUpdateFrom,
		ScyllaDBUpgradeFrom:                o.ScyllaDBUpgradeFrom,
		ClusterObjectStorageSettings:       o.ClusterObjectStorageSettings,
		WorkerClusterObjectStorageSettings: o.WorkerClusterObjectStorageSettings,
	}

	if o.IngressController != nil {
		framework.TestContext.IngressController = &framework.IngressController{
			Address:           o.IngressController.Address,
			IngressClassName:  o.IngressController.IngressClassName,
			CustomAnnotations: o.IngressController.CustomAnnotations,
		}
	}

	return nil
}
