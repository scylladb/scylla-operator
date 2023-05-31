// Copyright (C) 2021 ScyllaDB

package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

func IsNodeConfigRolledOut(nc *scyllav1alpha1.NodeConfig) (bool, error) {
	cond := controllerhelpers.FindNodeConfigCondition(nc.Status.Conditions, scyllav1alpha1.NodeConfigReconciledConditionType)
	return nc.Status.ObservedGeneration >= nc.Generation &&
		cond != nil && cond.ObservedGeneration >= nc.Generation && cond.Status == corev1.ConditionTrue, nil
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

func IsNodeConfigDoneWithNodes(nodes []*corev1.Node) func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
	const (
		nodeAvailableConditionFormat   = "Node%sAvailable"
		nodeProgressingConditionFormat = "Node%sProgressing"
		nodeDegradedConditionFormat    = "Node%sDegraded"
	)

	return func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
		for _, node := range nodes {
			if nc.Status.ObservedGeneration < nc.Generation {
				return false, nil
			}
			if !controllerhelpers.IsNodeTuned(nc.Status.NodeStatuses, node.Name) {
				return false, nil
			}

			availableNodeConditionType := scyllav1alpha1.NodeConfigConditionType(fmt.Sprintf(nodeAvailableConditionFormat, node.Name))
			progressingNodeConditionType := scyllav1alpha1.NodeConfigConditionType(fmt.Sprintf(nodeProgressingConditionFormat, node.Name))
			degradedNodeConditionType := scyllav1alpha1.NodeConfigConditionType(fmt.Sprintf(nodeDegradedConditionFormat, node.Name))

			if !helpers.IsStatusNodeConfigConditionPresentAndTrue(nc.Status.Conditions, availableNodeConditionType, nc.Generation) {
				return false, nil
			}
			if !helpers.IsStatusNodeConfigConditionPresentAndFalse(nc.Status.Conditions, progressingNodeConditionType, nc.Generation) {
				return false, nil
			}
			if !helpers.IsStatusNodeConfigConditionPresentAndFalse(nc.Status.Conditions, degradedNodeConditionType, nc.Generation) {
				return false, nil
			}
		}
		return true, nil
	}
}

func IsNodeConfigDoneWithNodeTuningFunc(nodes []*corev1.Node) func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
	return func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
		for _, node := range nodes {
			if !controllerhelpers.IsNodeTuned(nc.Status.NodeStatuses, node.Name) {
				return false, nil
			}
		}
		return true, nil
	}
}

func RolloutTimeoutForScyllaCluster(sc *scyllav1.ScyllaCluster) time.Duration {
	return SyncTimeout + time.Duration(GetMemberCount(sc))*memberRolloutTimeout
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

type listerWatcher[ListObject runtime.Object] interface {
	List(context.Context, metav1.ListOptions) (ListObject, error)
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}

type WaitForStateOptions struct {
	TolerateDelete bool
}

func WaitForObjectState[Object, ListObject runtime.Object](ctx context.Context, client listerWatcher[ListObject], name string, options WaitForStateOptions, condition func(obj Object) (bool, error), additionalConditions ...func(obj Object) (bool, error)) (Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.Watch(ctx, options)
		},
	}

	conditions := make([]func(Object) (bool, error), 0, 1+len(additionalConditions))
	conditions = append(conditions, condition)
	if len(additionalConditions) != 0 {
		conditions = append(conditions, additionalConditions...)
	}
	aggregatedCond := func(obj Object) (bool, error) {
		allDone := true
		for _, c := range conditions {
			var err error
			var done bool

			done, err = c(obj)
			if err != nil {
				return done, err
			}
			if !done {
				allDone = false
			}
		}
		return allDone, nil
	}

	event, err := watchtools.UntilWithSync(ctx, lw, *new(Object), nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return aggregatedCond(event.Object.(Object))

		case watch.Deleted:
			if options.TolerateDelete {
				return aggregatedCond(event.Object.(Object))
			}
			fallthrough

		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return *new(Object), err
	}

	return event.Object.(Object), nil
}

func WaitForScyllaClusterState(ctx context.Context, client scyllav1client.ScyllaV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*scyllav1.ScyllaCluster) (bool, error), additionalConditions ...func(*scyllav1.ScyllaCluster) (bool, error)) (*scyllav1.ScyllaCluster, error) {
	return WaitForObjectState[*scyllav1.ScyllaCluster, *scyllav1.ScyllaClusterList](ctx, client.ScyllaClusters(namespace), name, options, condition, additionalConditions...)
}

