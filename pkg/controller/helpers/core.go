package helpers

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/util/nodeaffinity"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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
		match, err := nodeaffinity.NewLazyErrorNodeSelector(pv.Spec.NodeAffinity.Required).Match(node)
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
