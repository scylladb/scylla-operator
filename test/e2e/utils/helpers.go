// Copyright (C) 2021 ScyllaDB

package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

func IsNodeConfigRolledOut(nc *scyllav1alpha1.NodeConfig) (bool, error) {
	statusConditions := nc.Status.Conditions.ToMetaV1Conditions()

	if !helpers.IsStatusConditionPresentAndTrue(statusConditions, scyllav1alpha1.AvailableCondition, nc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(statusConditions, scyllav1alpha1.ProgressingCondition, nc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(statusConditions, scyllav1alpha1.DegradedCondition, nc.Generation) {
		return false, nil
	}

	framework.Infof("NodeConfig %q (RV=%s) is rolled out", naming.ObjRef(nc), nc.ResourceVersion)
	return true, nil
}

func GetMatchingNodesForNodeConfig(ctx context.Context, nodeGetter corev1client.NodesGetter, nc *scyllav1alpha1.NodeConfig) ([]*corev1.Node, error) {
	nodeList, err := nodeGetter.Nodes().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	var matchingNodes []*corev1.Node

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		isSelectingNode, err := controllerhelpers.IsNodeConfigSelectingNode(nc, node)
		o.Expect(err).NotTo(o.HaveOccurred())

		if isSelectingNode {
			matchingNodes = append(matchingNodes, node)
		}
	}

	return matchingNodes, nil
}

func RolloutTimeoutForScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	return SyncTimeout + time.Duration(GetMemberCount(sc))*memberRolloutTimeout + cleanupJobTimeout
}

func RolloutTimeoutForMultiDatacenterScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	return SyncTimeout + time.Duration(GetMemberCount(sc))*multiDatacenterMemberRolloutTimeout + cleanupJobTimeout
}

func GetMemberCount(sc *scyllav1.ScyllaCluster) int32 {
	members := int32(0)
	for _, r := range sc.Spec.Datacenter.Racks {
		members += r.Members
	}

	return members
}

func ContextForRollout(parent context.Context, sc *scyllav1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, RolloutTimeoutForScyllaCluster(sc))
}

func ContextForMultiDatacenterRollout(parent context.Context, sc *scyllav1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, RolloutTimeoutForMultiDatacenterScyllaCluster(sc))
}

func SyncTimeoutForScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	tasks := int64(len(sc.Spec.Repairs) + len(sc.Spec.Backups))
	return baseManagerSyncTimeout + time.Duration(tasks)*managerTaskSyncTimeout
}

func ContextForManagerSync(parent context.Context, sc *scyllav1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, SyncTimeoutForScyllaCluster(sc))
}

func ContextForPodStartup(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, imagePullTimeout)
}

func RolloutTimeoutForRemoteKubernetesCluster(rkc *scyllav1alpha1.RemoteKubernetesCluster) time.Duration {
	healthcheckProbesTimeout := time.Duration(0)
	if rkc.Spec.ClientHealthcheckProbes != nil {
		healthcheckProbesTimeout = 2 * time.Duration(rkc.Spec.ClientHealthcheckProbes.PeriodSeconds) * time.Second
	}
	return time.Minute + healthcheckProbesTimeout
}

func ContextForRemoteKubernetesClusterRollout(ctx context.Context, rkc *scyllav1alpha1.RemoteKubernetesCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, RolloutTimeoutForRemoteKubernetesCluster(rkc))
}

func RolloutTimeoutForScyllaDBCluster(sc *scyllav1alpha1.ScyllaDBCluster) time.Duration {
	return SyncTimeout + time.Duration(controllerhelpers.GetScyllaDBClusterNodeCount(sc))*memberRolloutTimeout
}

func RolloutTimeoutForMultiDatacenterScyllaDBCluster(sc *scyllav1alpha1.ScyllaDBCluster) time.Duration {
	return SyncTimeout + time.Duration(controllerhelpers.GetScyllaDBClusterNodeCount(sc))*multiDatacenterMemberRolloutTimeout
}

func ContextForMultiDatacenterScyllaDBClusterRollout(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, RolloutTimeoutForMultiDatacenterScyllaDBCluster(sc))
}

