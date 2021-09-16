// Copyright (C) 2021 ScyllaDB

package operator

import (
	"context"
	"fmt"
	"time"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type NodeConfigOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	NodeName             string
	ScyllaNodeConfigName string
	ScyllaImage          string
	DisableOptimizations bool

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewNodeConfigOptions(streams genericclioptions.IOStreams) *NodeConfigOptions {
	return &NodeConfigOptions{
		ClientConfig:        genericclioptions.NewClientConfig("node-config"),
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewNodeConfigCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeConfigOptions(streams)

	cmd := &cobra.Command{
		Use:   "node-config",
		Short: "Run node setup and optimizations.",
		Long:  "Run node setup and optimizations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd.Name())
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

	cmd.Flags().StringVarP(&o.NodeName, "node-name", "", o.NodeName, "Name of the node where this Pod is running.")
	cmd.Flags().StringVarP(&o.ScyllaNodeConfigName, "scylla-node-config-name", "", o.ScyllaNodeConfigName, "Name of Scylla Node Config spec.")
	cmd.Flags().StringVarP(&o.ScyllaImage, "scylla-image", "", o.ScyllaImage, "Scylla image used for running perftune.")
	cmd.Flags().BoolVarP(&o.DisableOptimizations, "disable-optimizations", "", o.DisableOptimizations, "Controls if optimizations are disabled")

	return cmd
}

func (o *NodeConfigOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())

	if len(o.NodeName) == 0 {
		errs = append(errs, fmt.Errorf("node-name cannot be empty"))
	}
	if len(o.ScyllaNodeConfigName) == 0 {
		errs = append(errs, fmt.Errorf("scylla-node-config-name cannot be empty"))
	}
	if len(o.ScyllaImage) == 0 {
		errs = append(errs, fmt.Errorf("scylla-image cannot be empty"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *NodeConfigOptions) Complete() error {
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

func (o *NodeConfigOptions) Run(streams genericclioptions.IOStreams, commandName string) error {
	klog.Infof("%s version %s", commandName, version.Get())
	klog.Infof("loglevel is set to %q", cmdutil.GetLoglevel())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	criClient, err := cri.NewClient(ctx, naming.HostFilesystemDirName)
	if err != nil {
		return err
	}

	singleNamespaceInformer := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 12*time.Hour, informers.WithNamespace(naming.ScyllaOperatorNodeTuningNamespace))
	scyllaNodeConfigInformer := scyllainformers.NewSharedInformerFactoryWithOptions(o.scyllaClient, 12*time.Hour, scyllainformers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", o.ScyllaNodeConfigName).String()
		},
	))
	singleNodeScyllaPodsInformer := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 12*time.Hour, informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.LabelSelector = naming.ScyllaSelector().String()
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", o.NodeName).String()
		},
	))
	nodeOwnedConfigMapInformer := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 12*time.Hour, informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.LabelSelector = labels.SelectorFromSet(map[string]string{
				naming.NodeConfigNameLabel: o.ScyllaNodeConfigName,
			}).String()
		},
	))

	ntc, err := nodeconfigpod.NewController(
		o.kubeClient,
		o.scyllaClient,
		criClient,
		scyllaNodeConfigInformer.Scylla().V1alpha1().ScyllaNodeConfigs(),
		singleNodeScyllaPodsInformer.Core().V1().Pods(),
		nodeOwnedConfigMapInformer.Core().V1().ConfigMaps(),
		singleNamespaceInformer.Apps().V1().DaemonSets(),
		singleNamespaceInformer.Batch().V1().Jobs(),
		o.NodeName,
		o.ScyllaNodeConfigName,
		o.ScyllaImage,
	)
	if err != nil {
		return fmt.Errorf("can't create node tuner controller: %w", err)
	}

	// Start informers.
	scyllaNodeConfigInformer.Start(ctx.Done())
	singleNodeScyllaPodsInformer.Start(ctx.Done())
	nodeOwnedConfigMapInformer.Start(ctx.Done())
	singleNamespaceInformer.Start(ctx.Done())

	ntc.Run(ctx)

	<-ctx.Done()

	return nil
}
