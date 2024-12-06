// Copyright (c) 2024 ScyllaDB.

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func WaitForFullScyllaDBClusterQuorum(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *scyllav1alpha1.ScyllaDBCluster) error {
	allBroadcastAddresses := map[string][]string{}
	var sortedAllBroadcastAddresses []string

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for DC %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		if !ok {
			return fmt.Errorf("can't find %q datacenter status", dc.Name)
		}
		if dcStatus.RemoteNamespaceName == nil {
			return fmt.Errorf("unexpected nil remote namespace name in %q datacenter status", dc.Name)
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		hosts, err := GetBroadcastAddresses(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc)
		if err != nil {
			return fmt.Errorf("can't get broadcast addresses for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
		}
		allBroadcastAddresses[dc.Name] = hosts
		sortedAllBroadcastAddresses = append(sortedAllBroadcastAddresses, hosts...)
	}
	err := errors.Join(errs...)
	if err != nil {
		return err
	}

	sort.Strings(sortedAllBroadcastAddresses)

	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for DC %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
			return dcStatus.Name == dc.Name
		})
		if !ok {
			return fmt.Errorf("can't find %q datacenter status", dc.Name)
		}
		if dcStatus.RemoteNamespaceName == nil {
			return fmt.Errorf("unexpected nil remote namespace name in %q datacenter status", dc.Name)
		}

		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", dc.Name, err))
			continue
		}

		err = waitForFullQuorum(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc, sortedAllBroadcastAddresses)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return fmt.Errorf("can't wait for scylla nodes to reach status consistency: %w", err)
	}

	framework.Infof("ScyllaDB nodes have reached status consistency.")

	return nil
}

// TODO: Should be unified with function coming from test/helpers once e2e's there starts using ScyllaDBDatacenter API.
func waitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, sortedExpectedHosts []string) error {
	scyllaClient, hosts, err := GetScyllaClient(ctx, client, sdc)
	if err != nil {
		return fmt.Errorf("can't get scylla client: %w", err)
	}
	defer scyllaClient.Close()

	// Wait for node status to propagate and reach consistency.
	// This can take a while so let's set a large enough timeout to avoid flakes.
	return wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		allSeeAllAsUN := true
		infoMessages := make([]string, 0, len(hosts))
		var errs []error
		for _, h := range hosts {
			s, err := scyllaClient.Status(ctx, h)
			if err != nil {
				return true, fmt.Errorf("can't get scylla status on node %q: %w", h, err)
			}

			sHosts := s.Hosts()
			sort.Strings(sHosts)
			if !reflect.DeepEqual(sHosts, sortedExpectedHosts) {
				errs = append(errs, fmt.Errorf("node %q thinks the cluster consists of different nodes: %s", h, sHosts))
			}

			downHosts := s.DownHosts()
			infoMessages = append(infoMessages, fmt.Sprintf("Node %q, down: %q, up: %q", h, strings.Join(downHosts, "\n"), strings.Join(s.LiveHosts(), ",")))

			if len(downHosts) != 0 {
				allSeeAllAsUN = false
			}
		}

		if !allSeeAllAsUN {
			framework.Infof("ScyllaDB nodes have not reached status consistency yet. Statuses:\n%s", strings.Join(infoMessages, ","))
		}

		err = errors.Join(errs...)
		if err != nil {
			framework.Infof("ScyllaDB nodes encountered an error. Statuses:\n%s", strings.Join(infoMessages, ","))
			return true, err
		}

		return allSeeAllAsUN, nil
	})
}

