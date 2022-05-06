// Copyright (C) 2021 ScyllaDB

package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	o "github.com/onsi/gomega"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/mermaidclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		cond != nil && cond.Status == corev1.ConditionTrue, nil
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
	return baseRolloutTimout + time.Duration(GetMemberCount(sc))*memberRolloutTimeout
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

func IsScyllaClusterRolledOut(sc *scyllav1.ScyllaCluster) (bool, error) {
	// ObservedGeneration == nil will filter out the case when the object is initially created
	// so no other optional (but required) field should be nil after this point and we should error out.
	if sc.Status.ObservedGeneration == nil || *sc.Status.ObservedGeneration < sc.Generation {
		return false, nil
	}

	// TODO: this should be more straight forward - we need better status (conditions, aggregated state, ...)

	for _, r := range sc.Spec.Datacenter.Racks {
		rackStatus, found := sc.Status.Racks[r.Name]
		if !found {
			return false, nil
		}

		if rackStatus.Stale == nil {
			return true, fmt.Errorf("stale shouldn't be nil")
		}

		if *rackStatus.Stale {
			return false, nil
		}

		if rackStatus.Members != r.Members {
			return false, nil
		}

		if rackStatus.ReadyMembers != r.Members {
			return false, nil
		}

		if rackStatus.UpdatedMembers == nil {
			return true, fmt.Errorf("updatedMembers shouldn't be nil")
		}

		if *rackStatus.UpdatedMembers != r.Members {
			return false, nil
		}

		if rackStatus.Version != sc.Spec.Version {
			return false, nil
		}
	}

	if sc.Status.Upgrade != nil && sc.Status.Upgrade.FromVersion != sc.Status.Upgrade.ToVersion {
		return false, nil
	}

	framework.Infof("ScyllaCluster %s (RV=%s) is rolled out", klog.KObj(sc), sc.ResourceVersion)

	// Allow CI to remove stale conntrack entries.
	// https://github.com/scylladb/scylla-operator/issues/975
	// TODO: remove when GitHub Actions upgrades to kernel version 5.14.
	time.Sleep(5 * time.Second)

	return true, nil
}

func WaitForScyllaClusterState(ctx context.Context, client scyllav1client.ScyllaV1Interface, namespace string, name string, conditions ...func(sc *scyllav1.ScyllaCluster) (bool, error)) (*scyllav1.ScyllaCluster, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.ScyllaClusters(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.ScyllaClusters(namespace).Watch(ctx, options)
		},
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &scyllav1.ScyllaCluster{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			for _, condition := range conditions {
				done, err := condition(event.Object.(*scyllav1.ScyllaCluster))
				if err != nil {
					return false, err
				}
				if !done {
					return false, nil
				}
			}
			return true, nil
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*scyllav1.ScyllaCluster), nil
}

type WaitForStateOptions struct {
	TolerateDelete bool
}

func WaitForPodState(ctx context.Context, client corev1client.PodInterface, name string, condition func(sc *corev1.Pod) (bool, error), o WaitForStateOptions) (*corev1.Pod, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*corev1.Pod))
		case watch.Deleted:
			if o.TolerateDelete {
				return condition(event.Object.(*corev1.Pod))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.Pod), nil
}

