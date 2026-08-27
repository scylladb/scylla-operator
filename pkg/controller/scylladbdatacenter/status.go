package scylladbdatacenter

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

func (sdcc *Controller) updateStatus(ctx context.Context, currentSC *scyllav1alpha1.ScyllaDBDatacenter, status *scyllav1alpha1.ScyllaDBDatacenterStatus) error {
	if apiequality.Semantic.DeepEqual(&currentSC.Status, status) {
		return nil
	}

	sdc := currentSC.DeepCopy()
	sdc.Status = *status

	// Make sure that any "live" updates to the status are always manifested in the aggregated fields.
	updateAggregatedStatusFields(&sdc.Status)

	klog.V(2).InfoS("Updating status", "ScyllaDBDatacenter", klog.KObj(sdc))

	_, err := sdcc.scyllaClient.ScyllaDBDatacenters(sdc.Namespace).UpdateStatus(ctx, sdc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaDBDatacenter", klog.KObj(sdc))

	return nil
}

// getScyllaVersion returns the Scylla version from the ordinal-0 pod owned by sts.
func getScyllaVersion(podLister corev1listers.PodLister, sts *appsv1.StatefulSet) (string, error) {
	firstMemberName := fmt.Sprintf("%s-0", sts.Name)
	firstMember, err := podLister.Pods(sts.Namespace).Get(firstMemberName)
	if err != nil {
		return "", fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sts.Namespace, firstMemberName), err)
	}

	controllerRef := metav1.GetControllerOfNoCopy(firstMember)
	if controllerRef == nil || controllerRef.UID != sts.UID {
		return "", fmt.Errorf("internal error: pod %q is not controlled by us", naming.ManualRef(sts.Namespace, firstMemberName))
	}

	version, err := naming.ScyllaVersion(firstMember.Spec.Containers)
	if err != nil {
		return "", fmt.Errorf("can't get scylla version for pod %q: %w", naming.ManualRef(sts.Namespace, firstMemberName), err)
	}

	return version, nil
}

// calculateRackStatus builds rack status from its StatefulSet and member Services.
// If sts is nil, it returns a stale zero status so the rack still appears in status.
// Empty racks report the target image version; non-empty racks report the version from ordinal-0.
// The decommissioning nodes are derived from the decommissioned labels of the rack's member Services in services.
func calculateRackStatus(podLister corev1listers.PodLister, sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName string, sts *appsv1.StatefulSet, services map[string]*corev1.Service) *scyllav1alpha1.RackStatus {
	status := &scyllav1alpha1.RackStatus{
		Name:                 rackName,
		Nodes:                new(int32(0)),
		CurrentNodes:         new(int32(0)),
		UpdatedNodes:         new(int32(0)),
		ReadyNodes:           new(int32(0)),
		AvailableNodes:       new(int32(0)),
		DecommissioningNodes: calculateDecommissioningNodes(rackName, services),
		Stale:                new(true),
	}

	if sts == nil {
		return status
	}

	status.Nodes = new(*sts.Spec.Replicas)
	status.ReadyNodes = new(sts.Status.ReadyReplicas)
	status.AvailableNodes = new(sts.Status.AvailableReplicas)
	status.UpdatedNodes = new(sts.Status.UpdatedReplicas)
	status.CurrentNodes = new(sts.Status.CurrentReplicas)
	status.Stale = new(sts.Status.ObservedGeneration < sts.Generation)

	scyllaDBImageVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
	if err != nil {
		klog.ErrorS(err, "can't get version of image", "Image", sdc.Spec.ScyllaDB.Image)
	}

	status.UpdatedVersion = scyllaDBImageVersion

	// Update Rack Version
	if status.Nodes != nil && *status.Nodes == 0 {
		status.CurrentVersion = scyllaDBImageVersion
	} else {
		version, err := getScyllaVersion(podLister, sts)
		if err != nil {
			klog.ErrorS(err, "can't get scylla version")
		} else {
			status.CurrentVersion = version
		}
	}

	return status
}

// calculateDecommissioningNodes returns the nodes of the rack whose member Service carries the decommissioned label,
// with either value: a node is leaving from the moment its decommission is requested until its Service is pruned.
// The labels are the ground truth, so the list is derived from them on every sync.
// Entries are sorted by name so that the list is stable across syncs. It is nil when no node is leaving.
func calculateDecommissioningNodes(rackName string, services map[string]*corev1.Service) []scyllav1alpha1.DecommissioningNodeStatus {
	var nodes []scyllav1alpha1.DecommissioningNodeStatus
	for _, svc := range services {
		if svc.Labels[naming.RackNameLabel] != rackName {
			continue
		}

		if _, ok := svc.Labels[naming.DecommissionedLabel]; !ok {
			continue
		}

		nodes = append(nodes, scyllav1alpha1.DecommissioningNodeStatus{
			Name: svc.Name,
		})
	}

	slices.SortFunc(nodes, func(a, b scyllav1alpha1.DecommissioningNodeStatus) int {
		return strings.Compare(a.Name, b.Name)
	})

	return nodes
}