func GetScyllaClient(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) (*scyllaclient.Client, []string, error) {
	hosts, err := GetBroadcastRPCAddresses(ctx, client, sdc)
	if err != nil {
		return nil, nil, err
	}

	if len(hosts) < 1 {
		return nil, nil, fmt.Errorf("no services found")
	}

	tokenSecret, err := client.Secrets(sdc.Namespace).Get(ctx, naming.AgentAuthTokenSecretName(sdc), metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	authToken, err := helpers.GetAgentAuthTokenFromSecret(tokenSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get auth token: %w", err)
	}

	cfg := scyllaclient.DefaultConfig(authToken, hosts...)
	scyllaClient, err := scyllaclient.NewClient(cfg)
	if err != nil {
		return nil, nil, err
	}

	return scyllaClient, hosts, nil
}

func GetMemberServiceSelector(sdc *scyllav1alpha1.ScyllaDBDatacenter) labels.Selector {
	return labels.Set{
		naming.ClusterNameLabel:       sdc.Name,
		naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
	}.AsSelector()
}

func GetBroadcastRPCAddresses(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]string, error) {
	serviceList, err := client.Services(sdc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sdc).String(),
	})
	if err != nil {
		return nil, err
	}

	var broadcastRPCAddresses []string
	for _, svc := range serviceList.Items {
		broadcastRPCAddress, err := GetBroadcastRPCAddress(ctx, client, sdc, &svc)
		if err != nil {
			return nil, fmt.Errorf("can't get broadcast rpc address for service %q: %w", naming.ObjRef(&svc), err)
		}

		broadcastRPCAddresses = append(broadcastRPCAddresses, broadcastRPCAddress)
	}

	return broadcastRPCAddresses, err
}

func GetBroadcastRPCAddress(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service) (string, error) {
	host := svc.Spec.ClusterIP

	if host == corev1.ClusterIPNone {
		pod, err := client.Pods(sdc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, svc.Name), err)
		}
		host = pod.Status.PodIP
		if len(host) < 1 {
			return "", fmt.Errorf("empty podIP of pod %q", naming.ManualRef(sdc.Namespace, svc.Name))
		}
	}

	configClient, err := GetScyllaConfigClient(ctx, client, sdc, host)
	if err != nil {
		return "", fmt.Errorf("can't create scylla config client with host %q: %w", host, err)
	}

	broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("can't get broadcast_rpc_address of host %q: %w", host, err)
	}

	return broadcastRPCAddress, nil
}

func GetScyllaConfigClient(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, host string) (*scyllaclient.ConfigClient, error) {
	tokenSecret, err := client.Secrets(sdc.Namespace).Get(ctx, naming.AgentAuthTokenSecretName(sdc), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get Secret %q: %w", naming.ManualRef(sdc.Namespace, naming.AgentAuthTokenSecretName(sdc)), err)
	}

	authToken, err := helpers.GetAgentAuthTokenFromSecret(tokenSecret)
	if err != nil {
		return nil, fmt.Errorf("can't get auth token: %w", err)
	}

	configClient := scyllaclient.NewConfigClient(host, authToken)
	return configClient, nil
}

func GetBroadcastAddresses(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]string, error) {
	serviceList, err := client.Services(sdc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sdc).String(),
	})
	if err != nil {
		return nil, err
	}

	var broadcastAddresses []string
	for _, svc := range slices.ConvertSlice(serviceList.Items, pointer.Ptr[corev1.Service]) {
		podName := naming.PodNameFromService(svc)
		pod, err := client.Pods(sdc.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
		}

		broadcastAddress, err := GetBroadcastAddress(ctx, client, sdc, svc, pod)
		if err != nil {
			return nil, fmt.Errorf("can't get broadcast address of Service %q: %w", naming.ObjRef(svc), err)
		}

		broadcastAddresses = append(broadcastAddresses, broadcastAddress)
	}

	return broadcastAddresses, nil
}

func GetBroadcastAddress(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service, pod *corev1.Pod) (string, error) {
	host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
	if err != nil {
		return "", fmt.Errorf("can't get Scylla host for Service %q: %w", naming.ObjRef(svc), err)
	}

	configClient, err := GetScyllaConfigClient(ctx, client, sdc, host)
	if err != nil {
		return "", fmt.Errorf("can't create scylla config client with host %q: %w", host, err)
	}
	broadcastAddress, err := configClient.BroadcastAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("can't get broadcast_address of host %q: %w", host, err)
	}

	return broadcastAddress, nil
}
