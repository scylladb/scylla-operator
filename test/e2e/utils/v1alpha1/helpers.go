// Copyright (c) 2024 ScyllaDB.

package v1alpha1

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclientset "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
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

func GetBroadcastRPCAddressesForScyllaDBCluster(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *scyllav1alpha1.ScyllaDBCluster) (map[string][]string, error) {
	broadcastRPCAddressesMap, err := collectFromEachDatacenter(ctx, rkcClusterMap, sc, GetBroadcastRPCAddresses)
	if err != nil {
		return nil, fmt.Errorf("can't collect broadcast rpc addresses datacenters of ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	return broadcastRPCAddressesMap, nil
}

func GetHostIDsForScyllaDBCluster(ctx context.Context, rkcClusterMap map[string]framework.ClusterInterface, sc *scyllav1alpha1.ScyllaDBCluster) (map[string][]string, error) {
	hostIDsMap, err := collectFromEachDatacenter(ctx, rkcClusterMap, sc, GetHostIDs)
	if err != nil {
		return nil, fmt.Errorf("can't collect host IDs from datacenters of ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	return hostIDsMap, nil
}

func collectFromEachDatacenter[T any](
	ctx context.Context,
	rkcClusterMap map[string]framework.ClusterInterface,
	sc *scyllav1alpha1.ScyllaDBCluster,
	f func(context.Context, corev1client.CoreV1Interface, *scyllav1alpha1.ScyllaDBDatacenter) ([]T, error),
) (map[string][]T, error) {
	valuesMap := make(map[string][]T, len(sc.Spec.Datacenters))

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		clusterClient, ok := rkcClusterMap[dc.RemoteKubernetesClusterName]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster client is missing for datacenter %q of ScyllaDBCluster %q", dc.Name, naming.ObjRef(sc)))
			continue
		}

		remoteNamespaceName, err := naming.RemoteNamespaceName(sc, &dc)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get remote namespace name for datacenter %q of ScyllaDBCluster %q: %w", dc.Name, naming.ObjRef(sc), err))
			continue
		}

		sdcName := naming.ScyllaDBDatacenterName(sc, &dc)
		sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(remoteNamespaceName).Get(ctx, sdcName, metav1.GetOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", sdcName, err))
			continue
		}

		values, err := f(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't collect from ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err))
			continue
		}

		valuesMap[dc.Name] = values
	}
	err := errors.Join(errs...)
	if err != nil {
		return nil, err
	}

	return valuesMap, nil
}

func collectFromEachNode[T any](ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, f func(*corev1.Pod, *corev1.Service) (T, error)) ([]T, error) {
	serviceList, err := client.Services(sdc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sdc).String(),
	})
	if err != nil {
		return nil, err
	}

	var values []T
	for _, svc := range serviceList.Items {
		podName := naming.PodNameFromService(&svc)
		pod, err := client.Pods(sdc.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
		}

		v, err := f(pod, &svc)
		if err != nil {
			return nil, fmt.Errorf("can't get broadcast rpc address for service %q: %w", naming.ObjRef(&svc), err)
		}

		values = append(values, v)
	}

	return values, nil
}

func GetBroadcastRPCAddresses(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]string, error) {
	broadcastRPCAddresses, err := collectFromEachNode(ctx, client, sdc, func(pod *corev1.Pod, service *corev1.Service) (string, error) {
		return GetBroadcastRPCAddress(ctx, client, sdc, service)
	})
	if err != nil {
		return nil, fmt.Errorf("can't get broadcast rpc addresses: %w", err)
	}

	return broadcastRPCAddresses, nil
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

func GetHostIDs(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]string, error) {
	hostIDs, err := collectFromEachNode(ctx, client, sdc, func(pod *corev1.Pod, svc *corev1.Service) (string, error) {
		return GetHostID(ctx, client, sdc, svc, pod)
	})
	if err != nil {
		return nil, fmt.Errorf("can't get host IDs: %w", err)
	}

	return hostIDs, nil
}

func GetBroadcastAddresses(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]string, error) {
	broadcastAddresses, err := collectFromEachNode(ctx, client, sdc, func(pod *corev1.Pod, svc *corev1.Service) (string, error) {
		return GetBroadcastAddress(ctx, client, sdc, svc, pod)
	})
	if err != nil {
		return nil, fmt.Errorf("can't get broadcast addresses: %w", err)
	}

	return broadcastAddresses, nil
}

func GetHostID(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service, pod *corev1.Pod) (string, error) {
	scyllaClient, _, err := GetScyllaClient(ctx, client, sdc)
	if err != nil {
		return "", fmt.Errorf("can't get scylla client: %w", err)
	}

	host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
	if err != nil {
		return "", fmt.Errorf("can't get Scylla host for Service %q: %w", naming.ObjRef(svc), err)
	}

	hostID, err := scyllaClient.GetLocalHostId(ctx, host, false)
	if err != nil {
		return "", fmt.Errorf("can't get hots ID from host %q: %w", host, err)
	}

	return hostID, nil
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

func ContextForRollout(parent context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, RolloutTimeoutForScyllaDBDatacenter(sdc))
}

