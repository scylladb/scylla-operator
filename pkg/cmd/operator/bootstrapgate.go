// Copyright (C) 2025 ScyllaDB

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

// FIXME: this is VERY fragile; create actual structs from BNF: https://enterprise.docs.scylladb.com/stable/operating-scylla/admin-tools/scylla-sstable.html
type AutoGenerateSSTables struct {
	SSTables struct {
		Anonymous []struct {
			ClusteringElements []struct {
				Columns struct {
					Bootstrapped struct {
						Value string `json:"value"`
					} `json:"bootstrapped"`
					HostID struct {
						Value string `json:"value"`
					} `json:"host_id"`
				} `json:"columns"`
			} `json:"clustering_elements"`
		} `json:"anonymous"`
	} `json:"sstables"`
}

type BootstrapGateOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName                string
	ClusterStatusConfigMapName string
	SStableSystemLocalDumpPath string

	bootstrapped bool
	hostID       *string

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewBootstrapGateOptions(streams genericclioptions.IOStreams) *BootstrapGateOptions {
	return &BootstrapGateOptions{
		ClientConfig: genericclioptions.NewClientConfig("bootstrap-gate"),
	}
}

func (o *BootstrapGateOptions) AddFlags(cmd *cobra.Command) {
	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	cmd.Flags().StringVarP(&o.ClusterStatusConfigMapName, "cluster-status-configmap-name", "", o.ClusterStatusConfigMapName, "Name of the ConfigMap containing the cluster status.")
	cmd.Flags().StringVarP(&o.SStableSystemLocalDumpPath, "sstable-system-local-dump-path", "", o.SStableSystemLocalDumpPath, "Path to JSON file containing the dump of system.local table.")
}

func NewBootstrapGateCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewBootstrapGateOptions(streams)

	cmd := &cobra.Command{
		Use:   "bootstrap-gate",
		Short: "",
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

func (o *BootstrapGateOptions) Validate(args []string) error {
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

	if len(o.ClusterStatusConfigMapName) == 0 {
		errs = append(errs, fmt.Errorf("cluster-status-configmap-name can't be empty"))
	} else {
		configMapNameValidationErrs := apimachineryvalidation.NameIsDNSSubdomain(o.ClusterStatusConfigMapName, false)
		if len(configMapNameValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid configmap name %q: %v", o.ClusterStatusConfigMapName, configMapNameValidationErrs))
		}
	}

	if len(o.SStableSystemLocalDumpPath) == 0 {
		errs = append(errs, fmt.Errorf("sstable-system-local-dump-path can't be empty"))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *BootstrapGateOptions) Complete(args []string) error {
	var err error

	var rawData []byte
	rawData, err = os.ReadFile(o.SStableSystemLocalDumpPath)
	if err != nil {
		return fmt.Errorf("can't read sstabledump file %q: %w", o.SStableSystemLocalDumpPath, err)
	}

	var data AutoGenerateSSTables
	if len(rawData) != 0 {
		err = json.Unmarshal(rawData, &data)
		if err != nil {
			return fmt.Errorf("can't unmarshall sstabledump file %q: %w", o.SStableSystemLocalDumpPath, err)
		}
	}

	for _, partition := range data.SSTables.Anonymous {
		for _, ce := range partition.ClusteringElements {
			if ce.Columns.Bootstrapped.Value == "COMPLETED" {
				o.bootstrapped = true
			}
			if len(ce.Columns.HostID.Value) != 0 {
				o.hostID = pointer.Ptr(ce.Columns.HostID.Value)
			}
		}
	}

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

	o.scyllaClient, err = scyllaversionedclient.NewForConfig(o.RestConfig)
	if err != nil {
		return fmt.Errorf("can't build scylla clientset: %w", err)
	}

	return nil
}

func (o *BootstrapGateOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
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

func (o *BootstrapGateOptions) Execute(ctx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	var err error
	ignoredNodes := apimachineryutilsets.New[string]()

	// Rolling restart case.
	if o.bootstrapped {
		if o.hostID == nil {
			return fmt.Errorf("node is bootstrapped but hostID is nil, this should never happen")
		}

		// Node is restarting, other nodes are going to consider it down.
		ignoredNodes.Insert(*o.hostID)
	}
	//

	// Node replace case.
	var svc *corev1.Service
	svc, err = o.kubeClient.CoreV1().Services(o.Namespace).Get(ctx, o.ServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get service %q: %w", naming.ManualRef(o.Namespace, o.ServiceName), err)
	}

	replacingHostID, ok := svc.Labels[naming.ReplacingNodeHostIDLabel]
	if ok && len(replacingHostID) == 0 {
		return fmt.Errorf("service %q has an empty %q label, this should never happen", naming.ManualRef(o.Namespace, o.ServiceName), naming.ReplacingNodeHostIDLabel)
	}

	if len(replacingHostID) > 0 {
		ignoredNodes.Insert(replacingHostID)
	}
	//

	// ALTERNATIVE: instead of getting the configmap with statuses, this is where we could ask all other nodes for their statuses (by using broadcastAddresses from corresponding services/pods)

	klog.V(2).InfoS("Waiting for required bootstrap conditions")
	_, err = controllerhelpers.WaitForConfigMapState(
		ctx,
		o.kubeClient.CoreV1().ConfigMaps(o.Namespace),
		o.ClusterStatusConfigMapName,
		controllerhelpers.WaitForStateOptions{},
		func(cm *corev1.ConfigMap) (bool, error) {
			data, ok := cm.Data[naming.ScyllaDBClusterStatusKey]
			if !ok {
				return false, fmt.Errorf("missing %s key", naming.ScyllaDBClusterStatusKey)
			}

			var nodeClusterStatuses []controllerhelpers.ClusterStatus
			err = json.Unmarshal([]byte(data), &nodeClusterStatuses)
			if err != nil {
				return false, fmt.Errorf("can't unmarshal cluster status: %w", err)
			}

			// FIXME: this could be preprocessed
			hostIDs := apimachineryutilsets.New[string]()
			hostIDToStatusMap := make(map[string]map[string]bool)
			for _, nodeClusterStatus := range nodeClusterStatuses {
				if nodeClusterStatus.Error != nil {
					// We might get errors from nodes that haven't started bootstrap yet, so we need to ignore them.
					// If the node is already a part of the cluster, we'll see it in other nodes' statuses.
					continue
				}

				hostIDs.Insert(nodeClusterStatus.HostID)
				otherNodesHostIDToStatusMap := make(map[string]bool, len(nodeClusterStatus.Status))
				for _, otherNodeStatus := range nodeClusterStatus.Status {
					// Add to hostIDs to account for all nodes appearing in any node's status.
					hostIDs.Insert(otherNodeStatus.HostID)
					otherNodesHostIDToStatusMap[otherNodeStatus.HostID] = bool(otherNodeStatus.Status)
				}
				hostIDToStatusMap[nodeClusterStatus.HostID] = otherNodesHostIDToStatusMap
			}

			for hostID := range hostIDs {
				if ignoredNodes.Has(hostID) {
					// We don't care about this node.
					continue
				}

				nodeStatuses, ok := hostIDToStatusMap[hostID]
				if !ok {
					// We don't have a status from this node.
					// It must be a part of the cluster as it appeared in other nodes' statuses.
					// We don't know what it thinks about other nodes, so we must assume the worst.
					return false, nil
				}

				// Check that this node sees all other nodes as up.
				for otherHostID := range hostIDs {
					if ignoredNodes.Has(otherHostID) {
						// We don't care about this node.
						continue
					}

					otherNodeStatus, ok := nodeStatuses[otherHostID]
					if !ok || !otherNodeStatus {
						// This node doesn't see the other node, or sees it as down.
						return false, nil
					}
				}
			}

			return true, nil
		},
	)

	return nil
}
