// Copyright (C) 2025 ScyllaDB

package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	localhost = "localhost"
)

type ScyllaDBAPIStatusUpdateOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName   string
	PeriodSeconds int

	kubeClient kubernetes.Interface
}

func NewScyllaDBAPIStatusUpdateOptions(streams genericclioptions.IOStreams) *ScyllaDBAPIStatusUpdateOptions {
	return &ScyllaDBAPIStatusUpdateOptions{
		ClientConfig: genericclioptions.NewClientConfig("scylla-operator-scylladb-api-status-update"),
	}
}

func (o *ScyllaDBAPIStatusUpdateOptions) AddFlags(cmd *cobra.Command) {
	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	cmd.Flags().IntVarP(&o.PeriodSeconds, "period-seconds", "", 5, "How often (in seconds) to poll the status.")
}

func NewScyllaDBAPIStatusUpdateCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewScyllaDBAPIStatusUpdateOptions(streams)

	cmd := &cobra.Command{
		Use:   "scylladb-api-status-update",
		Short: "Updates cluster status, as seen by the local node, based on ScyllaDB API status.",
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

func (o *ScyllaDBAPIStatusUpdateOptions) Validate(args []string) error {
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

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *ScyllaDBAPIStatusUpdateOptions) Complete(args []string) error {
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

	return nil
}

func (o *ScyllaDBAPIStatusUpdateOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
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

func (o *ScyllaDBAPIStatusUpdateOptions) Execute(cmdCtx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
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

	c, err := newController(
		o.Namespace,
		o.ServiceName,
		o.kubeClient,
		singleServiceInformer,
	)
	if err != nil {
		return fmt.Errorf("can't create controller: %w", err)
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
		singleServiceKubeInformers.Start(ctx.Done())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Second * time.Duration(o.PeriodSeconds))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				c.Observer.Enqueue()

			}
		}
	}()

	<-cmdCtx.Done()

	return nil
}

type controller struct {
	*controllertools.Observer

	namespace   string
	serviceName string

	kubeClient    kubernetes.Interface
	serviceLister corev1listers.ServiceLister
}

func newController(
	namespace string,
	serviceName string,
	kubeClient kubernetes.Interface,
	serviceInformer corev1informers.ServiceInformer,
) (*controller, error) {
	c := &controller{
		namespace:     namespace,
		serviceName:   serviceName,
		kubeClient:    kubeClient,
		serviceLister: serviceInformer.Lister(),
	}

	observer := controllertools.NewObserver(
		"scylladb-api-status-update",
		kubeClient.CoreV1().Events(corev1.NamespaceAll),
		c.sync,
	)

	observer.AddCachesToSync(serviceInformer.Informer().HasSynced)

	c.Observer = observer

	return c, nil
}

func (c *controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", c.Observer.Name(), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", c.Observer.Name(), "duration", time.Since(startTime))
	}()

	svc, err := c.serviceLister.Services(c.namespace).Get(c.serviceName)
	if err != nil {
		return fmt.Errorf("can't get service %q: %w", naming.ManualRef(c.namespace, c.serviceName), err)
	}

	clusterStatus, err := getClusterStatus(ctx)
	if err != nil {
		return fmt.Errorf("can't get cluster status: %w", err)
	}

	statusAnnotationValue, err := makeClusterStatusAnnotationValue(clusterStatus)
	if err != nil {
		return fmt.Errorf("can't make cluster status annotation value: %w", err)
	}

	patch, err := controllerhelpers.PrepareSetAnnotationPatch(svc, naming.ClusterStatusAnnotation, &statusAnnotationValue)
	if err != nil {
		return fmt.Errorf("can't prepare annotation patch: %w", err)
	}

	_, err = c.kubeClient.CoreV1().Services(c.namespace).Patch(ctx, c.serviceName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch service %q: %w", naming.ObjRef(svc), err)
	}

	return nil
}

func getClusterStatus(ctx context.Context) (*controllerhelpers.ClusterStatus, error) {
	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost()
	if err != nil {
		return nil, fmt.Errorf("can't create Scylla client for localhost: %w", err)
	}
	defer scyllaClient.Close()

	// FIXME: this is copied over from prober, we only need liveness here?
	nodeStatuses, err := scyllaClient.Status(ctx, localhost)
	if err != nil {
		klog.V(4).InfoS("Error getting node statuses", "error", err)
		return &controllerhelpers.ClusterStatus{
			Error: pointer.Ptr(err.Error()),
		}, nil
	}

	hostID, err := scyllaClient.GetLocalHostId(ctx, localhost, false)
	if err != nil {
		klog.V(4).InfoS("Error getting local host ID", "error", err)
		return &controllerhelpers.ClusterStatus{
			Error: pointer.Ptr(err.Error()),
		}, nil
	}

	return &controllerhelpers.ClusterStatus{
		HostID: hostID,
		Status: nodeStatuses,
	}, nil
}

func makeClusterStatusAnnotationValue(status *controllerhelpers.ClusterStatus) (string, error) {
	var err error
	buf := bytes.Buffer{}

	err = json.NewEncoder(&buf).Encode(status)
	if err != nil {
		return "", fmt.Errorf("can't encode status: %w", err)
	}

	return buf.String(), nil
}