func IsScyllaClusterRolledOut(sc *scyllav1.ScyllaCluster) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(sc.Status.Conditions, scyllav1.AvailableCondition, sc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sc.Status.Conditions, scyllav1.ProgressingCondition, sc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sc.Status.Conditions, scyllav1.DegradedCondition, sc.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaCluster %s (RV=%s) is rolled out", klog.KObj(sc), sc.ResourceVersion)

	return true, nil
}

func IsScyllaDBMonitoringRolledOut(sm *scyllav1alpha1.ScyllaDBMonitoring) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(sm.Status.Conditions, scyllav1alpha1.AvailableCondition, sm.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sm.Status.Conditions, scyllav1alpha1.ProgressingCondition, sm.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sm.Status.Conditions, scyllav1alpha1.DegradedCondition, sm.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaDBMonitoring %s (RV=%s) is rolled out", klog.KObj(sm), sm.ResourceVersion)

	return true, nil
}

func IsRemoteKubernetesClusterRolledOut(rkc *scyllav1alpha1.RemoteKubernetesCluster) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(rkc.Status.Conditions, scyllav1alpha1.AvailableCondition, rkc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(rkc.Status.Conditions, scyllav1alpha1.ProgressingCondition, rkc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(rkc.Status.Conditions, scyllav1alpha1.DegradedCondition, rkc.Generation) {
		return false, nil
	}

	framework.Infof("RemoteKubernetesCluster %s (RV=%s) is rolled out", klog.KObj(rkc), rkc.ResourceVersion)

	return true, nil
}

func IsScyllaDBClusterRolledOut(sc *scyllav1alpha1.ScyllaDBCluster) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(sc.Status.Conditions, scyllav1alpha1.AvailableCondition, sc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sc.Status.Conditions, scyllav1alpha1.ProgressingCondition, sc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sc.Status.Conditions, scyllav1alpha1.DegradedCondition, sc.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaDBCluster %s (RV=%s) is rolled out", klog.KObj(sc), sc.ResourceVersion)

	return true, nil
}

func IsScyllaDBClusterDegraded(sc *scyllav1alpha1.ScyllaDBCluster) (bool, error) {
	return helpers.IsStatusConditionPresentAndTrue(sc.Status.Conditions, scyllav1alpha1.DegradedCondition, sc.Generation), nil
}

func RunEphemeralContainerAndCollectLogs(ctx context.Context, client corev1client.PodInterface, podName string, ec *corev1.EphemeralContainer) (*corev1.Pod, []byte, error) {
	pod, err := RunEphemeralContainerAndWaitForCompletion(ctx, client, podName, ec)
	if err != nil {
		return nil, nil, fmt.Errorf("can't run ephemeral container: %w", err)
	}

	logOptions := &corev1.PodLogOptions{
		Container: ec.EphemeralContainerCommon.Name,
	}
	logs := &bytes.Buffer{}

	err = collect.GetPodLogs(ctx, client, logs, podName, logOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("can't collect Pod logs: %w", err)
	}

	return pod, logs.Bytes(), nil
}

func RunEphemeralContainerAndWaitForCompletion(ctx context.Context, client corev1client.PodInterface, podName string, ec *corev1.EphemeralContainer) (*corev1.Pod, error) {
	ephemeralPod := &corev1.Pod{
		Spec: corev1.PodSpec{
			EphemeralContainers: []corev1.EphemeralContainer{*ec},
		},
	}
	patch, err := helpers.CreateTwoWayMergePatch(&corev1.Pod{}, ephemeralPod)
	if err != nil {
		return nil, fmt.Errorf("can't create two-way merge patch: %w", err)
	}

	ephemeralPod, err = client.Patch(
		ctx,
		podName,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
		"ephemeralcontainers",
	)
	if err != nil {
		return nil, fmt.Errorf("can't path pod %q to add ephemeral container: %w", podName, err)
	}

	return controllerhelpers.WaitForPodState(
		ctx,
		client,
		podName,
		controllerhelpers.WaitForStateOptions{},
		func(pod *corev1.Pod) (bool, error) {
			s := controllerhelpers.FindContainerStatus(pod, ec.Name)
			if s == nil {
				framework.Infof("Waiting for the ephemeral container %q in Pod %q to be created", ec.Name, naming.ObjRef(pod))
				return false, nil
			}

			if s.State.Terminated != nil {
				return true, nil
			}

			if s.State.Running != nil {
				framework.Infof("Waiting for the ephemeral container %q in Pod %q to finish", ec.Name, naming.ObjRef(pod))
				return false, nil
			}

			framework.Infof("Waiting for the ephemeral container %q in Pod %q to start", ec.Name, naming.ObjRef(pod))
			return false, nil
		},
	)
}

func GetStatefulSetsForScyllaCluster(ctx context.Context, client appv1client.AppsV1Interface, sc *scyllav1.ScyllaCluster) (map[string]*appsv1.StatefulSet, error) {
	statefulsetList, err := client.StatefulSets(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.ClusterNameLabel: sc.Name,
		}.AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	res := map[string]*appsv1.StatefulSet{}
	for _, s := range statefulsetList.Items {
		rackName := s.Labels[naming.RackNameLabel]
		res[rackName] = &s
	}

	return res, nil
}

func GetPodsForStatefulSet(ctx context.Context, client corev1client.CoreV1Interface, sts *appsv1.StatefulSet) (map[string]*corev1.Pod, error) {
	return utilsv1alpha1.GetPodsForStatefulSet(ctx, client, sts)
}

func GetDaemonSetsForNodeConfig(ctx context.Context, client appv1client.AppsV1Interface, nc *scyllav1alpha1.NodeConfig) ([]*appsv1.DaemonSet, error) {
	daemonSetList, err := client.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.NodeConfigNameLabel: nc.Name,
		}.AsSelector().String(),
	})
	if err != nil {
		return nil, fmt.Errorf("can't list daemonsets: %w", err)
	}

	var res []*appsv1.DaemonSet
	for _, s := range daemonSetList.Items {
		controllerRef := metav1.GetControllerOfNoCopy(&s)
		if controllerRef == nil {
			continue
		}

		if controllerRef.UID != nc.UID {
			continue
		}

		res = append(res, &s)
	}

	return res, nil
}

