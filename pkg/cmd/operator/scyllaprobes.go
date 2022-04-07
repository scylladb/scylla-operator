package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaprobes"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type ScyllaProbesOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName string

	kubeClient kubernetes.Interface
}

func NewScyllaProbesOptions(streams genericclioptions.IOStreams) *ScyllaProbesOptions {
	clientConfig := genericclioptions.NewClientConfig("scylla-probes")
	clientConfig.QPS = 2
	clientConfig.Burst = 5

	return &ScyllaProbesOptions{
		ClientConfig:        clientConfig,
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewScyllaProbesCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewScyllaProbesOptions(streams)

	cmd := &cobra.Command{
		Use:   "serve-scylla-probes",
		Short: "Run a server that translates standard probes for scylla.",
		Long:  `Run a server that translates standard probes for scylla`,
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

	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")

	return cmd
}

func (o *ScyllaProbesOptions) Validate() error {
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

	return utilerrors.NewAggregate(errs)
}

func (o *ScyllaProbesOptions) Complete() error {
	err := o.ClientConfig.Complete()
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

	return nil
}

func (o *ScyllaProbesOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	singleServiceKubeInformers := informers.NewSharedInformerFactoryWithOptions(
		o.kubeClient,
		12*time.Hour,
		informers.WithNamespace(o.Namespace),
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("metadata.name", o.ServiceName).String()
			},
		),
	)

	singleServiceInformer := singleServiceKubeInformers.Core().V1().Services()

	prober := scyllaprobes.NewProber(
		o.Namespace,
		o.ServiceName,
		singleServiceInformer.Lister(),
	)

	// Start informers.
	singleServiceKubeInformers.Start(ctx.Done())

	klog.V(2).InfoS("Waiting for single service informer caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), singleServiceInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// Run probes.
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", naming.ProbePort),
		Handler: nil,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()

		ok := cache.WaitForNamedCacheSync("Prober", ctx.Done(), singleServiceInformer.Informer().HasSynced)
		if !ok {
			return
		}

		klog.InfoS("Starting Prober server")
		defer klog.InfoS("Prober server shut down")

		http.HandleFunc(naming.LivenessProbePath, prober.Healthz)
		http.HandleFunc(naming.ReadinessProbePath, prober.Readyz)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatal("ListenAndServe failed: %v", err)
		}
	}()

	<-ctx.Done()

	klog.InfoS("Shutting down Prober server")
	defer klog.InfoS("Shut down Prober server")

	shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCtxCancel()
	err := server.Shutdown(shutdownCtx)
	if err != nil {
		klog.ErrorS(err, "Shutting down Prober server")
	}

	return nil
}
