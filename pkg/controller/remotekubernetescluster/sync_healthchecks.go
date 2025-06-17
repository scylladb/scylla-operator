package remotekubernetescluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
)

const (
	remoteScyllaClientName = "RemoteOpenSourceScylla"
	remoteKubeClientName   = "RemoteKubernetes"
)

func (rkcc *Controller) syncClientHealthchecks(ctx context.Context, key types.NamespacedName, rkc *scyllav1alpha1.RemoteKubernetesCluster, status *scyllav1alpha1.RemoteKubernetesClusterStatus) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var errs []error

	if rkc.Spec.ClientHealthcheckProbes == nil {
		klog.V(4).InfoS("Healthchecks are disabled", "RemoteKubernetesCluster", rkc.Name)
		return progressingConditions, nil
	}
	rkcc.queue.AddAfter(key, time.Duration(rkc.Spec.ClientHealthcheckProbes.PeriodSeconds)*time.Second)

	now := time.Now()
	klog.V(4).InfoS("Started syncing client healthchecks")
	defer func() {
		klog.V(4).InfoS("Finished syncing client healthchecks", "Duration", time.Since(now).String())
	}()

	clients := map[string]func(clusterName string) (discovery.DiscoveryInterface, error){
		remoteScyllaClientName: func(clusterName string) (discovery.DiscoveryInterface, error) {
			scyllaClient, err := rkcc.clusterScyllaClient.Cluster(rkc.Name)
			if err != nil {
				return nil, fmt.Errorf("can't get open-source Scylla client to cluster %q: %w", rkc.Name, err)
			}
			return scyllaClient.Discovery(), nil
		},
		remoteKubeClientName: func(clusterName string) (discovery.DiscoveryInterface, error) {
			kubeClient, err := rkcc.clusterKubeClient.Cluster(rkc.Name)
			if err != nil {
				return nil, fmt.Errorf("can't get Kubernetes client to cluster %q: %w", rkc.Name, err)
			}
			return kubeClient.Discovery(), nil
		},
	}

	desiredClients := len(clients)
	successfulClients := 0
	var messages []string
	for clientName, getDiscoveryClient := range clients {
		discoveryClient, err := getDiscoveryClient(rkc.Name)
		if err != nil {
			message := fmt.Sprintf("can't get discovery %q client for %q cluster", clientName, rkc.Name)
			messages = append(messages, fmt.Sprintf("%s: %v", message, err))
			errs = append(errs, fmt.Errorf("%s: %w", message, err))
			continue
		}

		_, err = discoveryClient.ServerVersion()
		if err != nil {
			message := fmt.Sprintf("can't check server info for %q client", clientName)
			messages = append(messages, fmt.Sprintf("%s: %v", message, err))
			klog.ErrorS(err, message, "Client", clientName, "RemoteKubernetesCluster", rkc.Name)

			continue
		}

		successfulClients++
	}

	if successfulClients != desiredClients {
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               clientHealthcheckControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "ClientProbeFailed",
			Message:            strings.Join(messages, "\n"),
			ObservedGeneration: rkc.Generation,
		})
	} else {
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               clientHealthcheckControllerAvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "ClientProbeSucceded",
			Message:            strings.Join(messages, "\n"),
			ObservedGeneration: rkc.Generation,
		})
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't run healthcheck probes: %w", err)
	}

	return progressingConditions, nil
}
