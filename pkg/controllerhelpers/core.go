package controllerhelpers

import (
	"context"
	"fmt"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	outilerrors "github.com/scylladb/scylla-operator/pkg/util/errors"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// FindStatusConditionsWithSuffix finds all conditions that end with the suffix, except the identity.
func FindStatusConditionsWithSuffix(conditions []metav1.Condition, suffix string) []metav1.Condition {
	var res []metav1.Condition

	suffixLen := len(suffix)
	for _, c := range conditions {
		// Filter out identity and optimize filtering out shorter strings.
		if len(c.Type) <= suffixLen {
			continue
		}

		if strings.HasSuffix(c.Type, suffix) {
			res = append(res, c)
		}
	}

	return res
}

const (
	// https://github.com/kubernetes/apimachinery/blob/f7c43800319c674eecce7c80a6ac7521a9b50aa8/pkg/apis/meta/v1/types.go#L1640
	maxReasonLength = 1024
	// https://github.com/kubernetes/apimachinery/blob/f7c43800319c674eecce7c80a6ac7521a9b50aa8/pkg/apis/meta/v1/types.go#L1648
	maxMessageLength = 32768

	messageOmissionIndicator = "(%d more omitted)"
	reasonOmissionIndicator  = "And%dMoreOmitted"
)

// aggregateStatusConditionInfo aggregates status conditions reasons and messages into a single reason and message.
// It ensures that the length of the reason and message does not exceed the maximum allowed length.
func aggregateStatusConditionInfo(conditions []metav1.Condition) (string, string) {
	reasons := make([]string, 0, len(conditions))
	messages := make([]string, 0, len(conditions))

	for _, c := range conditions {
		reasons = append(reasons, c.Reason)

		for _, line := range strings.Split(c.Message, "\n") {
			messages = append(messages, fmt.Sprintf("%s: %s", c.Type, line))
		}
	}

	return joinWithLimit(reasons, joinWithLimitOptions{
			separator:            ",",
			limit:                maxReasonLength,
			omissionIndicatorFmt: reasonOmissionIndicator,
		}),
		joinWithLimit(messages, joinWithLimitOptions{
			separator:            "\n",
			limit:                maxMessageLength,
			omissionIndicatorFmt: messageOmissionIndicator,
		})
}

type joinWithLimitOptions struct {
	separator            string
	limit                int
	omissionIndicatorFmt string
}

// joinWithLimit joins the elements of the slice into a single string, separated by the specified separator.
// If the length of the resulting string exceeds the specified limit, it omits the remaining elements and appends an
// indication of how many were omitted.
func joinWithLimit(elems []string, opts joinWithLimitOptions) string {
	var joined strings.Builder
	for i, elem := range elems {
		sep := ""
		if i > 0 {
			sep = opts.separator
		}

		var (
			currentLen            = joined.Len() + len(sep) + len(elem)
			nextOmissionIndicator = opts.separator + fmt.Sprintf(opts.omissionIndicatorFmt, len(elems)-i-1)
		)

		var (
			// Check if adding the current element and the potential next omission indicator exceeds the limit.
			elemWithNextOmissionExceedsLimit = currentLen+len(nextOmissionIndicator) > opts.limit
			// Check if adding just the last element fits within the limit.
			lastAndFits = currentLen <= opts.limit && i == len(elems)-1
		)
		if elemWithNextOmissionExceedsLimit && !lastAndFits {
			omissionSep := ""
			if i > 0 {
				omissionSep = opts.separator
			}
			// We're safe to add the current omission indicator as we've verified in the previous iteration it would fit.
			currentOmissionIndicator := omissionSep + fmt.Sprintf(opts.omissionIndicatorFmt, len(elems)-i)
			joined.WriteString(currentOmissionIndicator)
			break
		}

		// It's verified that either the current element + the potential next omission indicator fits,
		// or it's the last element and it fits.
		joined.WriteString(sep + elem)
	}

	return strings.TrimSpace(joined.String())
}

func AggregateStatusConditions(conditions []metav1.Condition, condition metav1.Condition) (metav1.Condition, error) {
	var defaultVal bool
	switch condition.Status {
	case metav1.ConditionTrue:
		defaultVal = true

	case metav1.ConditionFalse:
		defaultVal = false

	default:
		return metav1.Condition{}, fmt.Errorf("unsupported default value %q", condition.Status)
	}

	var trueConditions, falseConditions, unknownConditions []metav1.Condition
	for _, c := range conditions {
		switch c.Status {
		case metav1.ConditionUnknown:
			unknownConditions = append(unknownConditions, c)

		case metav1.ConditionTrue:
			trueConditions = append(trueConditions, c)

		case metav1.ConditionFalse:
			falseConditions = append(falseConditions, c)

		default:
			return metav1.Condition{}, fmt.Errorf("unknown condition status %q", c.Status)
		}
	}

	if defaultVal == true && len(falseConditions) > 0 {
		reason, message := aggregateStatusConditionInfo(falseConditions)
		return metav1.Condition{
			Type:               condition.Type,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: condition.ObservedGeneration,
		}, nil
	}

	if defaultVal == false && len(trueConditions) > 0 {
		reason, message := aggregateStatusConditionInfo(trueConditions)
		return metav1.Condition{
			Type:               condition.Type,
			Status:             metav1.ConditionTrue,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: condition.ObservedGeneration,
		}, nil
	}

	if len(unknownConditions) > 0 {
		reason, message := aggregateStatusConditionInfo(unknownConditions)
		return metav1.Condition{
			Type:               condition.Type,
			Status:             metav1.ConditionUnknown,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: condition.ObservedGeneration,
		}, nil
	}

	return condition, nil
}

func SetStatusConditionFromError(conditions *[]metav1.Condition, err error, conditionType string, observedGeneration int64) {
	if err != nil {
		apimeta.SetStatusCondition(conditions, metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.ErrorReason,
			Message:            outilerrors.NewMultilineAggregate([]error{err}).Error(),
			ObservedGeneration: observedGeneration,
		})
	} else {
		apimeta.SetStatusCondition(conditions, metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: observedGeneration,
		})
	}
}

func AddGenericProgressingStatusCondition(conditions *[]metav1.Condition, conditionType string, obj runtime.Object, verb string, observedGeneration int64) {
	*conditions = append(*conditions, metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             internalapi.ProgressingReason,
		Message:            fmt.Sprintf("Progressing: Running %q on %q", verb, resource.GetObjectGVKOrUnknown(obj)),
		ObservedGeneration: observedGeneration,
	})
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

func IsNodeConfigPod(pod *corev1.Pod) bool {
	_, ok := pod.Labels[naming.NodeConfigNameLabel]
	return ok
}
