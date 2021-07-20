package operator

import (
	"context"
	"fmt"
	"os"
	"path"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type AutoReplaceOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName string

	kubeClient kubernetes.Interface
}

func NewAutoReplaceOptions(streams genericclioptions.IOStreams) *AutoReplaceOptions {
	return &AutoReplaceOptions{
		ClientConfig:        genericclioptions.NewClientConfig("replace-verifier"),
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewAutoReplaceCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAutoReplaceOptions(streams)

	cmd := &cobra.Command{
		Use:   "auto-replace",
		Short: "Run the auto-replace detector.",
		Long:  `Run the auto-replace detector.`,
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

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the Service.")

	return cmd
}

func (o *AutoReplaceOptions) Validate() error {
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

	return apierrors.NewAggregate(errs)
}

func (o *AutoReplaceOptions) Complete() error {
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

func (o *AutoReplaceOptions) Run(streams genericclioptions.IOStreams, commandName string) error {
	klog.Infof("%s version %s", commandName, version.Get())
	klog.Infof("loglevel is set to %q", cmdutil.GetLoglevel())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := o.kubeClient.CoreV1().Services(o.Namespace).Get(ctx, o.ServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service: %v", err)
	}
	pod, err := o.kubeClient.CoreV1().Pods(o.Namespace).Get(ctx, o.ServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	// Check if first node in cluster
	podList, err := o.kubeClient.CoreV1().Pods(o.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			naming.ClusterNameLabel: pod.Labels[naming.ClusterNameLabel],
		}).String(),
	})
	if err != nil {
		return fmt.Errorf("can't list pods: %w", err)
	}

	foundOther := false
	for i := range podList.Items {
		p := &podList.Items[i]
		if p.Name != pod.Name {
			foundOther = true
			break
		}
	}
	if !foundOther {
		// First node in cluster skips bootstrap, can't replace other node
		klog.Infof("first node in cluster, skipping")
		return nil
	}

	// Create dummy cluster for helper functions
	sc := &scyllav1.ScyllaCluster{ObjectMeta: metav1.ObjectMeta{
		Name:      pod.Labels[naming.ClusterNameLabel],
		Namespace: o.Namespace,
	}}

	scyllaClient, _, err := scyllaclient.GetScyllaClientForScyllaCluster(ctx, o.kubeClient.CoreV1(), sc)
	if err != nil {
		return fmt.Errorf("can't get scylla client: %v", err)
	}

	st, err := scyllaClient.Status(ctx, "")
	if err != nil {
		return fmt.Errorf("can't get status: %v", err)
	}

	inGossip := false
	for _, h := range st.Hosts() {
		if h == service.Spec.ClusterIP {
			inGossip = true
			break
		}
	}
	if !inGossip {
		klog.Infof("address %s doesn't exist in gossip, skipping", service.Spec.ClusterIP)
		return nil
	}

	filepath := path.Join(naming.ScyllaReplaceAddressDirName, naming.ScyllaReplaceAddressFileName)
	if _, err = os.OpenFile(filepath, os.O_RDONLY|os.O_CREATE, 0666); err != nil {
		return fmt.Errorf("failed to open file %s: %v", filepath, err)
	}
	klog.Infof("touched %s", filepath)

	return nil
}
