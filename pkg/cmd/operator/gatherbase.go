package operator

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	kgenericclioptions "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type GatherBaseOptions struct {
	GathererName string
	ConfigFlags  *kgenericclioptions.ConfigFlags

	kubeClient      kubernetes.Interface
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface

	DestDir              string
	CollectManagedFields bool
	LogsLimitBytes       int64
	KeepGoing            bool
}

func NewGatherBaseOptions(gathererName string, keepGoing bool) *GatherBaseOptions {
	return &GatherBaseOptions{
		GathererName: gathererName,
		ConfigFlags: kgenericclioptions.NewConfigFlags(true).WithWrapConfigFn(func(c *rest.Config) *rest.Config {
			c.UserAgent = genericclioptions.MakeVersionedUserAgent(fmt.Sprintf("scylla-operator-%s", gathererName))
			// Don't slow down artificially.
			c.QPS = math.MaxFloat32
			c.Burst = math.MaxInt
			return c
		}),
		DestDir:              "",
		CollectManagedFields: false,
		LogsLimitBytes:       0,
		KeepGoing:            keepGoing,
	}
}

func (o *GatherBaseOptions) AddFlags(flagset *pflag.FlagSet) {
	o.ConfigFlags.AddFlags(flagset)

	flagset.StringVarP(&o.DestDir, "dest-dir", "", o.DestDir, "Destination directory where to store the artifacts.")
	flagset.Int64VarP(&o.LogsLimitBytes, "log-limit-bytes", "", o.LogsLimitBytes, "Maximum number of bytes collected for each log file, 0 means unlimited.")
	flagset.BoolVarP(&o.CollectManagedFields, "managed-fields", "", o.CollectManagedFields, "Controls whether metadata.managedFields should be collected in the resource dumps.")
	flagset.BoolVarP(&o.KeepGoing, "keep-going", "", o.KeepGoing, "Controls whether the collection should proceed to other resources over collection errors, accumulating errors.")
}

func (o *GatherBaseOptions) Validate() error {
	var errs []error

	if o.LogsLimitBytes < 0 {
		errs = append(errs, fmt.Errorf("log-limit-bytes can't be lower then 0 but %v has been specified", o.LogsLimitBytes))
	}

	if len(o.DestDir) > 0 {
		files, err := os.ReadDir(o.DestDir)
		if err == nil {
			if len(files) > 0 {
				errs = append(errs, fmt.Errorf("destination directory %q is not empty", o.DestDir))
			}
		} else {
			if os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("destination directory %q doesn't exist", o.DestDir))
			} else {
				errs = append(errs, fmt.Errorf("can't stat destination directory %q: %w", o.DestDir, err))
			}
		}
	}

	return apierrors.NewAggregate(errs)
}

func (o *GatherBaseOptions) Complete() error {
	restConfig, err := o.ConfigFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("can't create RESTConfig: %w", err)
	}

	o.kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("can't build dynamic clientset: %w", err)
	}

	o.discoveryClient = cacheddiscovery.NewMemCacheClient(o.kubeClient.Discovery())

	if len(o.DestDir) == 0 {
		o.DestDir = fmt.Sprintf("%s-%s", o.GathererName, utilrand.String(12))
		err := os.Mkdir(o.DestDir, 0770)
		if err != nil {
			return fmt.Errorf("can't create destination directory %q: %w", o.DestDir, err)
		}
		klog.InfoS("Created destination directory", "Path", o.DestDir)
	}

	return nil
}

func (o *GatherBaseOptions) RunInit(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	err := flag.Set("logtostderr", "false")
	if err != nil {
		return fmt.Errorf("can't set logtostderr flag: %w", err)
	}

	err = flag.Set("alsologtostderr", "true")
	if err != nil {
		return fmt.Errorf("can't set alsologtostderr flag: %w", err)
	}

	err = flag.Set("log_file", filepath.Join(o.DestDir, fmt.Sprintf("%s.log", o.GathererName)))
	if err != nil {
		return fmt.Errorf("can't set log_file flag: %w", err)
	}

	flag.Parse()

	klog.InfoS("Program info", "Command", cmd.Name(), "Version", version.Get())
	cliflag.PrintFlags(cmd.Flags())

	return nil
}

func (o *GatherBaseOptions) GetPrinters() []collect.ResourcePrinterInterface {
	printers := make([]collect.ResourcePrinterInterface, 0, 1)

	if o.CollectManagedFields {
		printers = append(printers, &collect.YAMLPrinter{})
	} else {
		printers = append(printers, &collect.OmitManagedFieldsPrinter{
			Delegate: &collect.YAMLPrinter{},
		})
	}

	return printers
}
