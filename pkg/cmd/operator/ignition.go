package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/cmd/operator/probeserver"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
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
	nodesBroadcastAddressType   scyllav1.BroadcastAddressType
	clientsBroadcastAddressType scyllav1.BroadcastAddressType

	ignited atomic.Bool
}

func NewIgnitionOptions(streams genericclioptions.IOStreams) *IgnitionOptions {
	mux := http.NewServeMux()

	return &IgnitionOptions{
		ServeProbesOptions: *probeserver.NewServeProbesOptions(streams, naming.ScyllaDBIgnitionProbePort, mux),
		ClientConfig:       genericclioptions.NewClientConfig("scylla-operator-ignition"),
		mux:                mux,
		ignited:            atomic.Bool{},
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

	if !slices.ContainsItem(validation.SupportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)) {
		errs = append(errs, fmt.Errorf("unsupported value of nodes-broadcast-address-type %q, supported ones are: %v", o.NodesBroadcastAddressTypeString, validation.SupportedBroadcastAddressTypes))
	}

	if !slices.ContainsItem(validation.SupportedBroadcastAddressTypes, scyllav1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)) {
		errs = append(errs, fmt.Errorf("unsupported value of clients-broadcast-address-type %q, supported ones are: %v", o.ClientsBroadcastAddressTypeString, validation.SupportedBroadcastAddressTypes))
	}

	return utilerrors.NewAggregate(errs)
}

func (o *IgnitionOptions) Complete(args []string) error {
	err := o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	klog.InfoS("checkpoint===", "proto ratelimitter", o.ProtoConfig.RateLimiter)
	klog.InfoS("checkpoint===", "rest ratelimitter", o.RestConfig.RateLimiter)

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

	o.nodesBroadcastAddressType = scyllav1.BroadcastAddressType(o.NodesBroadcastAddressTypeString)
	o.clientsBroadcastAddressType = scyllav1.BroadcastAddressType(o.ClientsBroadcastAddressTypeString)

	return nil
}

