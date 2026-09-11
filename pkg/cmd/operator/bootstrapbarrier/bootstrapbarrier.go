// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"context"
	"fmt"
	"sync"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controller/bootstrapbarrier"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// Options holds the options for running the bootstrap barrier controller.
// Due to the dependency on scyllaSSTableBootstrappedQueryOptions, this command depends on the availability of `scylla sstable query` command.
type Options struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	scyllaSSTableBootstrappedQueryOptions

	ServiceName                          string
	SelectorLabelValue                   string
	SingleReportAllowNonReportingHostIDs bool
}

func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{
		ClientConfig: genericclioptions.NewClientConfig("bootstrap-barrier"),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)
	o.scyllaSSTableBootstrappedQueryOptions.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	cmd.Flags().StringVarP(&o.SelectorLabelValue, "selector-label-value", "", o.SelectorLabelValue, "Value of the selector label used to select ScyllaDBDatacenterNodesStatusReports to use for the precondition evaluation.")
	cmd.Flags().BoolVarP(&o.SingleReportAllowNonReportingHostIDs, "single-report-allow-non-reporting-host-ids", "", o.SingleReportAllowNonReportingHostIDs, "Specifies whether non-reporting HostIDs are allowed when a single ScyllaDBDatacenterNodesStatusReport is used for precondition evaluation.")
}

func NewCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:   "run-bootstrap-barrier",
		Short: "Runs a bootstrap barrier controller that waits for preconditions to be met before allowing bootstrapping to proceed.",
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

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *Options) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())
	errs = append(errs, o.scyllaSSTableBootstrappedQueryOptions.Validate())

	if len(o.ServiceName) == 0 {
		errs = append(errs, fmt.Errorf("service-name can't be empty"))
	} else {
		serviceNameValidationErrs := apimachineryvalidation.NameIsDNS1035Label(o.ServiceName, false)
		if len(serviceNameValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid service name %q: %v", o.ServiceName, serviceNameValidationErrs))
		}
	}

	if len(o.SelectorLabelValue) == 0 {
		errs = append(errs, fmt.Errorf("selector-label-value can't be empty"))
	} else {
		selectorLabelValueValidationErrs := apimachineryutilvalidation.IsValidLabelValue(o.SelectorLabelValue)
		if len(selectorLabelValueValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid selector label value %q: %v", o.SelectorLabelValue, selectorLabelValueValidationErrs))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *Options) Complete() error {
	var err error

	err = o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.InClusterReflection.Complete()
	if err != nil {
		return err
	}

	err = o.scyllaSSTableBootstrappedQueryOptions.Complete()
	if err != nil {
		return err
	}

	return nil
}

func (o *Options) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	cmdutil.LogCommandStarting(cmd)
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	bootstrapped, err := o.IsBootstrapped(ctx)
	if err != nil {
		return fmt.Errorf("can't check whether node has been bootstrapped: %w", err)
	}

	if bootstrapped {
		klog.V(2).InfoS("Node has already been bootstrapped, skipping the bootstrap barrier.", "Service", naming.ManualRef(o.Namespace, o.ServiceName))
		return nil
	}

	klog.V(2).InfoS("Node has not been bootstrapped yet, running the bootstrap barrier.", "Service", naming.ManualRef(o.Namespace, o.ServiceName))
	return o.Execute(ctx, originalStreams, cmd)
}

func (o *Options) Execute(cmdCtx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	mgr, err := controllermanager.NewManager(
		o.RestConfig,
		klog.NewKlogr(),
		bootstrapbarrier.CacheOptions(o.Namespace, o.ServiceName, o.SelectorLabelValue),
		controllermanager.MetricsDisabledBindAddress,
	)
	if err != nil {
		return fmt.Errorf("can't create controller manager: %w", err)
	}

	boostrapPreconditionCh := make(chan struct{})
	boostrapBarrierController := bootstrapbarrier.NewController(
		o.Namespace,
		o.ServiceName,
		o.SelectorLabelValue,
		o.SingleReportAllowNonReportingHostIDs,
		boostrapPreconditionCh,
		mgr.GetClient(),
	)
	err = boostrapBarrierController.SetupWithManager(mgr, controller.Options{
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return fmt.Errorf("can't set up bootstrap barrier controller: %w", err)
	}

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Waiting for background tasks to finish")
		wg.Wait()
		klog.InfoS("Background tasks have finished")
	}()

	ctx, taskCtxCancel := context.WithCancel(cmdCtx)
	defer taskCtxCancel()

	mgrErrCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgrErrCh <- mgr.Start(ctx)
	}()

	select {
	case <-cmdCtx.Done():
		return fmt.Errorf("stopped before bootstrap barrier precondition was met: %w", cmdCtx.Err())

	case err := <-mgrErrCh:
		return fmt.Errorf("controller manager stopped before bootstrap barrier precondition was met: %w", err)

	case <-boostrapPreconditionCh:
		return nil

	}
}
