// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller/daemon/util"
	pluginhelper "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/scheduler/framework/plugins/helper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
)

// setCondition updates the status to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func setCondition(status *scyllav1alpha1.ScyllaNodeConfigStatus, condition scyllav1alpha1.Condition) {
	currentCond := getCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// newCondition creates a new condition.
func newCondition(condType scyllav1alpha1.ConditionType, status corev1.ConditionStatus, reason, message string) scyllav1alpha1.Condition {
	return scyllav1alpha1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// filterOutCondition returns a new slice of conditions without conditions with the provided type.
func filterOutCondition(conditions []scyllav1alpha1.Condition, condType scyllav1alpha1.ConditionType) []scyllav1alpha1.Condition {
	var newConditions []scyllav1alpha1.Condition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// getCondition returns the condition with the provided type.
func getCondition(status scyllav1alpha1.ScyllaNodeConfigStatus, condType scyllav1alpha1.ConditionType) *scyllav1alpha1.Condition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func nodeConfigAvailable(ds *appsv1.DaemonSet, snc *scyllav1alpha1.ScyllaNodeConfig) bool {
	return ds.Status.ObservedGeneration == ds.Generation &&
		snc.Status.Updated.Desired == snc.Status.Updated.Actual &&
		snc.Status.Updated.Actual == snc.Status.Updated.Ready
}

func nodeIsTargetedByDaemonSet(node *corev1.Node, ds *appsv1.DaemonSet) bool {
	shouldRun, shouldContinueRunning := nodeShouldRunDaemonPod(node, ds)
	return shouldRun || shouldContinueRunning
}

// nodeShouldRunDaemonPod checks a set of preconditions against a (node,daemonset) and returns a
// summary. Returned booleans are:
// * shouldRun:
//     Returns true when a daemonset should run on the node if a daemonset pod is not already
//     running on that node.
// * shouldContinueRunning:
//     Returns true when a daemonset should continue running on a node if a daemonset pod is already
//     running on that node.
func nodeShouldRunDaemonPod(node *corev1.Node, ds *appsv1.DaemonSet) (bool, bool) {
	pod := newDaemonSetPod(ds, node.Name)

	// If the daemon set specifies a node name, check that it matches with node.Name.
	if !(ds.Spec.Template.Spec.NodeName == "" || ds.Spec.Template.Spec.NodeName == node.Name) {
		return false, false
	}

	fitsNodeName, fitsNodeAffinity, fitsTaints := canPodRunOnNode(pod, node)
	if !fitsNodeName || !fitsNodeAffinity {
		return false, false
	}

	if !fitsTaints {
		// Scheduled daemon pods should continue running if they tolerate NoExecute taint.
		_, hasUntoleratedTaint := corev1helpers.FindMatchingUntoleratedTaint(node.Spec.Taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
			return t.Effect == corev1.TaintEffectNoExecute
		})
		return false, !hasUntoleratedTaint
	}

	return true, true
}

// Predicates checks if a pod can run on a node.
func canPodRunOnNode(pod *corev1.Pod, node *corev1.Node) (fitsNodeName, fitsNodeAffinity, fitsTaints bool) {
	fitsNodeName = len(pod.Spec.NodeName) == 0 || pod.Spec.NodeName == node.Name
	fitsNodeAffinity = pluginhelper.PodMatchesNodeSelectorAndAffinityTerms(pod, node)
	_, hasUntoleratedTaint := corev1helpers.FindMatchingUntoleratedTaint(node.Spec.Taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute || t.Effect == corev1.TaintEffectNoSchedule
	})
	fitsTaints = !hasUntoleratedTaint
	return
}

// newDaemonSetPod creates a new pod
func newDaemonSetPod(ds *appsv1.DaemonSet, nodeName string) *corev1.Pod {
	newPod := &corev1.Pod{Spec: ds.Spec.Template.Spec, ObjectMeta: ds.Spec.Template.ObjectMeta}
	newPod.Namespace = ds.Namespace
	newPod.Spec.NodeName = nodeName

	// Added default tolerations for DaemonSet pods.
	util.AddOrUpdateDaemonPodTolerations(&newPod.Spec)

	return newPod
}
