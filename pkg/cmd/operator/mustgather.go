package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	mustGatherLongDescription = templates.LongDesc(`
		must-gather collects a dump of resources required for debugging.
	
		This command is experimental and subject to change without notice.
	`)

	mustGatherExample = templates.Examples(`
		# Collect archive of all resources related to scyllacluster and its APIs.
		scylla-operator must-gather
	
		# Collect archive of all resources related to scyllacluster and its APIs.
		# Namespaced APIs targeted for collection will be limited to the supplied namespace.
		# (E.g. this will limit ScyllaClusters to the supplied namespace, 
		#       unless some other resource requests them collected.)
		scylla-operator must-gather -n my-namespace
		
		# Collect archive of all resources present in the Kubernetes cluster.
		scylla-operator must-gather --all-resources

		# Collect archive of all resources present in the Kubernetes cluster,
		# excluding LimitRange and DeviceClass.
		scylla-operator must-gather --all-resources --exclude-resource=LimitRange --exclude-resource=DeviceClass.resource.k8s.io
	`)
)

type MustGatherOptions struct {
	*GatherBaseOptions

	AllResources bool
}

func NewMustGatherOptions(streams genericclioptions.IOStreams) *MustGatherOptions {
	options := &MustGatherOptions{
		GatherBaseOptions: NewGatherBaseOptions("scylla-operator-must-gather"),
		AllResources:      false,
	}

	return options
}

func (o *MustGatherOptions) AddFlags(flagset *pflag.FlagSet) {
	o.GatherBaseOptions.AddFlags(flagset)

	flagset.BoolVarP(&o.AllResources, "all-resources", "", o.AllResources, "Gather will discover preferred API resources from the apiserver.")
}

func NewMustGatherCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMustGatherOptions(streams)

	cmd := &cobra.Command{
		Use:     "must-gather",
		Short:   "Run the scylla must-gather.",
		Long:    mustGatherLongDescription,
		Example: mustGatherExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},
		ValidArgs: []string{},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

func (o *MustGatherOptions) Validate() error {
	var errs []error

	errs = append(errs, o.GatherBaseOptions.Validate())

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *MustGatherOptions) Complete() error {
	err := o.GatherBaseOptions.Complete()
	if err != nil {
		return err
	}

	return nil
}

func (o *MustGatherOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
	err := o.GatherBaseOptions.RunInit(originalStreams, cmd)
	if err != nil {
		return err
	}

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.run(ctx)
}

func findResource(preferredResources []*collect.ResourceInfo, gr schema.GroupResource) (*collect.ResourceInfo, error) {
	for _, ri := range preferredResources {
		if ri.Resource.GroupResource() == gr {
			return ri, nil
		}
	}

	return nil, fmt.Errorf("can't find resource %q", gr)
}

var defaultCollectedResourceGroups = []schema.GroupResource{
	{
		Resource: "scyllaclusters",
		Group:    "scylla.scylladb.com",
	},
	{
		Resource: "scyllaoperatorconfigs",
		Group:    "scylla.scylladb.com",
	},
	{
		Resource: "nodeconfigs",
		Group:    "scylla.scylladb.com",
	},
	{
		Resource: "customresourcedefinitions",
		Group:    "apiextensions.k8s.io",
	},
	{
		Resource: "nodes",
		Group:    "",
	},
	{
		Resource: "storageclasses",
		Group:    "storage.k8s.io",
	},
	{
		Resource: "validatingwebhookconfigurations",
		Group:    "admissionregistration.k8s.io",
	},
	{
		Resource: "mutatingwebhookconfigurations",
		Group:    "admissionregistration.k8s.io",
	},
}

var defaultCollectedNamespaces = []string{
	"scylla-operator",
	"scylla-manager",
	"scylla-operator-node-tuning",
}

func (o *MustGatherOptions) run(ctx context.Context) error {
	startTime := time.Now()
	klog.InfoS("Gathering artifacts", "DestDir", o.cliFlags.DestDir)
	defer func() {
		klog.InfoS("Finished gathering artifacts", "Duration", time.Since(startTime))
	}()

	discoverer := collect.NewResourceDiscoverer(o.cliFlags.IncludeSensitiveResources, o.discoveryClient)
	discoveredResources, err := discoverer.DiscoverResources(o.resourcesToExclude...)
	if err != nil {
		return fmt.Errorf("can't discover resources: %w", err)
	}

	collector := collect.NewCollector(
		o.cliFlags.DestDir,
		o.GetPrinters(),
		o.restConfig,
		discoveredResources,
		o.kubeClient.CoreV1(),
		o.dynamicClient,
		true,
		o.cliFlags.KeepGoing,
		o.cliFlags.LogsLimitBytes,
	)

	if o.AllResources {
		return collector.CollectResourcesObjects(ctx, discoveredResources)
	}
	return o.collectDefaultResources(ctx, collector, discoveredResources)
}

func (o *MustGatherOptions) collectDefaultResources(ctx context.Context, collector *collect.Collector, discoveredResources []*collect.ResourceInfo) error {
	var errs []error

	for _, s := range defaultCollectedResourceGroups {
		ri, err := findResource(discoveredResources, s)
		if err != nil {
			return fmt.Errorf("can't find resource in preferred resources: %w", err)
		}

		namespace := corev1.NamespaceAll
		if ri.Scope.Name() == meta.RESTScopeNameNamespace &&
			o.GatherBaseOptions.ConfigFlags.Namespace != nil {
			namespace = *o.GatherBaseOptions.ConfigFlags.Namespace
		}
		if err := collector.CollectResourceObjects(ctx, ri, namespace); err != nil {
			errs = append(errs, fmt.Errorf("can't collect resource %q: %w", ri.Resource, err))
		}
	}

	for _, ns := range defaultCollectedNamespaces {
		if err := collector.CollectResourceObject(ctx, &collect.ResourceInfo{
			Scope:    meta.RESTScopeRoot,
			Resource: corev1.SchemeGroupVersion.WithResource("namespaces"),
		}, "", ns); err != nil {
			if apierrors.IsNotFound(err) {
				klog.InfoS("Namespace not found", "Namespace", ns)
			} else {
				errs = append(errs, fmt.Errorf("can't collect namespace %q: %w", ns, err))
			}
		}
	}
	return apimachineryutilerrors.NewAggregate(errs)
}