func GetScyllaClient(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) (*scyllaclient.Client, []string, error) {
	hosts, err := GetBroadcastRPCAddresses(ctx, client, sc)
	if err != nil {
		return nil, nil, err
	}

	if len(hosts) < 1 {
		return nil, nil, fmt.Errorf("no services found")
	}

	tokenSecret, err := client.Secrets(sc.Namespace).Get(ctx, naming.AgentAuthTokenSecretNameForScyllaCluster(sc), metav1.GetOptions{})
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

func GetCurrentTokenRingHash(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) (string, error) {
	scyllaClient, hosts, err := GetScyllaClient(ctx, client, sc)
	if err != nil {
		return "", fmt.Errorf("can't get scylla client: %w", err)
	}

	tokenRing, err := scyllaClient.GetTokenRing(ctx, hosts[0])
	if err != nil {
		return "", fmt.Errorf("can't get token ring: %w", err)
	}

	tokenRingHash, err := hash.HashObjects(tokenRing)
	if err != nil {
		return "", fmt.Errorf("can't hash token ring: %w", err)
	}

	return tokenRingHash, nil
}

func GetScyllaConfigClient(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, host string) (*scyllaclient.ConfigClient, error) {
	tokenSecret, err := client.Secrets(sc.Namespace).Get(ctx, naming.AgentAuthTokenSecretNameForScyllaCluster(sc), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get Secret %q: %w", naming.ManualRef(sc.Namespace, naming.AgentAuthTokenSecretNameForScyllaCluster(sc)), err)
	}

	authToken, err := helpers.GetAgentAuthTokenFromSecret(tokenSecret)
	if err != nil {
		return nil, fmt.Errorf("can't get auth token: %w", err)
	}

	configClient := scyllaclient.NewConfigClient(host, authToken)
	return configClient, nil
}

func GetBroadcastAddresses(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sc).String(),
	})
	if err != nil {
		return nil, err
	}

	var broadcastAddresses []string
	for _, svc := range oslices.ConvertSlice(serviceList.Items, pointer.Ptr[corev1.Service]) {
		podName := naming.PodNameFromService(svc)
		pod, err := client.Pods(sc.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sc.Namespace, podName), err)
		}

		broadcastAddress, err := GetBroadcastAddress(ctx, client, sc, svc, pod)
		if err != nil {
			return nil, fmt.Errorf("can't get broadcast address of Service %q: %w", naming.ObjRef(svc), err)
		}

		broadcastAddresses = append(broadcastAddresses, broadcastAddress)
	}

	return broadcastAddresses, nil
}

