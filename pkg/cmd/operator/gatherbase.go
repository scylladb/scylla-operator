package operator

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
	kgenericclioptions "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	excludeResourceFlagHelpFormat = `Kubernetes resource to exclude, in the format kind[.group]. Can be specified multiple times.
Kind matching is case-insensitive; omit the group only for core resources (e.g., Pod, ConfigMap").
By default, the following are always excluded, despite the value of this flag: %s

Specifying this flag adds to the default exclusions. For example:
  --exclude-resource=Pod --exclude-resource=Deployment.apps

To force-include resources that are excluded by default, use --include-sensitive-resources`

	includeSensitiveResourcesFlagHelpFormat = `Controls whether sensitive resources (%s) should be collected. (default false)
When this flag is set, you can still exclude selected ones using --exclude-resource.
For example:
  --include-sensitive-resources --exclude-resource=Secret
will only exclude Secrets.`
)

// GatherBaseCLIFlags holds the command-line flags for the gather base options.
// These are raw values directly from the command line. They need to be validated and
// processed before use.
type GatherBaseCLIFlags struct {
	DestDir                   string
	CollectManagedFields      bool
	LogsLimitBytes            int64
	KeepGoing                 bool
	ExcludeResources          []string
	IncludeSensitiveResources bool
}

// NewGatherBaseCLIFlags creates a new GatherBaseCLIFlags with default values.
func NewGatherBaseCLIFlags() *GatherBaseCLIFlags {
	return &GatherBaseCLIFlags{
		DestDir:                   "",
		LogsLimitBytes:            0,
		CollectManagedFields:      false,
		KeepGoing:                 true,
		ExcludeResources:          []string{},
		IncludeSensitiveResources: false,
	}
}

type GatherBaseOptions struct {
	GathererName string
	ConfigFlags  *kgenericclioptions.ConfigFlags

	restConfig         *rest.Config
	kubeClient         kubernetes.Interface
	dynamicClient      dynamic.Interface
	discoveryClient    discovery.DiscoveryInterface
	resourcesToExclude []schema.GroupKind

	cliFlags *GatherBaseCLIFlags
}

func NewGatherBaseOptions(gathererName string) *GatherBaseOptions {
	return &GatherBaseOptions{
		GathererName: gathererName,
		ConfigFlags: kgenericclioptions.NewConfigFlags(true).WithWrapConfigFn(func(c *rest.Config) *rest.Config {
			c.UserAgent = genericclioptions.MakeVersionedUserAgent(fmt.Sprintf("scylla-operator-%s", gathererName))
			// Don't slow down artificially.
			c.QPS = math.MaxFloat32
			c.Burst = math.MaxInt
			return c
		}),
		cliFlags: NewGatherBaseCLIFlags(),
	}
}

func (o *GatherBaseOptions) AddFlags(flagset *pflag.FlagSet) {
	o.ConfigFlags.AddFlags(flagset)

	f := o.cliFlags
	flagset.StringVar(&f.DestDir, "dest-dir", f.DestDir, "Destination directory where to store the artifacts.")
	flagset.Int64Var(&f.LogsLimitBytes, "log-limit-bytes", f.LogsLimitBytes, "Maximum number of bytes collected for each log file, 0 means unlimited.")
	flagset.BoolVar(&f.CollectManagedFields, "managed-fields", f.CollectManagedFields, "Controls whether metadata.managedFields should be collected in the resource dumps.")
	flagset.BoolVar(&f.KeepGoing, "keep-going", f.KeepGoing, "Controls whether the collection should proceed to other resources over collection errors, accumulating errors.")
	flagset.StringArrayVar(&f.ExcludeResources, "exclude-resource", f.ExcludeResources, fmt.Sprintf(excludeResourceFlagHelpFormat, defaultExcludedSensitiveResourcesAsString()))
	flagset.BoolVar(&f.IncludeSensitiveResources, "include-sensitive-resources", f.IncludeSensitiveResources, fmt.Sprintf(includeSensitiveResourcesFlagHelpFormat, defaultExcludedSensitiveResourcesAsString()))
}

func (o *GatherBaseOptions) Validate() error {
	var errs []error

	f := o.cliFlags
	if f.LogsLimitBytes < 0 {
		errs = append(errs, fmt.Errorf("log-limit-bytes can't be lower then 0 but %v has been specified", f.LogsLimitBytes))
	}

	if len(f.DestDir) > 0 {
		files, err := os.ReadDir(f.DestDir)
		if err == nil {
			if len(files) > 0 {
				errs = append(errs, fmt.Errorf("destination directory %q is not empty", f.DestDir))
			}
		} else if !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("can't stat destination directory %q: %w", f.DestDir, err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *GatherBaseOptions) Complete() error {
	f := o.cliFlags

	restConfig, err := o.ConfigFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("can't create RESTConfig: %w", err)
	}

	o.restConfig = restConfig

	o.kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("can't build dynamic clientset: %w", err)
	}

	o.discoveryClient = cacheddiscovery.NewMemCacheClient(o.kubeClient.Discovery())

	ignoreDestDirExists := false
	if len(f.DestDir) == 0 {
		f.DestDir = fmt.Sprintf("%s-%s", o.GathererName, apimachineryutilrand.String(12))
	} else {
		// We have already made sure that the dir doesn't exist or is empty in validation.
		ignoreDestDirExists = true
	}

	err = os.Mkdir(f.DestDir, 0770)
	if err == nil {
		klog.InfoS("Created destination directory", "Path", f.DestDir)
	} else if os.IsExist(err) {
		// Just to be sure we cover it, but it's unlikely a dir with random suffix would already exist.
		if !ignoreDestDirExists {
			return fmt.Errorf("can't create destination directory %q because it already exists: %w", f.DestDir, err)
		}
	} else {
		return fmt.Errorf("can't create destination directory %q: %w", f.DestDir, err)
	}

	for _, exclude := range f.ExcludeResources {
		o.resourcesToExclude = append(o.resourcesToExclude, schema.ParseGroupKind(exclude))
	}

	return nil
}

func (o *GatherBaseOptions) RunInit(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	f := o.cliFlags

	err := flag.Set("logtostderr", "false")
	if err != nil {
		return fmt.Errorf("can't set logtostderr flag: %w", err)
	}

	err = flag.Set("alsologtostderr", "true")
	if err != nil {
		return fmt.Errorf("can't set alsologtostderr flag: %w", err)
	}

	err = flag.Set("log_file", filepath.Join(f.DestDir, fmt.Sprintf("%s.log", o.GathererName)))
	if err != nil {
		return fmt.Errorf("can't set log_file flag: %w", err)
	}

	flag.Parse()

	cmdutil.LogCommandStarting(cmd)
	cliflag.PrintFlags(cmd.Flags())

	return nil
}

func (o *GatherBaseOptions) GetPrinters() []collect.ResourcePrinterInterface {
	f := o.cliFlags
	printers := make([]collect.ResourcePrinterInterface, 0, 1)

	if f.CollectManagedFields {
		printers = append(printers, &collect.YAMLPrinter{})
	} else {
		printers = append(printers, &collect.OmitManagedFieldsPrinter{
			Delegate: &collect.YAMLPrinter{},
		})
	}

	return printers
}

// defaultExcludedSensitiveResourcesAsString returns a string representation of the default excluded sensitive resources for use in flag help.
func defaultExcludedSensitiveResourcesAsString() string {
	var ss []string
	for _, gk := range collect.DefaultExcludedSensitiveResources() {
		ss = append(ss, gk.String())
	}
	return strings.Join(ss, ", ")
}