func WaitForScyllaDBMonitoringState(ctx context.Context, client scyllav1alpha1client.ScyllaDBMonitoringInterface, name string, options WaitForStateOptions, condition func(monitoring *scyllav1alpha1.ScyllaDBMonitoring) (bool, error), additionalConditions ...func(monitoring *scyllav1alpha1.ScyllaDBMonitoring) (bool, error)) (*scyllav1alpha1.ScyllaDBMonitoring, error) {
	return WaitForObjectState[*scyllav1alpha1.ScyllaDBMonitoring, *scyllav1alpha1.ScyllaDBMonitoringList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForPodState(ctx context.Context, client corev1client.PodInterface, name string, options WaitForStateOptions, condition func(*corev1.Pod) (bool, error), additionalConditions ...func(*corev1.Pod) (bool, error)) (*corev1.Pod, error) {
	return WaitForObjectState[*corev1.Pod, *corev1.PodList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForServiceAccountState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*corev1.ServiceAccount) (bool, error), additionalConditions ...func(*corev1.ServiceAccount) (bool, error)) (*corev1.ServiceAccount, error) {
	return WaitForObjectState[*corev1.ServiceAccount, *corev1.ServiceAccountList](ctx, client.ServiceAccounts(namespace), name, options, condition, additionalConditions...)
}

func WaitForRoleBindingState(ctx context.Context, client rbacv1client.RbacV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*rbacv1.RoleBinding) (bool, error), additionalConditions ...func(*rbacv1.RoleBinding) (bool, error)) (*rbacv1.RoleBinding, error) {
	return WaitForObjectState[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctx, client.RoleBindings(namespace), name, options, condition, additionalConditions...)
}

func WaitForPVCState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, options WaitForStateOptions, condition func(*corev1.PersistentVolumeClaim) (bool, error), additionalConditions ...func(*corev1.PersistentVolumeClaim) (bool, error)) (*corev1.PersistentVolumeClaim, error) {
	return WaitForObjectState[*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaimList](ctx, client.PersistentVolumeClaims(namespace), name, options, condition, additionalConditions...)
}

func WaitForNodeConfigState(ctx context.Context, ncClient scyllav1alpha1client.NodeConfigInterface, name string, options WaitForStateOptions, condition func(*scyllav1alpha1.NodeConfig) (bool, error), additionalConditions ...func(*scyllav1alpha1.NodeConfig) (bool, error)) (*scyllav1alpha1.NodeConfig, error) {
	return WaitForObjectState[*scyllav1alpha1.NodeConfig, *scyllav1alpha1.NodeConfigList](ctx, ncClient, name, options, condition, additionalConditions...)
}

func WaitForConfigMapState(ctx context.Context, client corev1client.ConfigMapInterface, name string, options WaitForStateOptions, condition func(*corev1.ConfigMap) (bool, error), additionalConditions ...func(*corev1.ConfigMap) (bool, error)) (*corev1.ConfigMap, error) {
	return WaitForObjectState[*corev1.ConfigMap, *corev1.ConfigMapList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForSecretState(ctx context.Context, client corev1client.SecretInterface, name string, options WaitForStateOptions, condition func(*corev1.Secret) (bool, error), additionalConditions ...func(*corev1.Secret) (bool, error)) (*corev1.Secret, error) {
	return WaitForObjectState[*corev1.Secret, *corev1.SecretList](ctx, client, name, options, condition, additionalConditions...)
}

func WaitForServiceState(ctx context.Context, client corev1client.ServiceInterface, name string, options WaitForStateOptions, condition func(*corev1.Service) (bool, error), additionalConditions ...func(*corev1.Service) (bool, error)) (*corev1.Service, error) {
	return WaitForObjectState[*corev1.Service, *corev1.ServiceList](ctx, client, name, options, condition, additionalConditions...)
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

	return WaitForPodState(
		ctx,
		client,
		podName,
		WaitForStateOptions{},
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
		controllerRef := metav1.GetControllerOfNoCopy(&s)
		if controllerRef == nil {
			continue
		}

		if controllerRef.UID != sc.UID {
			continue
		}

		rackName := s.Labels[naming.RackNameLabel]
		res[rackName] = &s
	}

	return res, nil
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
	hosts, err := GetHosts(ctx, client, sc)
	if err != nil {
		return nil, nil, err
	}

	if len(hosts) < 1 {
		return nil, nil, fmt.Errorf("no services found")
	}

	tokenSecret, err := client.Secrets(sc.Namespace).Get(ctx, naming.AgentAuthTokenSecretName(sc.Name), metav1.GetOptions{})
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

func GetScyllaConfigClient(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, host string) (*scyllaclient.ConfigClient, error) {
	tokenSecret, err := client.Secrets(sc.Namespace).Get(ctx, naming.AgentAuthTokenSecretName(sc.Name), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get Secret %q: %w", naming.ManualRef(sc.Namespace, naming.AgentAuthTokenSecretName(sc.Name)), err)
	}

	authToken, err := helpers.GetAgentAuthTokenFromSecret(tokenSecret)
	if err != nil {
		return nil, fmt.Errorf("can't get auth token: %w", err)
	}

	configClient := scyllaclient.NewConfigClient(host, authToken)
	return configClient, nil
}

func GetHostsAndUUIDs(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, []string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sc.Name).String(),
	})
	if err != nil {
		return nil, nil, err
	}

	var hosts []string
	var uuids []string
	for _, s := range serviceList.Items {
		host := s.Spec.ClusterIP

		if host == corev1.ClusterIPNone {
			pod, err := client.Pods(sc.Namespace).Get(ctx, s.Name, metav1.GetOptions{})
			if err != nil {
				return nil, nil, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sc.Namespace, s.Name), err)
			}
			host = pod.Status.PodIP
		}

		configClient, err := GetScyllaConfigClient(ctx, client, sc, host)
		if err != nil {
			return nil, nil, fmt.Errorf("can't create scylla config client with host %q: %w", host, err)
		}
		clientBroadcastedAddress, err := configClient.BroadcastRPCAddress(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("can't get broadcast_rpc_address of host %q: %w", host, err)
		}

		hosts = append(hosts, clientBroadcastedAddress)
		uuids = append(uuids, s.Annotations[naming.HostIDAnnotation])
	}

	return hosts, uuids, nil
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
		LabelSelector: GetMemberServiceSelector(sc.Name).String(),
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
		LabelSelector: naming.ClusterSelector(sc).String(),
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

