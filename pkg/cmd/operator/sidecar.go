package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"syscall"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	sidecarcontroller "github.com/scylladb/scylla-operator/pkg/controller/sidecar"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/sidecar"
	"github.com/scylladb/scylla-operator/pkg/sidecar/config"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type SidecarOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName                       string
	CPUCount                          int
	ExternalSeeds                     []string
	NodesBroadcastAddressTypeString   string
	ClientsBroadcastAddressTypeString string

	nodesBroadcastAddressType   scyllav1.BroadcastAddressType
	clientsBroadcastAddressType scyllav1.BroadcastAddressType

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewSidecarOptions(streams genericclioptions.IOStreams) *SidecarOptions {
	clientConfig := genericclioptions.NewClientConfig("scylla-sidecar")
	clientConfig.QPS = 2
	clientConfig.Burst = 5

	return &SidecarOptions{
		ClientConfig:        clientConfig,
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewSidecarCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSidecarOptions(streams)

	cmd := &cobra.Command{
		Use:   "sidecar",
		Short: "Run the scylla sidecar.",
		Long:  `Run the scylla sidecar.`,
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
	cmd.Flags().IntVarP(&o.CPUCount, "cpu-count", "", o.CPUCount, "Number of cpus to use.")
	cmd.Flags().StringSliceVar(&o.ExternalSeeds, "external-seeds", o.ExternalSeeds, "The external seeds to propagate to ScyllaDB binary on startup as \"seeds\" parameter of seed-provider.")
	cmd.Flags().StringVarP(&o.NodesBroadcastAddressTypeString, "nodes-broadcast-address-type", "", o.NodesBroadcastAddressTypeString, "Address type that is broadcasted for communication with other nodes.")
	cmd.Flags().StringVarP(&o.ClientsBroadcastAddressTypeString, "clients-broadcast-address-type", "", o.ClientsBroadcastAddressTypeString, "Address type that is broadcasted for communication with clients.")

	return cmd
}

func (o *SidecarOptions) Validate() error {
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

	if !slices.ContainsItem(validation.SupportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)) {
		errs = append(errs, fmt.Errorf("unsupported value of nodes-broadcast-address-type %q, supported ones are: %v", o.NodesBroadcastAddressTypeString, validation.SupportedBroadcastAddressTypes))
	}

	if !slices.ContainsItem(validation.SupportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)) {
		errs = append(errs, fmt.Errorf("unsupported value of clients-broadcast-address-type %q, supported ones are: %v", o.ClientsBroadcastAddressTypeString, validation.SupportedBroadcastAddressTypes))
	}

	return utilerrors.NewAggregate(errs)
}

func (o *SidecarOptions) Complete() error {
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

	o.scyllaClient, err = scyllaversionedclient.NewForConfig(o.RestConfig)
	if err != nil {
		return fmt.Errorf("can't build scylla clientset: %w", err)
	}

	o.clientsBroadcastAddressType = scyllav1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)
	o.nodesBroadcastAddressType = scyllav1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)

	return nil
}

