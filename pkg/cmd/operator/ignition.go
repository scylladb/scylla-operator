package operator

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/cmd/operator/probeserver"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controller/ignition"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
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

type IgnitionOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection
	probeserver.ServeProbesOptions

	ServiceName                       string
	NodesBroadcastAddressTypeString   string
	ClientsBroadcastAddressTypeString string

	kubeClient                  kubernetes.Interface
	mux                         *http.ServeMux
	nodesBroadcastAddressType   scyllav1alpha1.BroadcastAddressType
	clientsBroadcastAddressType scyllav1alpha1.BroadcastAddressType
}

func NewIgnitionOptions(streams genericclioptions.IOStreams) *IgnitionOptions {
	mux := http.NewServeMux()

	return &IgnitionOptions{
		ServeProbesOptions: *probeserver.NewServeProbesOptions(streams, naming.ScyllaDBIgnitionProbePort, mux),
		ClientConfig:       genericclioptions.NewClientConfig("scylla-operator-ignition"),
		mux:                mux,
	}
}

func (o *IgnitionOptions) AddFlags(cmd *cobra.Command) {
	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)
	o.ServeProbesOptions.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	cmd.Flags().StringVarP(&o.NodesBroadcastAddressTypeString, "nodes-broadcast-address-type", "", o.NodesBroadcastAddressTypeString, "Address type that is broadcasted for communication with other nodes.")
	cmd.Flags().StringVarP(&o.ClientsBroadcastAddressTypeString, "clients-broadcast-address-type", "", o.ClientsBroadcastAddressTypeString, "Address type that is broadcasted for communication with clients.")

}

func NewIgnitionCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewIgnitionOptions(streams)

	cmd := &cobra.Command{
		Use:   "run-ignition",
		Short: "Bootstraps necessary configurations and waits for tuning to finish.",
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
		Hidden:        true,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *IgnitionOptions) Validate(args []string) error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())
	errs = append(errs, o.ServeProbesOptions.Validate(args))

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

func (o *IgnitionOptions) Complete(args []string) error {
	err := o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.InClusterReflection.Complete()
	if err != nil {
		return err
	}

	err = o.ServeProbesOptions.Complete(args)
	if err != nil {
		return err
	}

	o.kubeClient, err = kubernetes.NewForConfig(o.ProtoConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.nodesBroadcastAddressType = scyllav1alpha1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)
	o.clientsBroadcastAddressType = scyllav1alpha1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)

	return nil
}

func (o *IgnitionOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
	cmdutil.LogCommandStarting(cmd)
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

func (o *IgnitionOptions) Execute(cmdCtx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
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
	nodeconfigDataCMKubeInformers := informers.NewSharedInformerFactoryWithOptions(
		o.kubeClient,
		12*time.Hour,
		informers.WithNamespace(o.Namespace),
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.Set{
					naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
				}.String()
			},
		),
	)

	ignitionController, err := ignition.NewController(
		o.Namespace,
		o.ServiceName,
		o.nodesBroadcastAddressType,
		o.nodesBroadcastAddressType,
		o.kubeClient,
		nodeconfigDataCMKubeInformers.Core().V1().ConfigMaps(),
		identityKubeInformers.Core().V1().Services(),
		identityKubeInformers.Core().V1().Pods(),
	)
	if err != nil {
		return fmt.Errorf("can't create ignition controller: %w", err)
	}

	readyzFunc := func(w http.ResponseWriter, r *http.Request) {
		ignited := ignitionController.IsIgnited()
		klog.V(3).InfoS("Probe call", "URI", r.RequestURI, "Ignited", ignited)
		if !ignited {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		return
	}
	o.mux.HandleFunc(naming.ReadinessProbePath, readyzFunc)

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
		nodeconfigDataCMKubeInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := o.ServeProbesOptions.Execute(ctx, originalStreams, cmd)
		if err != nil {
			klog.ErrorS(err, "Error running probe server")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ignitionController.Run(ctx)
	}()

	<-cmdCtx.Done()

	return nil
}
