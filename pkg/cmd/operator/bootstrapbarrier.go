// Copyright (C) 2025 ScyllaDB

package operator

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/bootstrapbarrier"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type BootstrapBarrierOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName        string
	SelectorLabelValue string

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewBootstrapBarrierOptions(streams genericclioptions.IOStreams) *BootstrapBarrierOptions {
	return &BootstrapBarrierOptions{
		ClientConfig: genericclioptions.NewClientConfig("bootstrap-barrier"),
	}
}

func (o *BootstrapBarrierOptions) AddFlags(cmd *cobra.Command) {
	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	// TODO
	cmd.Flags().StringVarP(&o.SelectorLabelValue, "selector-label-value", "", o.SelectorLabelValue, "")
}

func NewBootstrapBarrierCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewBootstrapBarrierOptions(streams)

	cmd := &cobra.Command{
		Use: "run-bootstrap-barrier",
		// TODO
		Short: "",
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

	o.AddFlags(cmd)

	return cmd
}

func (o *BootstrapBarrierOptions) Validate(args []string) error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())

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
		// TODO: verify against actual requirements for a label value
		selectorLabelValueValidationErrs := apimachineryvalidation.NameIsDNSSubdomain(o.SelectorLabelValue, false)
		if len(selectorLabelValueValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid selector label value %q: %v", o.SelectorLabelValue, selectorLabelValueValidationErrs))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *BootstrapBarrierOptions) Complete(args []string) error {
	var err error

	err = o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.InClusterReflection.Complete()
	if err != nil {
		return err
	}

	o.kubeClient, err = kubernetes.NewForConfig(o.ProtoConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.scyllaClient, err = scyllaversionedclient.NewForConfig(o.RestConfig)
	if err != nil {
		return fmt.Errorf("can't build scylla clientset: %w", err)
	}

	return nil
}

func (o *BootstrapBarrierOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.Execute(ctx, originalStreams, cmd)
}

func (o *BootstrapBarrierOptions) Execute(cmdCtx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	identityKubeInformers := informers.NewSharedInformerFactoryWithOptions(
		o.kubeClient,
		12*time.Hour,
		informers.WithNamespace(o.Namespace),
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("metadata.name", o.ServiceName).String()
			},
		),
	)

	scyllaDBDatacenterNodesStatusReportKubeInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		o.scyllaClient,
		12*time.Hour,
		scyllainformers.WithNamespace(o.Namespace),
		scyllainformers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.Set{
					naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: o.SelectorLabelValue,
				}.String()
			},
		),
	)

	boostrapPreconditionCh := make(chan struct{})
	boostrapBarrierController, err := bootstrapbarrier.NewController(
		o.Namespace,
		o.ServiceName,
		o.SelectorLabelValue,
		boostrapPreconditionCh,
		o.kubeClient,
		identityKubeInformers.Core().V1().Services(),
		scyllaDBDatacenterNodesStatusReportKubeInformers.Scylla().V1alpha1().ScyllaDBDatacenterNodesStatusReports(),
	)
	if err != nil {
		return fmt.Errorf("can't create bootstrap barrier controller: %w", err)
	}

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Waiting for background tasks to finish")
		wg.Wait()
		klog.InfoS("Background tasks have finished")
	}()

	ctx, taskCtxCancel := context.WithCancel(cmdCtx)
	defer taskCtxCancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		identityKubeInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		scyllaDBDatacenterNodesStatusReportKubeInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		boostrapBarrierController.Run(ctx)
	}()

	select {
	case <-cmdCtx.Done():
		return fmt.Errorf("stopped before bootstrap barrier precondition was met: %w", cmdCtx.Err())

	case <-boostrapPreconditionCh:
		return nil

	}
}