func (o *SidecarOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
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

	namespacedKubeInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 12*time.Hour, informers.WithNamespace(o.Namespace))

	singleServiceInformer := singleServiceKubeInformers.Core().V1().Services()

	prober := sidecar.NewProber(
		o.Namespace,
		o.ServiceName,
		singleServiceInformer.Lister(),
	)

	sc, err := sidecarcontroller.NewController(
		o.Namespace,
		o.ServiceName,
		o.kubeClient,
		singleServiceInformer,
	)
	if err != nil {
		return fmt.Errorf("can't create sidecar controller: %w", err)
	}

	// Start informers.
	singleServiceKubeInformers.Start(ctx.Done())
	namespacedKubeInformers.Start(ctx.Done())

	klog.V(2).InfoS("Waiting for single service informer caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), singleServiceInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	// Wait for the service that holds identity for this scylla node.
	serviceFieldSelector := fields.OneTermEqualSelector("metadata.name", o.ServiceName)
	serviceLW := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = serviceFieldSelector.String()
			return o.kubeClient.CoreV1().Services(o.Namespace).List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = serviceFieldSelector.String()
			return o.kubeClient.CoreV1().Services(o.Namespace).Watch(ctx, options)
		},
	}
	klog.V(2).InfoS("Waiting for Service availability and IP address", "Service", naming.ManualRef(o.Namespace, o.ServiceName))
	event, err := watchtools.UntilWithSync(
		ctx,
		serviceLW,
		&corev1.Service{},
		nil,
		func(e watch.Event) (bool, error) {
			switch t := e.Type; t {
			case watch.Added, watch.Modified:
				service, ok := e.Object.(*corev1.Service)
				if !ok {
					return false, fmt.Errorf("invalid object type in Service watcher, expected *corev1.Service, got %T", e.Object)
				}

				if o.clientsBroadcastAddressType == scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress ||
					o.nodesBroadcastAddressType == scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress {
					if len(service.Status.LoadBalancer.Ingress) == 0 {
						klog.V(4).InfoS("LoadBalancer Service is awaiting for public endpoint")
						return false, nil
					}
				}

				return true, nil
			case watch.Error:
				return true, apierrors.FromObject(e.Object)
			default:
				return true, fmt.Errorf("unexpected event type %v", t)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("can't wait for service %q: %w", naming.ManualRef(o.Namespace, o.ServiceName), err)
	}
	service := event.Object.(*corev1.Service)

	// Wait for this Pod to have ContainerID set.
	podFieldSelector := fields.OneTermEqualSelector("metadata.name", o.ServiceName)
	podLW := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = podFieldSelector.String()
			return o.kubeClient.CoreV1().Pods(o.Namespace).List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = podFieldSelector.String()
			return o.kubeClient.CoreV1().Pods(o.Namespace).Watch(ctx, options)
		},
	}
	klog.V(2).InfoS("Waiting for Pod to have IP address assigned and scylla ContainerID set", "Pod", naming.ManualRef(o.Namespace, o.ServiceName))
	var containerID string
	var pod *corev1.Pod
	_, err = watchtools.UntilWithSync(
		ctx,
		podLW,
		&corev1.Pod{},
		nil,
		func(e watch.Event) (bool, error) {
			switch t := e.Type; t {
			case watch.Added, watch.Modified:
				pod = e.Object.(*corev1.Pod)

				if len(pod.Status.PodIP) == 0 {
					klog.V(4).InfoS("PodIP is not yet set", "Pod", klog.KObj(pod))
					return false, nil
				}

				containerID, err = controllerhelpers.GetScyllaContainerID(pod)
				if err != nil {
					klog.Warningf("can't get scylla container id in pod %q: %v", naming.ObjRef(pod), err)
					return false, nil
				}

				if len(containerID) == 0 {
					klog.V(4).InfoS("ContainerID is not yet set", "Pod", klog.KObj(pod))
					return false, nil
				}

				return true, nil

			case watch.Error:
				return true, apierrors.FromObject(e.Object)

			default:
				return true, fmt.Errorf("unexpected event type %v", t)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("can't wait for pod's ContainerID: %w", err)
	}

	member, err := identity.NewMember(service, pod, o.nodesBroadcastAddressType, o.clientsBroadcastAddressType)
	if err != nil {
		return fmt.Errorf("can't create new member from objects: %w", err)
	}

	labelSelector := labels.Set{
		naming.OwnerUIDLabel:      string(pod.UID),
		naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
	}.AsSelector()
	podLW = &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = labelSelector.String()
			return o.kubeClient.CoreV1().ConfigMaps(pod.Namespace).List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = labelSelector.String()
			return o.kubeClient.CoreV1().ConfigMaps(pod.Namespace).Watch(ctx, options)
		},
	}
	klog.V(2).InfoS("Waiting for NodeConfig's data ConfigMap ", "Selector", labelSelector.String())
	_, err = watchtools.UntilWithSync(
		ctx,
		podLW,
		&corev1.ConfigMap{},
		nil,
		func(e watch.Event) (bool, error) {
			switch t := e.Type; t {
			case watch.Added, watch.Modified:
				cm := e.Object.(*corev1.ConfigMap)

				if cm.Data == nil {
					klog.V(4).InfoS("ConfigMap missing data", "ConfigMap", klog.KObj(cm))
					return false, nil
				}

				srcData, found := cm.Data[naming.ScyllaRuntimeConfigKey]
				if !found {
					klog.V(4).InfoS("ConfigMap is missing key", "ConfigMap", klog.KObj(cm), "Key", naming.ScyllaRuntimeConfigKey)
					return false, nil
				}

				src := &internalapi.SidecarRuntimeConfig{}
				err = json.Unmarshal([]byte(srcData), src)
				if err != nil {
					klog.V(4).ErrorS(err, "Can't unmarshal scylla runtime config", "ConfigMap", klog.KObj(cm), "Key", naming.ScyllaRuntimeConfigKey)
					return false, nil
				}

				if src.ContainerID != containerID {
					klog.V(4).InfoS("Scylla runtime config is not yet updated with our container id",
						"ConfigMap", klog.KObj(cm),
						"ConfigContainerID", src.ContainerID,
						"SidecarContainerID", containerID,
					)
					return false, nil
				}

				if len(src.BlockingNodeConfigs) > 0 {
					klog.V(4).InfoS("Waiting on NodeConfig(s)",
						"ConfigMap", klog.KObj(cm),
						"ContainerID", containerID,
						"NodeConfig", src.BlockingNodeConfigs,
					)
					return false, nil
				}

				klog.V(4).InfoS("ConfigMap container ready", "ConfigMap", klog.KObj(cm), "ContainerID", containerID)
				return true, nil

			case watch.Error:
				return true, apierrors.FromObject(e.Object)

			default:
				return true, fmt.Errorf("unexpected event type %v", t)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("can't wait for optimization: %w", err)
	}

	klog.V(2).InfoS("Starting scylla")

	cfg := config.NewScyllaConfig(member, o.kubeClient, o.scyllaClient, o.CPUCount, o.ExternalSeeds)
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
			klog.Fatalf("ListenAndServe failed: %v", err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		klog.InfoS("Shutting down Prober server")
		defer klog.InfoS("Shut down Prober server")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCtxCancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			klog.ErrorS(err, "Shutting down Prober server")
		}
	}()

	// Run sidecar controller.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc.Run(ctx)
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
