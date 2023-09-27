package operator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	kgenericclioptions "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	gatherLongDescription = templates.LongDesc(`
		gather collects resource dumps.
	
		This command is experimental and subject to change without notice.
	`)

	gatherExample = templates.Examples(`
		# Collect deployments and pods in the current namespace
		scylla-operator gather deployments.apps,pods --all

		# Collect pods in all namespaces
		scylla-operator gather pods --all-namespaces

		# Collect single pod
		scylla-operator gather pod/foo
	`)
)

type GatherOptions struct {
	*GatherBaseOptions

	builderFlags *kgenericclioptions.ResourceBuilderFlags
	builder      kgenericclioptions.ResourceFinder

	CollectRelatedResources bool
}

func NewGatherOptions(streams genericclioptions.IOStreams) *GatherOptions {
	return &GatherOptions{
		GatherBaseOptions: NewGatherBaseOptions("scylla-operator-gather", false),
		builderFlags: (&kgenericclioptions.ResourceBuilderFlags{}).
			WithLabelSelector("").
			WithFieldSelector("").
			WithAll(false).
			WithAllNamespaces(false).
			WithLocal(false).
			WithScheme(nil).
			WithLatest(),
		CollectRelatedResources: true,
	}
}

func (o *GatherOptions) AddFlags(flagset *pflag.FlagSet) {
	o.GatherBaseOptions.AddFlags(flagset)
	o.builderFlags.AddFlags(flagset)

	flagset.BoolVarP(&o.CollectRelatedResources, "related", "", o.CollectRelatedResources, "Collects additional resources related to the resource(s) requested.")
}

func NewGatherCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGatherOptions(streams)

	cmd := &cobra.Command{
		Use:     "gather kind[.version].group [...] | kind[.version].group/name",
		Short:   "Run the scylla gather.",
		Long:    gatherLongDescription,
		Example: gatherExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate(args)
			if err != nil {
				return err
			}

			err = o.Complete(args)
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

func (o *GatherOptions) Validate(args []string) error {
	var errs []error

	errs = append(errs, o.GatherBaseOptions.Validate())

	if len(args) == 0 {
		return errors.New("at least one resource argument needs to be specified")
	}

	return apierrors.NewAggregate(errs)
}

func (o *GatherOptions) Complete(args []string) error {
	err := o.GatherBaseOptions.Complete()
	if err != nil {
		return fmt.Errorf("can't complete gather base: %w", err)
	}

	o.builder = o.builderFlags.ToBuilder(o.configFlags, args)

	return nil
}

func (o *GatherOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
	err := o.GatherBaseOptions.RunInit(originalStreams, cmd)
	if err != nil {
		return fmt.Errorf("can't initialize gather base: %w", err)
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

func (o *GatherOptions) run(ctx context.Context) error {
	collector := collect.NewCollector(
		o.DestDir,
		o.GetPrinters(),
		o.discoveryClient,
		o.kubeClient.CoreV1(),
		o.dynamicClient,
		o.CollectRelatedResources,
		o.KeepGoing,
		o.LogsLimitBytes,
	)

	startTime := time.Now()
	klog.InfoS("Gathering artifacts", "DestDir", o.DestDir)
	defer func() {
		klog.InfoS("Finished gathering artifacts", "Duration", time.Since(startTime))
	}()
	visitor := o.builder.Do()
	err := visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		u, ok := info.Object.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type %T", info.Object)
		}

		err = collector.CollectObject(ctx, u, collect.NewResourceInfoFromMapping(info.Mapping))
		if err != nil {
			return fmt.Errorf("can't collect object %q: %w", naming.ObjRef(u), err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("can't visit object: %w", err)
	}

	return nil
}