func WaitForServiceAccountState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, condition func(sc *corev1.ServiceAccount) (bool, error), o WaitForStateOptions) (*corev1.ServiceAccount, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.ServiceAccounts(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.ServiceAccounts(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.ServiceAccount{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*corev1.ServiceAccount))
		case watch.Deleted:
			if o.TolerateDelete {
				return condition(event.Object.(*corev1.ServiceAccount))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.ServiceAccount), nil
}

func WaitForRoleBindingState(ctx context.Context, client rbacv1client.RbacV1Interface, namespace string, name string, condition func(sc *rbacv1.RoleBinding) (bool, error), o WaitForStateOptions) (*rbacv1.RoleBinding, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.RoleBindings(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.RoleBindings(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &rbacv1.RoleBinding{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*rbacv1.RoleBinding))
		case watch.Deleted:
			if o.TolerateDelete {
				return condition(event.Object.(*rbacv1.RoleBinding))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*rbacv1.RoleBinding), nil
}

func WaitForPVCState(ctx context.Context, client corev1client.CoreV1Interface, namespace string, name string, condition func(sc *corev1.PersistentVolumeClaim) (bool, error), o WaitForStateOptions) (*corev1.PersistentVolumeClaim, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.PersistentVolumeClaims(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.PersistentVolumeClaims(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.PersistentVolumeClaim{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return condition(event.Object.(*corev1.PersistentVolumeClaim))
		case watch.Deleted:
			if o.TolerateDelete {
				return condition(event.Object.(*corev1.PersistentVolumeClaim))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.PersistentVolumeClaim), nil
}

func WaitForNodeConfigState(ctx context.Context, ncClient scyllav1alpha1client.NodeConfigInterface, name string, o WaitForStateOptions, condition func(sc *scyllav1alpha1.NodeConfig) (bool, error), additionalConditions ...func(sc *scyllav1alpha1.NodeConfig) (bool, error)) (*scyllav1alpha1.NodeConfig, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return ncClient.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return ncClient.Watch(ctx, options)
		},
	}

	conditions := make([]func(sc *scyllav1alpha1.NodeConfig) (bool, error), 0, 1+len(additionalConditions))
	conditions = append(conditions, condition)
	if len(additionalConditions) != 0 {
		conditions = append(conditions, additionalConditions...)
	}
	aggregatedCond := func(nc *scyllav1alpha1.NodeConfig) (bool, error) {
		allDone := true
		for _, c := range conditions {
			var err error
			var done bool

			done, err = c(nc)
			if err != nil {
				return done, err
			}
			if !done {
				allDone = false
			}
		}
		return allDone, nil
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &scyllav1alpha1.NodeConfig{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return aggregatedCond(event.Object.(*scyllav1alpha1.NodeConfig))
		case watch.Deleted:
			if o.TolerateDelete {
				return aggregatedCond(event.Object.(*scyllav1alpha1.NodeConfig))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*scyllav1alpha1.NodeConfig), nil
}

func WaitForConfigMapState(ctx context.Context, cmClient corev1client.ConfigMapInterface, name string, o WaitForStateOptions, condition func(cm *corev1.ConfigMap) (bool, error), additionalConditions ...func(cm *corev1.ConfigMap) (bool, error)) (*corev1.ConfigMap, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return cmClient.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return cmClient.Watch(ctx, options)
		},
	}

	conditions := make([]func(sc *corev1.ConfigMap) (bool, error), 0, 1+len(additionalConditions))
	conditions = append(conditions, condition)
	if len(additionalConditions) != 0 {
		conditions = append(conditions, additionalConditions...)
	}
	aggregatedCond := func(nc *corev1.ConfigMap) (bool, error) {
		allDone := true
		for _, c := range conditions {
			var err error
			var done bool

			done, err = c(nc)
			if err != nil {
				return done, err
			}
			if !done {
				allDone = false
			}
		}
		return allDone, nil
	}

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.ConfigMap{}, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return aggregatedCond(event.Object.(*corev1.ConfigMap))
		case watch.Deleted:
			if o.TolerateDelete {
				return aggregatedCond(event.Object.(*corev1.ConfigMap))
			}
			fallthrough
		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.ConfigMap), nil
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
		return nil, fmt.Errorf("can't patch pod %q to add ephemeral container: %w", podName, err)
	}

	return WaitForPodState(
		ctx,
		client,
		podName,
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
		WaitForStateOptions{},
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

	// TODO: unify logging
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.InfoLevel),
	})
	cfg := scyllaclient.DefaultConfig(authToken, hosts...)

	scyllaClient, err := scyllaclient.NewClient(cfg, logger.Named("scylla_client"))
	if err != nil {
		return nil, nil, err
	}

	return scyllaClient, hosts, nil
}

func GetHosts(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) ([]string, error) {
	serviceList, err := client.Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: GetMemberServiceSelector(sc.Name).String(),
	})
	if err != nil {
		return nil, err
	}

	var hosts []string
	for _, s := range serviceList.Items {
		if s.Spec.Type != corev1.ServiceTypeClusterIP {
			return nil, fmt.Errorf("service %s/%s is of type %q instead of %q", s.Namespace, s.Name, s.Spec.Type, corev1.ServiceTypeClusterIP)
		}

		if s.Spec.ClusterIP == corev1.ClusterIPNone {
			return nil, fmt.Errorf("service %s/%s doesn't have a ClusterIP", s.Namespace, s.Name)
		}

		hosts = append(hosts, s.Spec.ClusterIP)
	}

	return hosts, nil
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
