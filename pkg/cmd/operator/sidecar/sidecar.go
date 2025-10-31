// Copyright (C) 2025 ScyllaDB

package sidecar

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	sidecarcontroller "github.com/scylladb/scylla-operator/pkg/controller/sidecar"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/sidecar/config"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type Options struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	statusReporterOptions

	ServiceName                       string
	CPUCount                          int
	ExternalSeeds                     []string
	NodesBroadcastAddressTypeString   string
	ClientsBroadcastAddressTypeString string

	nodesBroadcastAddressType   scyllav1alpha1.BroadcastAddressType
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType

	kubeClient kubernetes.Interface
}

func NewOptions(streams genericclioptions.IOStreams) *Options {
	clientConfig := genericclioptions.NewClientConfig("scylla-sidecar")
	clientConfig.QPS = 2
	clientConfig.Burst = 5

	return &Options{
		ClientConfig:        clientConfig,
		InClusterReflection: genericclioptions.InClusterReflection{},
		statusReporterOptions: statusReporterOptions{
			statusReportInterval: 5 * time.Second,
		},
	}
}

func NewCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewOptions(streams)

	cmd := &cobra.Command{
		Use:   "sidecar",
		Short: "Run the scylla sidecar.",
		Long:  `Run the scylla sidecar.`,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd, args)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)
	o.statusReporterOptions.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	cmd.Flags().IntVarP(&o.CPUCount, "cpu-count", "", o.CPUCount, "Number of cpus to use.")
	cmd.Flags().StringSliceVar(&o.ExternalSeeds, "external-seeds", o.ExternalSeeds, "The external seeds to propagate to ScyllaDB binary on startup as \"seeds\" parameter of seed-provider.")
	cmd.Flags().StringVarP(&o.NodesBroadcastAddressTypeString, "nodes-broadcast-address-type", "", o.NodesBroadcastAddressTypeString, "Address type that is broadcasted for communication with other nodes.")
	cmd.Flags().StringVarP(&o.ClientsBroadcastAddressTypeString, "clients-broadcast-address-type", "", o.ClientsBroadcastAddressTypeString, "Address type that is broadcasted for communication with clients.")

	return cmd
}

func (o *Options) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())
	errs = append(errs, o.statusReporterOptions.Validate())

	if len(o.ServiceName) == 0 {
		errs = append(errs, fmt.Errorf("service-name can't be empty"))
	} else {
		serviceNameValidationErrs := apimachineryvalidation.NameIsDNS1035Label(o.ServiceName, false)
		if len(serviceNameValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid service name %q: %v", o.ServiceName, serviceNameValidationErrs))
		}
	}

	if !oslices.ContainsItem(validation.SupportedScyllaV1Alpha1BroadcastAddressTypes, scyllav1alpha1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)) {
		errs = append(errs, fmt.Errorf("unsupported value of nodes-broadcast-address-type %q, supported ones are: %v", o.NodesBroadcastAddressTypeString, validation.SupportedScyllaV1Alpha1BroadcastAddressTypes))
	}

	if !oslices.ContainsItem(validation.SupportedScyllaV1Alpha1BroadcastAddressTypes, scyllav1alpha1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)) {
		errs = append(errs, fmt.Errorf("unsupported value of clients-broadcast-address-type %q, supported ones are: %v", o.ClientsBroadcastAddressTypeString, validation.SupportedScyllaV1Alpha1BroadcastAddressTypes))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *Options) Complete() error {
	err := o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.InClusterReflection.Complete()
	if err != nil {
		return err
	}

	err = o.statusReporterOptions.Complete()
	if err != nil {
		return err
	}

	o.kubeClient, err = kubernetes.NewForConfig(o.ProtoConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.clientsBroadcastAddressType = scyllav1alpha1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)
	o.nodesBroadcastAddressType = scyllav1alpha1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)

	return nil
}

func (o *Options) Run(streams genericclioptions.IOStreams, cmd *cobra.Command, args []string) error {
	cmdutil.LogCommandStarting(cmd)
	cliflag.PrintFlags(cmd.Flags())
	for _, arg := range args {
		klog.V(1).Infof("ARG: %q", arg)
	}

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

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

	namespacedKubeInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 12*time.Hour, informers.WithNamespace(o.Namespace))

	singleServiceInformer := identityKubeInformers.Core().V1().Services()

	sc, err := sidecarcontroller.NewController(
		o.Namespace,
		o.ServiceName,
		o.kubeClient,
		singleServiceInformer,
	)
	if err != nil {
		return fmt.Errorf("can't create sidecar controller: %w", err)
	}

	sr, err := NewStatusReporter(
		o.Namespace,
		o.ServiceName,
		o.statusReportInterval,
		o.kubeClient,
		identityKubeInformers.Core().V1().Pods(),
	)
	if err != nil {
		return fmt.Errorf("can't create status reporter: %w", err)
	}

	// Start informers.
	identityKubeInformers.Start(ctx.Done())
	namespacedKubeInformers.Start(ctx.Done())

	klog.V(2).InfoS("Waiting for single service informer caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), singleServiceInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	service, err := singleServiceInformer.Lister().Services(o.Namespace).Get(o.ServiceName)
	if err != nil {
		return fmt.Errorf("can't get service %q: %w", o.ServiceName, err)
	}

	pod, err := o.kubeClient.CoreV1().Pods(o.Namespace).Get(ctx, o.ServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get pod %q: %w", o.ServiceName, err)
	}

	member, err := identity.NewMember(service, pod, o.nodesBroadcastAddressType, o.clientsBroadcastAddressType, args)
	if err != nil {
		return fmt.Errorf("can't create new member from objects: %w", err)
	}

	klog.V(2).InfoS("Starting scylla")

	cfg := config.NewScyllaConfig(member, o.kubeClient, o.CPUCount, o.ExternalSeeds)
	scyllaCmd, err := cfg.Setup(ctx)
	if err != nil {
		return fmt.Errorf("can't set up scylla: %w", err)
	}
	// Make sure to propagate the signal if we die.
	scyllaCmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// Run sidecar controller.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc.Run(ctx)
	}()

	// Run status reporter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sr.Run(ctx)
	}()

	// Run scylla in a new process.
	err = scyllaCmd.Start()
	if err != nil {
		return fmt.Errorf("can't start scylla: %w", err)
	}

	defer func() {
		klog.InfoS("Waiting for scylla process to finish")
		defer klog.InfoS("Scylla process finished")

		err := scyllaCmd.Wait()
		if err != nil {
			klog.ErrorS(err, "Can't wait for scylla process to finish")
		}
	}()

	// Terminate the scylla process.
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		klog.InfoS("Sending SIGTERM to the scylla process")
		err := scyllaCmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			klog.ErrorS(err, "Can't send SIGTERM to the scylla process")
			return
		}
		klog.InfoS("Sent SIGTERM to the scylla process")
	}()

	<-ctx.Done()

	return nil
}