func (o *IgnitionOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
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

func (o *IgnitionOptions) Execute(ctx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
	startTime := time.Now()

	readyzFunc := func(w http.ResponseWriter, r *http.Request) {
		ignited := o.ignited.Load()
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
	}()

	probesCtx, probesCtxCancel := context.WithCancel(ctx)
	defer probesCtxCancel()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := o.ServeProbesOptions.Execute(probesCtx, originalStreams, cmd)
		if err != nil {
			klog.ErrorS(err, "Error running probe server")
		}
	}()

	// Wait for the service that holds identity for this scylla node.
	klog.InfoS("Waiting for Service availability and IP address", "Service", naming.ManualRef(o.Namespace, o.ServiceName))
	service, err := controllerhelpers.WaitForServiceState(
		ctx,
		o.kubeClient.CoreV1().Services(o.Namespace),
		o.ServiceName,
		controllerhelpers.WaitForStateOptions{},
		func(service *corev1.Service) (bool, error) {
			if o.clientsBroadcastAddressType == scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress ||
				o.nodesBroadcastAddressType == scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress {
				if len(service.Status.LoadBalancer.Ingress) == 0 {
					klog.V(4).InfoS("LoadBalancer Service is awaiting for public endpoint")
					return false, nil
				}
			}

			return true, nil
		},
	)
	if err != nil {
		return fmt.Errorf("can't wait for service %q: %w", naming.ManualRef(o.Namespace, o.ServiceName), err)
	}
	klog.InfoS(
		"Service is available and has an IP address",
		"Service", naming.ManualRef(service.Namespace, service.Name),
		"UID", service.UID,
	)

	// Wait for this Pod to have ContainerID set.
	klog.InfoS("Waiting for Pod to have IP address assigned and scylla ContainerID set",
		"Pod", naming.ManualRef(o.Namespace, o.ServiceName),
	)
	var containerID string
	pod, err := controllerhelpers.WaitForPodState(
		ctx,
		o.kubeClient.CoreV1().Pods(o.Namespace),
		o.ServiceName,
		controllerhelpers.WaitForStateOptions{},
		func(pod *corev1.Pod) (bool, error) {
			if len(pod.Status.PodIP) == 0 {
				klog.V(2).InfoS("PodIP is not yet set", "Pod", klog.KObj(pod))
				return false, nil
			}

			containerID, err = controllerhelpers.GetScyllaContainerID(pod)
			if err != nil {
				klog.Warningf("can't get scylla container id in pod %q: %v", naming.ObjRef(pod), err)
				return false, nil
			}

			if len(containerID) == 0 {
				klog.V(2).InfoS("ContainerID is not yet set", "Pod", klog.KObj(pod))
				return false, nil
			}

			return true, nil
		},
	)
	if err != nil {
		return fmt.Errorf("can't wait for pod's ContainerID: %w", err)
	}
	klog.InfoS("Pod has an IP address assigned and scylla ContainerID set",
		"Pod", naming.ManualRef(pod.Namespace, pod.Name),
		"UID", pod.UID,
	)

	labelSelector := labels.Set{
		naming.OwnerUIDLabel:      string(pod.UID),
		naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
	}.AsSelector()
	podLW := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = labelSelector.String()
			return o.kubeClient.CoreV1().ConfigMaps(pod.Namespace).List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = labelSelector.String()
			return o.kubeClient.CoreV1().ConfigMaps(pod.Namespace).Watch(ctx, options)
		},
	}
	klog.InfoS("Waiting for NodeConfig's data ConfigMap",
		"Selector", labelSelector.String(),
	)
	cmEvent, err := watchtools.UntilWithSync(
		ctx,
		podLW,
		&corev1.ConfigMap{},
		nil,
		func(e watch.Event) (bool, error) {
			switch t := e.Type; t {
			case watch.Added, watch.Modified:
				cm := e.Object.(*corev1.ConfigMap)

				if cm.Data == nil {
					klog.V(2).InfoS("ConfigMap missing data", "ConfigMap", klog.KObj(cm))
					return false, nil
				}

				srcData, found := cm.Data[naming.ScyllaRuntimeConfigKey]
				if !found {
					klog.V(2).InfoS("ConfigMap is missing key", "ConfigMap", klog.KObj(cm), "Key", naming.ScyllaRuntimeConfigKey)
					return false, nil
				}

				src := &internalapi.SidecarRuntimeConfig{}
				err = json.Unmarshal([]byte(srcData), src)
				if err != nil {
					klog.ErrorS(err, "Can't unmarshal scylla runtime config", "ConfigMap", klog.KObj(cm), "Key", naming.ScyllaRuntimeConfigKey)
					return false, nil
				}

				if src.ContainerID != containerID {
					klog.V(2).InfoS("Scylla runtime config is not yet updated with our container id",
						"ConfigMap", klog.KObj(cm),
						"ConfigContainerID", src.ContainerID,
						"SidecarContainerID", containerID,
					)
					return false, nil
				}

				if len(src.BlockingNodeConfigs) > 0 {
					klog.V(2).InfoS("Waiting on NodeConfig(s)",
						"ConfigMap", klog.KObj(cm),
						"ContainerID", containerID,
						"NodeConfig", src.BlockingNodeConfigs,
					)
					return false, nil
				}

				klog.V(2).InfoS("ConfigMap container ready", "ConfigMap", klog.KObj(cm), "ContainerID", containerID)
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

	cm, ok := cmEvent.Object.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("internal error: unexpected object type %T", cmEvent.Object)
	}
	klog.InfoS("NodeConfig's data ConfigMap is available",
		"ConfigMap", naming.ManualRef(cm.Namespace, cm.Name),
		"UID", cm.UID,
	)

	err = helpers.TouchFile(naming.ScyllaDBIgnitionDonePath)
	if err != nil {
		return fmt.Errorf("can't touch signal file %q: %w", naming.ScyllaDBIgnitionDonePath, err)
	}

	klog.InfoS("Ignition successful",
		"Duration", time.Now().Sub(startTime),
		"SignalFile", naming.ScyllaDBIgnitionDonePath,
	)

	// Update the probes.
	o.ignited.Store(true)

	klog.InfoS("Continuing serving probes")

	<-ctx.Done()

	return nil
}
