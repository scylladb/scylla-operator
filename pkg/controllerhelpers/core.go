package controllerhelpers

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1schedulinghelpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
)

func GetPodCondition(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) *corev1.PodCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsPodReady(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return false
	}

	condition := GetPodCondition(pod.Status.Conditions, corev1.PodReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsPodReadyWithPositiveLiveCheck(ctx context.Context, client corev1client.PodsGetter, pod *corev1.Pod) (bool, *corev1.Pod, error) {
	if !IsPodReady(pod) {
		return false, pod, nil
	}

	// Verify readiness with a live call.
	fresh, err := client.Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return false, pod, err
	}

	return IsPodReady(fresh), fresh, nil
}

func GetNodePointerArrayFromArray(nodes []corev1.Node) []*corev1.Node {
	res := make([]*corev1.Node, 0, len(nodes))
	for _, node := range nodes {
		res = append(res, &node)
	}

	return res
}

func IsOrphanedPV(pv *corev1.PersistentVolume, nodes []*corev1.Node) (bool, error) {
	if pv.Spec.NodeAffinity == nil {
		klog.V(4).InfoS("PV doesn't have nodeAffinity", "PV", klog.KObj(pv))
		return false, nil
	}

	for _, node := range nodes {
		match, err := corev1schedulinghelpers.MatchNodeSelectorTerms(node, pv.Spec.NodeAffinity.Required)
		if err != nil {
			return false, err
		}

		if match {
			klog.V(4).InfoS("PV is bound to an existing node", "PV", klog.KObj(pv), "Node", klog.KObj(node))
			return false, nil
		}
	}

	return true, nil
}

func FindContainerStatus(pod *corev1.Pod, containerName string) *corev1.ContainerStatus {
	for _, statusSet := range [][]corev1.ContainerStatus{
		pod.Status.InitContainerStatuses,
		pod.Status.ContainerStatuses,
		pod.Status.EphemeralContainerStatuses,
	} {
		for _, cs := range statusSet {
			if cs.Name == containerName {
				return &cs
			}
		}
	}

	return nil
}

func FindScyllaContainerStatus(pod *corev1.Pod) *corev1.ContainerStatus {
	return FindContainerStatus(pod, naming.ScyllaContainerName)
}

func IsScyllaContainerRunning(pod *corev1.Pod) bool {
	cs := FindScyllaContainerStatus(pod)
	if cs == nil {
		return false
	}

	return cs.State.Running != nil
}

func GetScyllaContainerID(pod *corev1.Pod) (string, error) {
	cs := FindScyllaContainerStatus(pod)
	if cs == nil {
		return "", fmt.Errorf("no scylla container found in pod %q", naming.ObjRef(pod))
	}

	return cs.ContainerID, nil
}
