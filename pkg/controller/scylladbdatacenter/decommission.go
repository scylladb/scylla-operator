// Copyright (c) 2026 ScyllaDB.

package scylladbdatacenter

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func getRackDecommissioningNodes(status *scyllav1alpha1.ScyllaDBDatacenterStatus, rackName string) []string {
	rackStatus, _, ok := oslices.Find(status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
		return rackStatus.Name == rackName
	})
	if !ok {
		return nil
	}

	return rackStatus.DecommissioningNodes
}

func setRackDecommissioningNodes(status *scyllav1alpha1.ScyllaDBDatacenterStatus, rackName string, decommissioningNodes []string) {
	_, i, ok := oslices.Find(status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
		return rackStatus.Name == rackName
	})
	if !ok {
		return
	}

	status.Racks[i].DecommissioningNodes = decommissioningNodes
}

// getEffectiveRackNodeCount returns the node count the rack reconciles towards.
// While any of the rack's nodes are recorded as decommissioning, the rack reconciles as if its node count excluded
// them. Because members are identified by their StatefulSet ordinal, and a StatefulSet can only remove its highest
// ordinal, the leaving members have to hold the top of the ordinal range until they are removed. The node count is
// therefore pinned to the lowest recorded ordinal, and the node count from the spec only takes effect once the record
// is empty.
func getEffectiveRackNodeCount(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName string, decommissioningNodes []string) (*int32, error) {
	if len(decommissioningNodes) > 0 {
		lowestOrdinal, err := getMemberServiceOrdinal(decommissioningNodes[0])
		if err != nil {
			return nil, fmt.Errorf("can't get ordinal of decommissioning node %q: %w", decommissioningNodes[0], err)
		}

		for _, name := range decommissioningNodes[1:] {
			ordinal, err := getMemberServiceOrdinal(name)
			if err != nil {
				return nil, fmt.Errorf("can't get ordinal of decommissioning node %q: %w", name, err)
			}

			lowestOrdinal = min(lowestOrdinal, ordinal)
		}

		return new(lowestOrdinal), nil
	}

	nodeCount, err := controllerhelpers.GetRackNodeCount(sdc, rackName)
	if err != nil {
		return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rackName, naming.ObjRef(sdc), err)
	}

	return nodeCount, nil
}

// reconcileRackDecommissioningNodes maintains the record of the nodes that are leaving their racks.
//
// A node's decommission can't be revoked, so before any decommission is initiated the operator records which nodes are
// going to leave. The record is what holds the barrier: while it's non-empty, the rack reconciles towards the node count
// that excludes the recorded nodes, so a node count change made in the meantime only takes effect once they are fully
// removed, and the capacity they give back bootstraps as new, empty nodes.
//
// The decommission labels on the member services stay the ground truth underneath the record: every labeled member is
// taken into the record, so a record lost to a restored backup or a manual edit is rebuilt from the labels.
//
// It reports whether the record has changed, in which case the caller must not act on it before it's persisted.
func (sdcc *Controller) reconcileRackDecommissioningNodes(
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	statefulSets map[string]*appsv1.StatefulSet,
	services map[string]*corev1.Service,
) ([]metav1.Condition, bool, error) {
	var progressingConditions []metav1.Condition
	changed := false

	for _, rack := range sdc.Spec.Racks {
		sts, ok := statefulSets[naming.StatefulSetNameForRack(rack, sdc)]
		if !ok || sts.Spec.Replicas == nil {
			// Nothing can be leaving a rack that hasn't been created yet.
			continue
		}

		rackServices := map[string]*corev1.Service{}
		for _, svc := range services {
			if svc.Labels[naming.RackNameLabel] == rack.Name {
				rackServices[svc.Name] = svc
			}
		}

		existing := getRackDecommissioningNodes(status, rack.Name)
		required, err := makeRackDecommissioningNodes(sdc, rack, existing, sts, rackServices)
		if err != nil {
			return progressingConditions, changed, fmt.Errorf("can't make decommissioning nodes of rack %q: %w", rack.Name, err)
		}

		if slices.Equal(existing, required) {
			continue
		}

		klog.V(2).InfoS("Updating the record of decommissioning nodes", "ScyllaDBDatacenter", klog.KObj(sdc), "Rack", rack.Name, "Existing", existing, "Required", required)
		setRackDecommissioningNodes(status, rack.Name, required)
		changed = true

		switch {
		case len(existing) == 0:
			sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeNormal, "DecommissioningRackNodes", "Node(s) %s of rack %q are being decommissioned. Node count changes of the rack will only take effect once they are removed.", strings.Join(required, ", "), rack.Name)
		case len(required) == 0:
			sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeNormal, "DecommissionedRackNodes", "Node(s) %s of rack %q have been decommissioned and removed. Reconciliation of the rack node count is resuming.", strings.Join(existing, ", "), rack.Name)
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               statefulSetControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "RecordingRackDecommissioningNodes",
			Message:            fmt.Sprintf("Recording the decommissioning nodes of rack %q in the status.", rack.Name),
			ObservedGeneration: sdc.Generation,
		})
	}

	return progressingConditions, changed, nil
}