func GetBroadcastAddress(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, svc *corev1.Service, pod *corev1.Pod) (string, error) {
	host, err := controllerhelpers.GetScyllaHostForScyllaCluster(sc, svc, pod)
	if err != nil {
		return "", fmt.Errorf("can't get Scylla host for Service %q: %w", naming.ObjRef(svc), err)
	}

	configClient, err := GetScyllaConfigClient(ctx, client, sc, host)
	if err != nil {
		return "", fmt.Errorf("can't create scylla config client with host %q: %w", host, err)
	}
	broadcastAddress, err := configClient.BroadcastAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("can't get broadcast_address of host %q: %w", host, err)
	}

	return broadcastAddress, nil
}

func GetBroadcastRPCAddresses(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sc).String(),
	})
	if err != nil {
		return nil, err
	}

	var broadcastRPCAddresses []string
	for _, svc := range serviceList.Items {
		broadcastRPCAddress, err := GetBroadcastRPCAddress(ctx, client, sc, &svc)
		if err != nil {
			return nil, fmt.Errorf("can't get broadcast rpc address for service %q: %w", naming.ObjRef(&svc), err)
		}

		broadcastRPCAddresses = append(broadcastRPCAddresses, broadcastRPCAddress)
	}

	return broadcastRPCAddresses, err
}

func GetBroadcastRPCAddressesAndUUIDsByDC(ctx context.Context, dcClientMap map[string]corev1client.CoreV1Interface, scs []*scyllav1.ScyllaCluster) (map[string][]string, map[string][]string, error) {
	allBroadcastRPCAddresses := map[string][]string{}
	allUUIDs := map[string][]string{}

	for _, sc := range scs {
		client, ok := dcClientMap[sc.Spec.Datacenter.Name]
		if !ok {
			return nil, nil, fmt.Errorf("client is missing for ScyllaCluster %q", naming.ObjRef(sc))
		}

		broadcastRPCAddresses, uuids, err := GetBroadcastRPCAddressesAndUUIDs(ctx, client, sc)
		if err != nil {
			return nil, nil, fmt.Errorf("can't get broadcast rpc address and UUID for ScyllaDBDatacenter %q: %w", naming.ObjRef(sc), err)
		}
		allBroadcastRPCAddresses[sc.Spec.Datacenter.Name] = broadcastRPCAddresses
		allUUIDs[sc.Spec.Datacenter.Name] = uuids
	}

	return allBroadcastRPCAddresses, allUUIDs, nil
}

func GetBroadcastRPCAddressesAndUUIDs(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, []string, error) {
	scyllaClient, broadcastRPCAddresses, err := GetScyllaClient(ctx, client, sc)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get scylla client for ScyllaCluster %q: %w", naming.ObjRef(sc), err)
	}

	var uuids []string
	for _, broadcastRPCAddress := range broadcastRPCAddresses {
		uuid, err := scyllaClient.GetLocalHostId(ctx, broadcastRPCAddress, false)
		if err != nil {
			return nil, nil, fmt.Errorf("can't get HostID for node with broadcast rpc address %q: %w", broadcastRPCAddress, err)
		}

		uuids = append(uuids, uuid)
	}

	return broadcastRPCAddresses, uuids, nil
}

