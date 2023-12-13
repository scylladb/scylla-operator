package controllerhelpers

import (
	"fmt"
	"sort"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	corev1schedulinghelpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
)

func GetScyllaHost(statefulsetName string, ordinal int32, services map[string]*corev1.Service, pods map[string]*corev1.Pod) (string, error) {
	svcName := fmt.Sprintf("%s-%d", statefulsetName, ordinal)
	svc, found := services[svcName]
	if !found {
		return "", fmt.Errorf("missing service %q", svcName)
	}

	if svc.Spec.ClusterIP != corev1.ClusterIPNone {
		return svc.Spec.ClusterIP, nil
	}

	pod, found := pods[svcName]
	if !found {
		return "", fmt.Errorf("missing pod %q", svcName)
	}

	host := pod.Status.PodIP
	if len(host) < 1 {
		return "", fmt.Errorf("missing PodIP of pod %q", svcName)
	}

	return host, nil
}

func GetRequiredScyllaHosts(sc *scyllav1.ScyllaCluster, services map[string]*corev1.Service, pods map[string]*corev1.Pod) ([]string, error) {
	var hosts []string
	var errs []error
	for _, rack := range sc.Spec.Datacenter.Racks {
		for ord := int32(0); ord < rack.Members; ord++ {
			stsName := naming.StatefulSetNameForRack(rack, sc)
			host, err := GetScyllaHost(stsName, ord, services, pods)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			hosts = append(hosts, host)
		}
	}
	var err error = errors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func NewScyllaClient(cfg *scyllaclient.Config) (*scyllaclient.Client, error) {
	scyllaClient, err := scyllaclient.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return scyllaClient, nil
}

func NewScyllaClientFromToken(hosts []string, authToken string) (*scyllaclient.Client, error) {
	cfg := scyllaclient.DefaultConfig(authToken, hosts...)
	return NewScyllaClient(cfg)
}

func NewScyllaClientForLocalhost() (*scyllaclient.Client, error) {
	cfg := scyllaclient.DefaultConfig("", "localhost")
	cfg.Scheme = "http"
	cfg.Port = fmt.Sprintf("%d", naming.ScyllaAPIPort)
	t := scyllaclient.DefaultTransport()
	t.TLSClientConfig = nil
	cfg.Transport = t
	return NewScyllaClient(cfg)
}

func SetRackCondition(rackStatus *scyllav1.RackStatus, newCondition scyllav1.RackConditionType) {
	for i := range rackStatus.Conditions {
		if rackStatus.Conditions[i].Type == newCondition {
			rackStatus.Conditions[i].Status = corev1.ConditionTrue
			return
		}
	}
	rackStatus.Conditions = append(
		rackStatus.Conditions,
		scyllav1.RackCondition{Type: newCondition, Status: corev1.ConditionTrue},
	)
}

func FindNodeStatus(nodeStatuses []scyllav1alpha1.NodeConfigNodeStatus, nodeName string) *scyllav1alpha1.NodeConfigNodeStatus {
	for i := range nodeStatuses {
		ns := &nodeStatuses[i]
		if ns.Name == nodeName {
			return ns
		}
	}

	return nil
}

func SetNodeStatus(nodeStatuses []scyllav1alpha1.NodeConfigNodeStatus, status *scyllav1alpha1.NodeConfigNodeStatus) []scyllav1alpha1.NodeConfigNodeStatus {
	for i, ns := range nodeStatuses {
		if ns.Name == status.Name {
			nodeStatuses[i] = *status
			return nodeStatuses
		}
	}

	nodeStatuses = append(nodeStatuses, *status)

	sort.SliceStable(nodeStatuses, func(i, j int) bool {
		return nodeStatuses[i].Name < nodeStatuses[j].Name
	})

	return nodeStatuses
}

func SetAggregatedNodeConditions(nodeName string, conditions *[]metav1.Condition, generation int64) error {
	const (
		nodeAvailableConditionFormat   = "Node%sAvailable"
		nodeProgressingConditionFormat = "Node%sProgressing"
		nodeDegradedConditionFormat    = "Node%sDegraded"
	)

	nodeAvailableConditionType := fmt.Sprintf(nodeAvailableConditionFormat, nodeName)
	availableCondition, err := AggregateStatusConditions(
		FindStatusConditionsWithSuffix(*conditions, nodeAvailableConditionType),
		metav1.Condition{
			Type:               nodeAvailableConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(conditions, availableCondition)

	nodeProgressingConditionType := fmt.Sprintf(nodeProgressingConditionFormat, nodeName)
	progressingCondition, err := AggregateStatusConditions(
		FindStatusConditionsWithSuffix(*conditions, nodeProgressingConditionType),
		metav1.Condition{
			Type:               nodeProgressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(conditions, progressingCondition)

	nodeDegradedConditionType := fmt.Sprintf(nodeDegradedConditionFormat, nodeName)
	degradedCondition, err := AggregateStatusConditions(
		FindStatusConditionsWithSuffix(*conditions, nodeDegradedConditionType),
		metav1.Condition{
			Type:               nodeDegradedConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(conditions, degradedCondition)

	return nil
}

// SetNodeConfigStatusCondition sets the corresponding condition in conditions to newCondition.
// conditions must be non-nil.
// If the condition of the specified type already exists (all fields of the existing condition are updated to
// newCondition, LastTransitionTime is set to now if the new status differs from the old status)
// If a condition of the specified type does not exist (LastTransitionTime is set to now() if unset, and newCondition is appended)
func SetNodeConfigStatusCondition(conditions *[]scyllav1alpha1.NodeConfigCondition, newCondition scyllav1alpha1.NodeConfigCondition) {
	if conditions == nil {
		return
	}

	existingCondition := FindNodeConfigCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
	existingCondition.ObservedGeneration = newCondition.ObservedGeneration
}

// FindNodeConfigCondition finds the conditionType in conditions.
func FindNodeConfigCondition(conditions []scyllav1alpha1.NodeConfigCondition, conditionType scyllav1alpha1.NodeConfigConditionType) *scyllav1alpha1.NodeConfigCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsNodeConfigSelectingNode(nc *scyllav1alpha1.NodeConfig, node *corev1.Node) (bool, error) {
	// Check nodeSelector.

	if !labels.SelectorFromSet(nc.Spec.Placement.NodeSelector).Matches(labels.Set(node.Labels)) {
		return false, nil
	}

	// Check affinity.

	if nc.Spec.Placement.Affinity.NodeAffinity != nil &&
		nc.Spec.Placement.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		affinityNodeSelector, err := nodeaffinity.NewNodeSelector(
			nc.Spec.Placement.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
		if err != nil {
			return false, fmt.Errorf("can't construct node affinity node selector: %w", err)
		}

		if !affinityNodeSelector.Match(node) {
			return false, nil
		}
	}

	// Check taints and tolerations.

	_, isUntolerated := corev1schedulinghelpers.FindMatchingUntoleratedTaint(
		node.Spec.Taints,
		nc.Spec.Placement.Tolerations,
		func(t *corev1.Taint) bool {
			// We are only interested in NoSchedule and NoExecute taints.
			return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
		},
	)
	if isUntolerated {
		return false, nil
	}

	return true, nil
}

func IsNodeTunedForContainer(nc *scyllav1alpha1.NodeConfig, nodeName string, containerID string) bool {
	ns := FindNodeStatus(nc.Status.NodeStatuses, nodeName)
	if ns == nil {
		return false
	}

	if !ns.TunedNode {
		return false
	}

	return true
}

func IsNodeTuned(ncnss []scyllav1alpha1.NodeConfigNodeStatus, nodeName string) bool {
	ns := FindNodeStatus(ncnss, nodeName)
	return ns != nil && ns.TunedNode
}

func IsScyllaPod(pod *corev1.Pod) bool {
	// TODO: use a better label, verify the container
	if pod.Labels == nil {
		return false
	}

	if !labels.SelectorFromSet(naming.ScyllaLabels()).Matches(labels.Set(pod.Labels)) {
		return false
	}

	_, ok := pod.Labels[naming.ClusterNameLabel]
	if !ok {
		return false
	}

	return true
}
