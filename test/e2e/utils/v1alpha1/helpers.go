// Copyright (c) 2022 ScyllaDB.

package v1alpha1

import (
	"context"
	"fmt"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	appv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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

func GetStatefulSetsForScyllaDatacenter(ctx context.Context, client appv1client.AppsV1Interface, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*appsv1.StatefulSet, error) {
	statefulsetList, err := client.StatefulSets(sd.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			naming.ClusterNameLabel: sd.Name,
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

		if controllerRef.UID != sd.UID {
			continue
		}

		rackName := s.Labels[naming.RackNameLabel]
		res[rackName] = &s
	}

	return res, nil
}

func WaitForNodeConfigState(ctx context.Context, ncClient v1alpha1.NodeConfigInterface, name string, options utils.WaitForStateOptions, condition func(*scyllav1alpha1.NodeConfig) (bool, error), additionalConditions ...func(*scyllav1alpha1.NodeConfig) (bool, error)) (*scyllav1alpha1.NodeConfig, error) {
	return utils.WaitForObjectState[*scyllav1alpha1.NodeConfig, *scyllav1alpha1.NodeConfigList](ctx, ncClient, name, options, condition, additionalConditions...)
}

func WaitForScyllaDatacenterState(ctx context.Context, client v1alpha1.ScyllaV1alpha1Interface, namespace string, name string, options utils.WaitForStateOptions, condition func(datacenter *scyllav1alpha1.ScyllaDatacenter) (bool, error), additionalConditions ...func(datacenter *scyllav1alpha1.ScyllaDatacenter) (bool, error)) (*scyllav1alpha1.ScyllaDatacenter, error) {
	return utils.WaitForObjectState[*scyllav1alpha1.ScyllaDatacenter, *scyllav1alpha1.ScyllaDatacenterList](ctx, client.ScyllaDatacenters(namespace), name, options, condition, additionalConditions...)
}