func GetBroadcastRPCAddress(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, svc *corev1.Service) (string, error) {
	host := svc.Spec.ClusterIP

	if host == corev1.ClusterIPNone {
		pod, err := client.Pods(sc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sc.Namespace, svc.Name), err)
		}
		host = pod.Status.PodIP
		if len(host) < 1 {
			return "", fmt.Errorf("empty podIP of pod %q", naming.ManualRef(sc.Namespace, svc.Name))
		}
	}

	configClient, err := GetScyllaConfigClient(ctx, client, sc, host)
	if err != nil {
		return "", fmt.Errorf("can't create scylla config client with host %q: %w", host, err)
	}

	broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("can't get broadcast_rpc_address of host %q: %w", host, err)
	}

	return broadcastRPCAddress, nil
}

func GetNodesServiceAndPodIPs(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	serviceIPs, err := GetNodesServiceIPs(ctx, client, sc)
	if err != nil {
		return nil, fmt.Errorf("can't get nodes service IPs: %w", err)
	}

	podIPs, err := GetNodesPodIPs(ctx, client, sc)
	if err != nil {
		return nil, fmt.Errorf("can't get nodes pod IPs: %w", err)
	}

	ipAddresses := make([]string, 0, len(serviceIPs)+len(podIPs))
	ipAddresses = append(ipAddresses, serviceIPs...)
	ipAddresses = append(ipAddresses, podIPs...)
	return ipAddresses, nil
}

func GetNodesServiceIPs(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sc).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("can't get member services: %w", err)
	}

	var ipAddresses []string

	for _, svc := range serviceList.Items {
		if svc.Spec.ClusterIP != corev1.ClusterIPNone {
			ipAddresses = append(ipAddresses, svc.Spec.ClusterIP)
		}

		for _, ingressStatus := range svc.Status.LoadBalancer.Ingress {
			ipAddresses = append(ipAddresses, ingressStatus.IP)
		}
	}

	return ipAddresses, nil
}

func GetNodesPodIPs(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	clusterPods, err := client.Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ScyllaDBNodesPodsLabelsForScyllaCluster(sc)).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("can't get cluster pods: %w", err)
	}

	ipAddresses := make([]string, 0, len(clusterPods.Items))

	for _, pod := range clusterPods.Items {
		ipAddresses = append(ipAddresses, pod.Status.PodIP)
	}

	return ipAddresses, nil
}

func GetIdentityServiceIP(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) (string, error) {
	svcName := naming.IdentityServiceNameForScyllaCluster(sc)
	svc, err := client.Services(sc.Namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("can't get service %q: %w", svcName, err)
	}

	clusterIP := svc.Spec.ClusterIP
	if len(clusterIP) == 0 {
		return "", fmt.Errorf("internal error: member service doesn't have clusterIP assigned")
	}

	return clusterIP, nil
}