func GetHosts(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	hosts, _, err := GetHostsAndUUIDs(ctx, client, sc)
	return hosts, err
}

// GetManagerClient gets managerClient using IP address. E2E tests shouldn't rely on InCluster DNS.
func GetManagerClient(ctx context.Context, client corev1client.CoreV1Interface) (*mermaidclient.Client, error) {
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

	manager, err := mermaidclient.NewClient(apiAddress, &http.Transport{})
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

func GetMemberServiceSelector(scyllaClusterName string) labels.Selector {
	return labels.Set{
		naming.ClusterNameLabel:       scyllaClusterName,
		naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
	}.AsSelector()
}

func GetScyllaHostsAndWaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	dcClientMap := make(map[string]corev1client.CoreV1Interface, 1)
	dcClientMap[sc.Spec.Datacenter.Name] = client
	hosts, err := GetScyllaHostsByDCAndWaitForFullQuorum(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc})
	if err != nil {
		return nil, err
	}
	return slices.Flatten(helpers.GetMapValues(hosts)), nil
}

func GetScyllaHostsByDCAndWaitForFullQuorum(ctx context.Context, dcClientMap map[string]corev1client.CoreV1Interface, scs []*scyllav1.ScyllaCluster) (map[string][]string, error) {
	allHosts := map[string][]string{}
	var sortedAllHosts []string

	var errs []error
	for _, sc := range scs {
		client, ok := dcClientMap[sc.Spec.Datacenter.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("client is missing for datacenter %q of ScyllaCluster %q", sc.Spec.Datacenter.Name, naming.ObjRef(sc)))
			continue
		}

		hosts, err := GetHosts(ctx, client, sc)
		if err != nil {
			return nil, fmt.Errorf("can't get hosts for ScyllaCluster %q: %w", sc.Name, err)
		}
		allHosts[sc.Spec.Datacenter.Name] = hosts
		sortedAllHosts = append(sortedAllHosts, hosts...)
	}
	err := errors.Join(errs...)
	if err != nil {
		return nil, err
	}

	sort.Strings(sortedAllHosts)

	for _, sc := range scs {
		client, ok := dcClientMap[sc.Spec.Datacenter.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("client is missing for datacenter %q of ScyllaCluster %q", sc.Spec.Datacenter.Name, naming.ObjRef(sc)))
			continue
		}

		err = waitForFullQuorum(ctx, client, sc, sortedAllHosts)
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return nil, fmt.Errorf("can't wait for scylla nodes to reach status consistency: %w", err)
	}

	framework.Infof("ScyllaDB nodes have reached status consistency.")

	return allHosts, nil
}

func waitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster, sortedExpectedHosts []string) error {
	scyllaClient, hosts, err := GetScyllaClient(ctx, client, sc)
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

	hosts, err := GetHosts(ctx, client, sc)
	if err != nil {
		return fmt.Errorf("can't get v1.ScyllaCluster %q hosts: %w", naming.ObjRef(sc), err)
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