func RolloutTimeoutForScyllaDBDatacenter(sdc *scyllav1alpha1.ScyllaDBDatacenter) time.Duration {
	return SyncTimeout + time.Duration(GetNodeCount(sdc))*memberRolloutTimeout + cleanupJobTimeout
}

func GetNodeCount(sdc *scyllav1alpha1.ScyllaDBDatacenter) int32 {
	nodes := int32(0)
	rackTemplateNodes := int32(0)

	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.Nodes != nil {
		rackTemplateNodes = *sdc.Spec.RackTemplate.Nodes
	}

	for _, r := range sdc.Spec.Racks {
		if r.Nodes != nil {
			nodes += *r.Nodes
		} else {
			nodes += rackTemplateNodes
		}
	}

	return nodes
}

func IsScyllaDBDatacenterRolledOut(sdc *scyllav1alpha1.ScyllaDBDatacenter) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(sdc.Status.Conditions, scyllav1.AvailableCondition, sdc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sdc.Status.Conditions, scyllav1.ProgressingCondition, sdc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sdc.Status.Conditions, scyllav1.DegradedCondition, sdc.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaDBDatacenter %s (RV=%s) is rolled out", klog.KObj(sdc), sdc.ResourceVersion)

	return true, nil
}

func IsScyllaDBManagerClusterRegistrationRolledOut(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (bool, error) {
	if !helpers.IsStatusConditionPresentAndFalse(smcr.Status.Conditions, scyllav1.ProgressingCondition, smcr.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(smcr.Status.Conditions, scyllav1.DegradedCondition, smcr.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaDBManagerClusterRegistration %s (RV=%s) is rolled out", klog.KObj(smcr), smcr.ResourceVersion)

	return true, nil
}

func GetStatefulSetsForScyllaDBDatacenter(ctx context.Context, client appv1client.AppsV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) (map[string]*appsv1.StatefulSet, error) {
	statefulsetList, err := client.StatefulSets(sdc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.ClusterNameLabel: sdc.Name,
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
	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("can't convert StatefulSet %q selector: %w", naming.ObjRef(sts), err)
	}

	podList, err := client.Pods(sts.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("can't list Pods for StatefulSet %q: %w", naming.ObjRef(sts), err)
	}

	res := map[string]*corev1.Pod{}
	for _, pod := range podList.Items {
		res[pod.Name] = &pod
	}

	return res, nil
}

// TODO: Should be unified with function coming from test/helpers once e2e's there starts using ScyllaDBDatacenter API.
func WaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, sortedExpectedHostIDs []string) error {
	scyllaClient, hosts, err := GetScyllaClient(ctx, client, sdc)
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
			var s scyllaclient.NodeStatusInfoSlice
			s, err = scyllaClient.NodesStatusInfo(ctx, h)
			if err != nil {
				return true, fmt.Errorf("can't get scylla status on node %q: %w", h, err)
			}

			sHostIDs := s.HostIDs()
			sort.Strings(sHostIDs)
			if !reflect.DeepEqual(sHostIDs, sortedExpectedHostIDs) {
				errs = append(errs, fmt.Errorf("node %q thinks the cluster consists of different nodes: got %s, expected %s", h, sHostIDs, sortedExpectedHostIDs))
			}

			downHosts := s.DownHostIDs()
			infoMessages = append(infoMessages, fmt.Sprintf("Node %q, down: %q, up: %q", h, strings.Join(downHosts, "\n"), strings.Join(s.UpHostIDs(), ",")))

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

func GetRemoteDatacenterScyllaConfigClient(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteScyllaAdminClient *scyllaclientset.Clientset, remoteKubeAdminClient *kubernetes.Clientset, agentAuthToken string) (*scyllaclient.ConfigClient, error) {
	dcStatus, _, ok := oslices.Find(sc.Status.Datacenters, func(dcStatus scyllav1alpha1.ScyllaDBClusterDatacenterStatus) bool {
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

func IsScyllaDBManagerTaskRolledOut(smt *scyllav1alpha1.ScyllaDBManagerTask) (bool, error) {
	if !helpers.IsStatusConditionPresentAndFalse(smt.Status.Conditions, scyllav1.ProgressingCondition, smt.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(smt.Status.Conditions, scyllav1.DegradedCondition, smt.Generation) {
		return false, nil
	}

	framework.Infof("ScyllaDBManagerTask %s (RV=%s) is rolled out", klog.KObj(smt), smt.ResourceVersion)

	return true, nil
}

func ScyllaDBManagerTaskHasDeletionFinalizer(smt *scyllav1alpha1.ScyllaDBManagerTask) (bool, error) {
	return slices.Contains(smt.Finalizers, naming.ScyllaDBManagerTaskFinalizer), nil
}