// GetManagerClient gets managerClient using IP address. E2E tests shouldn't rely on InCluster DNS.
func GetManagerClient(ctx context.Context, client corev1client.CoreV1Interface) (*managerclient.Client, error) {
	managerService, err := client.Services(naming.ScyllaManagerNamespace).Get(ctx, naming.ScyllaManagerServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if managerService.Spec.ClusterIP == corev1.ClusterIPNone {
		return nil, fmt.Errorf("service %s/%s doesn't have a ClusterIP", managerService.Namespace, managerService.Name)
	}
	apiAddress := (&url.URL{
		Scheme: "http",
		Host:   managerService.Spec.ClusterIP,
		Path:   "/api/v1",
	}).String()

	manager, err := managerclient.NewClient(apiAddress)
	if err != nil {
		return nil, fmt.Errorf("create manager client, %w", err)
	}

	return &manager, nil
}

func GetNodeName(sc *scyllav1.ScyllaCluster, idx int) string {
	return fmt.Sprintf(
		"%s-%s-%s-%d",
		sc.Name,
		sc.Spec.Datacenter.Name,
		sc.Spec.Datacenter.Racks[0].Name,
		idx,
	)
}

func GetMemberServiceSelector(sc *scyllav1.ScyllaCluster) labels.Selector {
	return labels.Set{
		naming.ClusterNameLabel:       sc.Name,
		naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
	}.AsSelector()
}

func WaitForFullMultiDCQuorum(ctx context.Context, dcClientMap map[string]corev1client.CoreV1Interface, scs []*scyllav1.ScyllaCluster) error {
	allHostIDs := map[string][]string{}
	var sortedAllHostIDs []string

	var errs []error
	for _, sc := range scs {
		client, ok := dcClientMap[sc.Spec.Datacenter.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("client is missing for ScyllaCluster %q", naming.ObjRef(sc)))
			continue
		}

		_, hostIDs, err := GetBroadcastRPCAddressesAndUUIDs(ctx, client, sc)
		if err != nil {
			return fmt.Errorf("can't get host IDs for ScyllaCluster %q: %w", naming.ObjRef(sc), err)
		}
		allHostIDs[sc.Spec.Datacenter.Name] = hostIDs
		sortedAllHostIDs = append(sortedAllHostIDs, hostIDs...)
	}
	err := errors.Join(errs...)
	if err != nil {
		return err
	}

	sort.Strings(sortedAllHostIDs)

	for _, sc := range scs {
		client, ok := dcClientMap[sc.Spec.Datacenter.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("client is missing for ScyllaCluster %q", naming.ObjRef(sc)))
			continue
		}

		err = waitForFullQuorum(ctx, client, sc, sortedAllHostIDs)
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

func waitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, sortedExpectedHostIDs []string) error {
	scyllaClient, hosts, err := GetScyllaClient(ctx, client, sc)
	if err != nil {
		return fmt.Errorf("can't get scylla client: %w", err)
	}
	defer scyllaClient.Close()

	// Wait for node status to propagate and reach consistency.
	// This can take a while so let's set a large enough timeout to avoid flakes.
	return apimachineryutilwait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		allSeeAllAsUN := true
		infoMessages := make([]string, 0, len(hosts))
		var errs []error
		for _, h := range hosts {
			s, err := scyllaClient.Status(ctx, h)
			if err != nil {
				return true, fmt.Errorf("can't get scylla status on node %q: %w", h, err)
			}

			sHostIDs := s.HostIDs()
			sort.Strings(sHostIDs)
			if !reflect.DeepEqual(sHostIDs, sortedExpectedHostIDs) {
				errs = append(errs, fmt.Errorf("node %q thinks the cluster consists of different nodes, got %s, expected %s", h, sHostIDs, sortedExpectedHostIDs))
			}

			downHosts := s.DownHostIDs()
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

func PodIsRunning(pod *corev1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		return true, nil
	case corev1.PodFailed, corev1.PodSucceeded:
		return false, fmt.Errorf("pod ran to completion")
	}
	return false, nil
}

func WaitUntilServingCertificateIsLive(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) error {
	servingCertSecret, err := client.Secrets(sc.Namespace).Get(ctx, fmt.Sprintf("%s-local-serving-certs", sc.Name), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get serving cert secret: %w", err)
	}
	servingCerts, err := ocrypto.DecodeCertificates(servingCertSecret.Data["tls.crt"])
	if err != nil {
		return fmt.Errorf("can't decode serving certificate: %w", err)
	}
	if len(servingCerts) != 1 {
		return fmt.Errorf("expected 1 serving certificate, got %d", len(servingCerts))
	}
	servingCert := servingCerts[0]

	adminClientSecret, err := client.Secrets(sc.Namespace).Get(ctx, fmt.Sprintf("%s-local-user-admin", sc.Name), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get client certificate secret: %w", err)
	}

	adminTLSCert, err := tls.X509KeyPair(adminClientSecret.Data["tls.crt"], adminClientSecret.Data["tls.key"])
	if err != nil {
		return fmt.Errorf("can't parse client certificate: %w", err)
	}

	servingCABundleConfigMap, err := client.ConfigMaps(sc.Namespace).Get(ctx, fmt.Sprintf("%s-local-serving-ca", sc.Name), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't get serving CA configmap: %w", err)
	}

	caBundleCerts, err := ocrypto.DecodeCertificates([]byte(servingCABundleConfigMap.Data["ca-bundle.crt"]))
	if err != nil {
		return fmt.Errorf("can't decode serving CA certificate: %w", err)
	}

	servingCAPool := x509.NewCertPool()
	for _, caCert := range caBundleCerts {
		servingCAPool.AddCert(caCert)
	}

	hosts, err := GetBroadcastRPCAddresses(ctx, client, sc)
	if err != nil {
		return fmt.Errorf("can't get v1.ScyllaDBDatacenter %q hosts: %w", naming.ObjRef(sc), err)
	}

	for _, host := range hosts {
		o.Eventually(func(eo o.Gomega) {
			serverCerts, err := GetServerTLSCertificates(fmt.Sprintf("%s:9142", host), &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: false,
				Certificates:       []tls.Certificate{adminTLSCert},
				RootCAs:            servingCAPool,
			})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(serverCerts).NotTo(o.BeEmpty())
			eo.Expect(serverCerts).To(o.HaveLen(1))

			eo.Expect(serverCerts[0].Raw).NotTo(o.BeEmpty())
			eo.Expect(serverCerts[0].Raw).To(o.Equal(servingCert.Raw))
		}).WithTimeout(5 * 60 * time.Second).WithPolling(1 * time.Second).Should(o.Succeed())
	}

	return nil
}