// makeRackDecommissioningNodes returns the names of the rack's nodes that are leaving, ordered by their ordinal.
// It keeps the recorded nodes that haven't been fully removed yet, adds any member already carrying the decommission
// label, and, if nothing is leaving, commits the nodes a scale down of the rack is about to remove.
func makeRackDecommissioningNodes(
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	rack scyllav1alpha1.RackSpec,
	decommissioningNodes []string,
	sts *appsv1.StatefulSet,
	rackServices map[string]*corev1.Service,
) ([]string, error) {
	var required []string

	for _, name := range decommissioningNodes {
		removed, err := isMemberRemoved(name, sts, rackServices)
		if err != nil {
			return nil, err
		}

		if !removed {
			required = append(required, name)
		}
	}

	// A labeled member is irrevocably leaving, no matter what the record says.
	for _, svc := range rackServices {
		if _, ok := svc.Labels[naming.DecommissionedLabel]; !ok {
			continue
		}

		_, err := getMemberServiceOrdinal(svc.Name)
		if err != nil {
			return nil, fmt.Errorf("can't get ordinal of decommissioned member service %q: %w", naming.ObjRef(svc), err)
		}

		if !slices.Contains(required, svc.Name) {
			required = append(required, svc.Name)
		}
	}

	if len(required) == 0 {
		// Commit the nodes of a scale down in a single write, before any of them is asked to decommission, so that the
		// operation is fully defined no matter what happens to the spec later. A scale down requested while another one
		// is still in progress is only committed once that one concludes.
		nodeCount, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
		}

		for ordinal := *nodeCount; ordinal < *sts.Spec.Replicas; ordinal++ {
			required = append(required, naming.MemberServiceName(rack, sdc, int(ordinal)))
		}
	}

	slices.SortFunc(required, func(lhs, rhs string) int {
		// Every name has already been parsed above, so an error here is impossible.
		lhsOrdinal, _ := getMemberServiceOrdinal(lhs)
		rhsOrdinal, _ := getMemberServiceOrdinal(rhs)
		return cmp.Compare(lhsOrdinal, rhsOrdinal)
	})

	return required, nil
}

// isMemberRemoved reports whether a member that was leaving is gone: the StatefulSet no longer accounts for its
// ordinal, and its service either has been pruned or no longer carries the decommission label, meaning it has been
// recreated for a fresh bootstrap.
func isMemberRemoved(name string, sts *appsv1.StatefulSet, rackServices map[string]*corev1.Service) (bool, error) {
	ordinal, err := getMemberServiceOrdinal(name)
	if err != nil {
		return false, fmt.Errorf("can't get ordinal of decommissioning node %q: %w", name, err)
	}

	if ordinal < *sts.Spec.Replicas {
		return false, nil
	}

	svc, ok := rackServices[name]
	if !ok {
		return true, nil
	}

	_, ok = svc.Labels[naming.DecommissionedLabel]

	return !ok, nil
}
