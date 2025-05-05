// Copyright (c) 2024 ScyllaDB.

package v1alpha1

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclientset "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

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

func GetRemoteDatacenterScyllaConfigClient(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteScyllaAdminClient *scyllaclientset.Clientset, remoteKubeAdminClient *kubernetes.Clientset, agentAuthToken string) (*scyllaclient.ConfigClient, error) {
	dcStatus, _, ok := slices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
		return dc.Name == dcStatus.Name
	})
	if !ok {
		return nil, fmt.Errorf("can't find datacenter %q in ScyllaDBCluster %q status", dc.Name, naming.ObjRef(sc))
	}

	if dcStatus.RemoteNamespaceName == nil {
		return nil, fmt.Errorf("empty remote namespace name in datacenter %q ScyllaDBCluster %q status", dc.Name, naming.ObjRef(sc))
	}

	sdc, err := remoteScyllaAdminClient.ScyllaV1alpha1().ScyllaDBDatacenters(*dcStatus.RemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, dc), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ScyllaDBDatacenterName(sc, dc), err)
	}

	svc, err := remoteKubeAdminClient.CoreV1().Services(*dcStatus.RemoteNamespaceName).Get(ctx, naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 0), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get Service %q: %w", naming.MemberServiceName(sdc.Spec.Racks[0], sdc, 0), err)
	}

	pod, err := remoteKubeAdminClient.CoreV1().Pods(*dcStatus.RemoteNamespaceName).Get(ctx, naming.PodNameFromService(svc), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get Pod %q: %w", naming.PodNameFromService(svc), err)
	}

	host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
	if err != nil {
		return nil, fmt.Errorf("can't get Scylla hosts: %w", err)
	}

	return scyllaclient.NewConfigClient(host, agentAuthToken), nil
}