func WaitForScyllaOperatorConfigState(ctx context.Context, socClient scyllav1alpha1client.ScyllaOperatorConfigInterface, name string, options controllerhelpers.WaitForStateOptions, condition func(*scyllav1alpha1.ScyllaOperatorConfig) (bool, error), additionalConditions ...func(*scyllav1alpha1.ScyllaOperatorConfig) (bool, error)) (*scyllav1alpha1.ScyllaOperatorConfig, error) {
	return controllerhelpers.WaitForObjectState[*scyllav1alpha1.ScyllaOperatorConfig, *scyllav1alpha1.ScyllaOperatorConfigList](ctx, socClient, name, options, condition, additionalConditions...)
}

func WaitForScyllaOperatorConfigStatus(ctx context.Context, client scyllav1alpha1client.ScyllaOperatorConfigInterface, scyllaOperatorConfig *scyllav1alpha1.ScyllaOperatorConfig) (*scyllav1alpha1.ScyllaOperatorConfig, error) {
	waitCtx, waitCtxCancel := context.WithTimeoutCause(ctx, SyncTimeout, errors.New("exceeded sync timeout to update the status"))
	defer waitCtxCancel()
	return controllerhelpers.WaitForObjectState[*scyllav1alpha1.ScyllaOperatorConfig, *scyllav1alpha1.ScyllaOperatorConfigList](
		waitCtx,
		client,
		scyllaOperatorConfig.Name,
		controllerhelpers.WaitForStateOptions{
			TolerateDelete: false,
		},
		func(soc *scyllav1alpha1.ScyllaOperatorConfig) (bool, error) {
			if soc.UID != scyllaOperatorConfig.UID {
				return true, fmt.Errorf(
					"scyllaOperatorConfig %q with UID %q doesn't exist anymore (current UID=%q)",
					scyllaOperatorConfig.Name,
					scyllaOperatorConfig.UID,
					soc.UID,
				)
			}

			if soc.Status.ObservedGeneration == nil {
				return false, nil
			}

			if *soc.Status.ObservedGeneration < scyllaOperatorConfig.Generation {
				return false, nil
			}

			return true, nil
		},
	)
}

func IsScyllaClusterRegisteredWithManager(sc *scyllav1.ScyllaCluster) (bool, error) {
	return sc.Status.ManagerID != nil && len(*sc.Status.ManagerID) > 0, nil
}

func GetContainerReadinessMap(pod *corev1.Pod) map[string]bool {
	res := map[string]bool{}

	for _, statusSet := range [][]corev1.ContainerStatus{
		pod.Status.InitContainerStatuses,
		pod.Status.ContainerStatuses,
		pod.Status.EphemeralContainerStatuses,
	} {
		for _, cs := range statusSet {
			res[cs.Name] = cs.Ready
		}
	}

	return res
}