// getRackStatus returns the status of the named rack from racks, or nil if there is none.
func getRackStatus(racks []scyllav1alpha1.RackStatus, rackName string) *scyllav1alpha1.RackStatus {
	rackStatus, _, ok := oslices.Find(racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
		return rackStatus.Name == rackName
	})
	if !ok {
		return nil
	}

	return &rackStatus
}

// getServiceOrdinal returns the ordinal of the member Service from its name.
func getServiceOrdinal(name string) (int32, error) {
	ordinalStrings := serviceOrdinalRegex.FindStringSubmatch(name)
	if len(ordinalStrings) != 2 {
		return 0, fmt.Errorf("can't parse ordinal from service name %q", name)
	}

	ordinal, err := strconv.ParseInt(ordinalStrings[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("can't parse ordinal from service name %q: %w", name, err)
	}

	return int32(ordinal), nil
}

// getEffectiveRackNodeCount returns the node count the rack reconciles to.
// While the rack has decommissioning nodes listed in status, the count excludes them: the listed nodes are leaving
// and the StatefulSet can only remove its highest ordinals, so the count is the lowest listed ordinal, capped at the
// current StatefulSet replicas. Any change to the spec node count waits until the list is empty.
// Otherwise it is the spec node count.
func getEffectiveRackNodeCount(sdc *scyllav1alpha1.ScyllaDBDatacenter, status *scyllav1alpha1.ScyllaDBDatacenterStatus, statefulSets map[string]*appsv1.StatefulSet, rack scyllav1alpha1.RackSpec) (*int32, error) {
	specNodeCount, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
	if err != nil {
		return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
	}

	rackStatus := getRackStatus(status.Racks, rack.Name)
	if rackStatus == nil || len(rackStatus.DecommissioningNodes) == 0 {
		return specNodeCount, nil
	}

	var nodeCount *int32
	for _, node := range rackStatus.DecommissioningNodes {
		ordinal, err := getServiceOrdinal(node.Name)
		if err != nil {
			return nil, fmt.Errorf("can't get ordinal of decommissioning node %q of rack %q of ScyllaDBDatacenter %q: %w", node.Name, rack.Name, naming.ObjRef(sdc), err)
		}

		if nodeCount == nil || ordinal < *nodeCount {
			nodeCount = new(ordinal)
		}
	}

	sts, ok := statefulSets[naming.StatefulSetNameForRack(rack, sdc)]
	if ok && sts.Spec.Replicas != nil && *sts.Spec.Replicas < *nodeCount {
		nodeCount = new(*sts.Spec.Replicas)
	}

	return nodeCount, nil
}

func updateAggregatedStatusFields(status *scyllav1alpha1.ScyllaDBDatacenterStatus) {
	status.Nodes = new(int32(0))
	status.ReadyNodes = new(int32(0))
	status.AvailableNodes = new(int32(0))

	for rackName := range status.Racks {
		rackStatus := status.Racks[rackName]

		*status.Nodes += *rackStatus.Nodes
		*status.ReadyNodes += *rackStatus.ReadyNodes
		*status.AvailableNodes += *rackStatus.AvailableNodes
	}
}

// calculateStatus calculates the ScyllaCluster status.
// This function should always succeed. Do not return an error.
// If a particular object can be missing, it should be reflected in the value itself, like "Unknown" or "".
func (sdcc *Controller) calculateStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter, statefulSetMap map[string]*appsv1.StatefulSet, serviceMap map[string]*corev1.Service) *scyllav1alpha1.ScyllaDBDatacenterStatus {
	status := sdc.Status.DeepCopy()
	status.ObservedGeneration = new(sdc.Generation)

	// Clear the previous rack status.
	status.Racks = []scyllav1alpha1.RackStatus{}

	// Calculate the status for racks.
	for _, rack := range sdc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sdc)
		status.Racks = append(status.Racks, *calculateRackStatus(sdcc.podLister, sdc, rack.Name, statefulSetMap[stsName], serviceMap))
	}

	updateAggregatedStatusFields(status)

	return status
}
