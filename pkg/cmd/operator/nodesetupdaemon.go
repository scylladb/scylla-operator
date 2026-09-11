// Copyright (c) 2023 ScyllaDB.

package operator

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controller/nodesetup"
	"github.com/scylladb/scylla-operator/pkg/controller/nodetune"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/kubelet"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

type NodeSetupDaemonOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	PodName        string
	NodeName       string
	NodeConfigName string
	NodeConfigUID  string
	ScyllaImage    string
	OperatorImage  string

	CRIEndpoints                []string
	KubeletPodResourcesEndpoint string

	kubeClient kubernetes.Interface
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
		KubeletPodResourcesEndpoint: "unix:///var/lib/kubelet/pod-resources/kubelet.sock",
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
	cmd.Flags().StringArrayVarP(&o.CRIEndpoints, "cri-endpoint", "", o.CRIEndpoints, "CRI endpoint to connect to. It will try to connect to any of them, in the given order.")
	cmd.Flags().StringVarP(&o.KubeletPodResourcesEndpoint, "kubelet-pod-resources-endpoint", "", o.KubeletPodResourcesEndpoint, "Endpoint to kubelet PodResources API server")
	cmd.Flags().StringVarP(&o.OperatorImage, "operator-image", "", o.OperatorImage, "Operator image used for running tuning.")

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

	if len(o.KubeletPodResourcesEndpoint) == 0 {
		errs = append(errs, fmt.Errorf("kubelet-pod-resources-endpoint can't be empty"))
	}

	return apimachineryutilerrors.NewAggregate(errs)
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

	return nil
}

func (o *NodeSetupDaemonOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	cmdutil.LogCommandStarting(cmd)
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
	defer criClient.Close()

	kubeletPodResourcesClient, err := kubelet.NewPodResourcesClient(ctx, o.KubeletPodResourcesEndpoint)
	if err != nil {
		return fmt.Errorf("can't create kubelet pod resources client: %w", err)
	}
	defer kubeletPodResourcesClient.Close()

	var node *corev1.Node
	err = apimachineryutilwait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func(fCtx context.Context) (bool, error) {
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

	mgr, err := controllermanager.NewManager(
		o.RestConfig,
		klog.NewKlogr(),
		nodetune.CacheOptions(o.Namespace, node.Name),
		controllermanager.MetricsDisabledBindAddress,
	)
	if err != nil {
		return fmt.Errorf("can't create controller manager: %w", err)
	}

	nsc, err := nodesetup.NewController(
		ctx,
		mgr.GetClient(),
		mgr.GetEventRecorderFor("nodesetup-controller"),
		node.Name,
		node.UID,
		o.NodeConfigName,
		types.UID(o.NodeConfigUID),
	)
	if err != nil {
		return fmt.Errorf("can't create node config instance controller: %w", err)
	}
	defer nsc.Close()

	err = nsc.SetupWithManager(mgr, controller.Options{
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return fmt.Errorf("can't set up node setup controller: %w", err)
	}

	ntc := nodetune.NewController(
		mgr.GetClient(),
		mgr.GetAPIReader(),
		mgr.GetEventRecorderFor("nodeconfigdaemon-controller"),
		criClient,
		kubeletPodResourcesClient,
		o.Namespace,
		o.PodName,
		node.Name,
		node.UID,
		o.NodeConfigName,
		types.UID(o.NodeConfigUID),
		o.ScyllaImage,
		o.OperatorImage,
	)
	err = ntc.SetupWithManager(mgr, nodetune.ControllerOptions())
	if err != nil {
		return fmt.Errorf("can't set up node tune controller: %w", err)
	}

	err = mgr.Start(ctx)
	if err != nil {
		return fmt.Errorf("controller manager failed: %w", err)
	}

	return nil
}
