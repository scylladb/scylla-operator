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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/discovery"
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
	`)
)

type MustGatherOptions struct {
	*GatherBaseOptions

	AllResources bool
}

func NewMustGatherOptions(streams genericclioptions.IOStreams) *MustGatherOptions {
	return &MustGatherOptions{
		GatherBaseOptions: NewGatherBaseOptions("scylla-operator-must-gather", true),
		AllResources:      false,
	}
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

	return utilerrors.NewAggregate(errs)
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

type resourceSpec struct {
	collect.ResourceInfo
	Namespace, Name string
}

var mustGatherSpecs = []struct {
	schema.GroupResource
	Namespace, Name string
}{
	{
		GroupResource: schema.GroupResource{
			Resource: "scyllaclusters",
			Group:    "scylla.scylladb.com",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "scyllaoperatorconfigs",
			Group:    "scylla.scylladb.com",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "nodeconfigs",
			Group:    "scylla.scylladb.com",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "namespaces",
			Group:    "",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "scylla-operator",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "namespaces",
			Group:    "",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "scylla-manager",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "namespaces",
			Group:    "",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "scylla-operator-node-tuning",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "customresourcedefinitions",
			Group:    "apiextensions.k8s.io",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "nodes",
			Group:    "",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "validatingwebhookconfigurations",
			Group:    "admissionregistration.k8s.io",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
	{
		GroupResource: schema.GroupResource{
			Resource: "mutatingwebhookconfigurations",
			Group:    "admissionregistration.k8s.io",
		},
		Namespace: corev1.NamespaceAll,
		Name:      "",
	},
}

func (o *MustGatherOptions) run(ctx context.Context) error {
	startTime := time.Now()
	klog.InfoS("Gathering artifacts", "DestDir", o.DestDir)
	defer func() {
		klog.InfoS("Finished gathering artifacts", "Duration", time.Since(startTime))
	}()

	collector := collect.NewCollector(
		o.DestDir,
		o.GetPrinters(),
		o.discoveryClient,
		o.kubeClient.CoreV1(),
		o.dynamicClient,
		true,
		o.KeepGoing,
		o.LogsLimitBytes,
	)

	var resourceSpecs []resourceSpec
	if o.AllResources {
		allPreferredListableResources, err := collector.DiscoverResources(ctx, discovery.SupportsAllVerbs{
			Verbs: []string{"list"},
		}.Match)
		if err != nil {
			return fmt.Errorf("can't discover resources: %w", err)
		}

		// Filter out native resources that share storage across groups.
		preferredListableResources, err := collect.ReplaceIsometricResourceInfosIfPresent(allPreferredListableResources)
		if err != nil {
			return fmt.Errorf("can't replace isometric resourceInfos: %w", err)
		}

		resourceSpecs = make([]resourceSpec, 0, len(preferredListableResources))
		for _, pr := range preferredListableResources {
			resourceSpecs = append(resourceSpecs, resourceSpec{
				ResourceInfo: collect.ResourceInfo{
					Resource: pr.Resource,
					Scope:    pr.Scope,
				},
				Namespace: corev1.NamespaceAll,
				Name:      "",
			})
		}
	} else {
		preferredResources, err := collector.DiscoverResources(ctx, func(groupVersion string, r *metav1.APIResource) bool {
			return true
		})
		if err != nil {
			return fmt.Errorf("can't discover preferred resources: %w", err)
		}

		resourceSpecs = make([]resourceSpec, 0, len(mustGatherSpecs))
		for _, s := range mustGatherSpecs {
			ri, err := findResource(preferredResources, s.GroupResource)
			if err != nil {
				return fmt.Errorf("can't find resource in preferred resources: %w", err)
			}

			namespace := s.Namespace
			if ri.Scope.Name() == meta.RESTScopeNameNamespace &&
				s.Namespace == corev1.NamespaceAll &&
				o.GatherBaseOptions.ConfigFlags.Namespace != nil {
				namespace = *o.GatherBaseOptions.ConfigFlags.Namespace
			}

			resourceSpecs = append(resourceSpecs, resourceSpec{
				ResourceInfo: *ri,
				Namespace:    namespace,
				Name:         s.Name,
			})
		}
	}

	var err error
	var errs []error
	for _, rs := range resourceSpecs {
		if len(rs.Name) != 0 {
			err = collector.CollectResource(ctx, &rs.ResourceInfo, rs.Namespace, rs.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.InfoS("Resource not found", "Resource", rs.ResourceInfo.Resource)
				} else {
					errs = append(errs, fmt.Errorf("can't collect resource %q: %w", rs, err))
				}
			}
		} else {
			err = collector.CollectResources(ctx, &rs.ResourceInfo, rs.Namespace)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't collect resources %q: %w", rs, err))
			}
		}
	}

	return utilerrors.NewAggregate(errs)
}
