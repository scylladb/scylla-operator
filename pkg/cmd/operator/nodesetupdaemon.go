// Copyright (c) 2023 ScyllaDB.

package operator

import (
	"context"
	"fmt"
	"sync"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/nodesetup"
	"github.com/scylladb/scylla-operator/pkg/controller/nodetune"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type NodeSetupDaemonOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	PodName              string
	NodeName             string
	NodeConfigName       string
	NodeConfigUID        string
	ScyllaImage          string
	DisableOptimizations bool

	CRIEndpoints []string

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewNodeSetupOptions(streams genericclioptions.IOStreams) *NodeSetupDaemonOptions {
	return &NodeSetupDaemonOptions{
		ClientConfig:        genericclioptions.NewClientConfig("node-setup"),
		InClusterReflection: genericclioptions.InClusterReflection{},
		CRIEndpoints: []string{
			"unix:///var/run/dockershim.sock",
			"unix:///run/containerd/containerd.sock",
			"unix:///run/crio/crio.sock",
		},
	}
}

func NewNodeSetupCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeSetupOptions(streams)

	cmd := &cobra.Command{
		Use:   "node-setup-daemon",
		Short: "Runs a controller that configures this machine.",
		Long:  "Runs a controller that configures this machine.",
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

	cmd.Flags().StringVarP(&o.PodName, "pod-name", "", o.PodName, "Name of the pod this container this running in.")
	cmd.Flags().StringVarP(&o.NodeName, "node-name", "", o.NodeName, "Name of the node where this Pod is running.")
	cmd.Flags().StringVarP(&o.NodeConfigName, "node-config-name", "", o.NodeConfigName, "Name of the NodeConfig that owns this subcontroller.")
	cmd.Flags().StringVarP(&o.NodeConfigUID, "node-config-uid", "", o.NodeConfigUID, "UID of the NodeConfig that owns this subcontroller.")
	cmd.Flags().StringVarP(&o.ScyllaImage, "scylla-image", "", o.ScyllaImage, "Scylla image used for running perftune.")
	cmd.Flags().BoolVarP(&o.DisableOptimizations, "disable-optimizations", "", o.DisableOptimizations, "Controls if optimizations are disabled")
	cmd.Flags().StringArrayVarP(&o.CRIEndpoints, "cri-endpoint", "", o.CRIEndpoints, "CRI endpoint to connect to. It will try to connect to any of them, in the given order.")

	return cmd
}

func (o *NodeSetupDaemonOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())

	if len(o.PodName) == 0 {
		errs = append(errs, fmt.Errorf("pod-name can't be empty"))
	}

	if len(o.NodeName) == 0 {
		errs = append(errs, fmt.Errorf("node-name can't be empty"))
	}

	if len(o.NodeConfigName) == 0 {
		errs = append(errs, fmt.Errorf("node-config-name can't be empty"))
	}

	if len(o.NodeConfigUID) == 0 {
		errs = append(errs, fmt.Errorf("node-config-uid can't be empty"))
	}

	if len(o.ScyllaImage) == 0 {
		errs = append(errs, fmt.Errorf("scylla-image can't be empty"))
	}

	if len(o.CRIEndpoints) == 0 {
		errs = append(errs, fmt.Errorf("there must be at least one cri-endpoint"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *NodeSetupDaemonOptions) Complete() error {
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

	return nil
}

func (o *NodeSetupDaemonOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	criClient, err := cri.NewClient(ctx, o.CRIEndpoints)
	if err != nil {
		return fmt.Errorf("can't create cri client: %w", err)
	}

	scyllaInformers := scyllainformers.NewSharedInformerFactory(o.scyllaClient, resyncPeriod)
	namespacedKubeInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, resyncPeriod, informers.WithNamespace(o.Namespace))
	localNodeScyllaCoreInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, resyncPeriod, informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.LabelSelector = naming.ScyllaSelector().String()
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", o.NodeName).String()
		},
	))
	selfPodInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, resyncPeriod, informers.WithNamespace(o.Namespace), informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", o.PodName).String()
		},
	))

	var node *corev1.Node
	err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func(fCtx context.Context) (bool, error) {
		node, err = o.kubeClient.CoreV1().Nodes().Get(fCtx, o.NodeName, metav1.GetOptions{})
		if err != nil {
			klog.V(2).InfoS("Can't get Node", "Node", o.NodeName, "Error", err.Error())
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("can't get node %q: %w", o.NodeName, err)
	}

	nsc, err := nodesetup.NewController(
		ctx,
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		scyllaInformers.Scylla().V1alpha1().NodeConfigs(),
		node.Name,
		node.UID,
		o.NodeConfigName,
		types.UID(o.NodeConfigUID),
	)
	if err != nil {
		return fmt.Errorf("can't create node config instance controller: %w", err)
	}
	defer nsc.Close()

	ntc, err := nodetune.NewController(
		o.kubeClient,
		o.scyllaClient,
		criClient,
		scyllaInformers.Scylla().V1alpha1().NodeConfigs(),
		localNodeScyllaCoreInformers.Core().V1().Pods(),
		namespacedKubeInformers.Apps().V1().DaemonSets(),
		namespacedKubeInformers.Batch().V1().Jobs(),
		selfPodInformers.Core().V1().Pods(),
		o.Namespace,
		o.PodName,
		node.Name,
		node.UID,
		o.NodeConfigName,
		types.UID(o.NodeConfigUID),
		o.ScyllaImage,
	)
	if err != nil {
		return fmt.Errorf("can't create node config instance controller: %w", err)
	}

	scyllaInformers.Start(ctx.Done())
	namespacedKubeInformers.Start(ctx.Done())
	localNodeScyllaCoreInformers.Start(ctx.Done())
	selfPodInformers.Start(ctx.Done())

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		nsc.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ntc.Run(ctx)
	}()

	wg.Wait()

	return nil
}
