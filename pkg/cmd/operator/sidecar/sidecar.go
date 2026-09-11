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
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/sidecar/config"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	IPFamilyString                    string

	nodesBroadcastAddressType   scyllav1alpha1.BroadcastAddressType
	scyllaLocalhostAddress      string
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType
	ipFamily                    corev1.IPFamily

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
	cmd.Flags().StringVarP(&o.IPFamilyString, "ip-family", "", string(corev1.IPv4Protocol), "IP family to use for ScyllaDB (IPv4 or IPv6).")

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

	if len(o.IPFamilyString) == 0 {
		errs = append(errs, fmt.Errorf("ip-family can't be empty"))
	} else if o.IPFamilyString != string(corev1.IPv4Protocol) && o.IPFamilyString != string(corev1.IPv6Protocol) {
		errs = append(errs, fmt.Errorf("ip-family must be either %q or %q, got %q", corev1.IPv4Protocol, corev1.IPv6Protocol, o.IPFamilyString))
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
	o.ipFamily = corev1.IPFamily(o.IPFamilyString)

	// Automatically set scyllaLocalhostAddress based on IP family
	switch o.ipFamily {
	case corev1.IPv4Protocol:
		o.scyllaLocalhostAddress = "127.0.0.1"
	case corev1.IPv6Protocol:
		o.scyllaLocalhostAddress = "::1"
	default:
		return fmt.Errorf("unsupported IP family: %s", o.ipFamily)
	}

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

	// The manager's cache is restricted to the member's Service and Pod. It starts first so that the identity can be
	// read from it; the controllers are registered once ScyllaDB is configured, right before it starts, and are
	// started by the running manager then.
	mgr, err := controllermanager.NewManager(
		o.RestConfig,
		klog.NewKlogr(),
		sidecarcontroller.CacheOptions(o.Namespace, o.ServiceName),
		controllermanager.MetricsDisabledBindAddress,
	)
	if err != nil {
		return fmt.Errorf("can't create controller manager: %w", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		err := mgr.Start(ctx)
		if err != nil {
			klog.ErrorS(err, "Controller manager failed")
		}
	}()

	klog.V(2).InfoS("Waiting for caches to sync")
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	service, err := ctrlclient.Get[corev1.Service](ctx, mgr.GetClient(), o.Namespace, o.ServiceName)
	if err != nil {
		return fmt.Errorf("can't get service %q: %w", o.ServiceName, err)
	}

	pod, err := o.kubeClient.CoreV1().Pods(o.Namespace).Get(ctx, o.ServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get pod %q: %w", o.ServiceName, err)
	}

	member, err := identity.NewMember(service, pod, o.nodesBroadcastAddressType, o.clientsBroadcastAddressType, o.ipFamily, args)
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

	newScyllaClient := func() (*scyllaclient.Client, error) {
		return controllerhelpers.NewScyllaClientForLocalhost(o.ipFamily)
	}

	// Run sidecar controller.
	sc, err := sidecarcontroller.NewController(
		o.Namespace,
		o.ServiceName,
		o.scyllaLocalhostAddress,
		mgr.GetClient(),
		newScyllaClient,
	)
	if err != nil {
		return fmt.Errorf("can't create sidecar controller: %w", err)
	}
	err = sc.SetupWithManager(mgr, sidecarcontroller.ControllerOptions())
	if err != nil {
		return fmt.Errorf("can't set up sidecar controller: %w", err)
	}

	// Run status reporter.
	sr := NewStatusReporter(
		o.Namespace,
		o.ServiceName,
		o.statusReportInterval,
		mgr.GetClient(),
		newScyllaClient,
	)
	err = sr.SetupWithManager(mgr, controller.Options{
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return fmt.Errorf("can't set up status reporter: %w", err)
	}

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
