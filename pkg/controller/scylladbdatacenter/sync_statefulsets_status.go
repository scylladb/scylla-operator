package scylladbdatacenter

import (
	"fmt"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (sdcc *Controller) setStatefulSetsAvailableStatusCondition(
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
) {
	desiredMembers := int32(0)
	updatedMembers := int32(0)
	readyMembers := int32(0)
	var racksInDifferentVersion []string
	for _, rack := range sdc.Spec.Racks {
		rackCount, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			klog.ErrorS(err, "can't get rack node count", "ScyllaDBDatacenter", naming.ObjRef(sdc), "Rack", rack.Name)
			continue
		}
		desiredMembers += *rackCount

		rackStatus, _, found := oslices.Find(status.Racks, func(status scyllav1alpha1.RackStatus) bool {
			return status.Name == rack.Name
		})
		if !found {
			klog.Errorf("Can't find status for rack %q", rack.Name)
			continue
		}

		expectedVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
		if err != nil {
			klog.ErrorS(err, "can't get version from image", "Image", sdc.Spec.ScyllaDB.Image)
			continue
		}

		if rackStatus.CurrentVersion != expectedVersion {
			racksInDifferentVersion = append(racksInDifferentVersion, rack.Name)
		}

		if rackStatus.Stale == nil || (*rackStatus.Stale) {
			continue
		}

		if rackStatus.ReadyNodes != nil {
			readyMembers += *rackStatus.ReadyNodes
		}

		if rackStatus.UpdatedNodes != nil {
			updatedMembers += *rackStatus.UpdatedNodes
		}
	}

	switch {
	case len(racksInDifferentVersion) > 0:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "RacksNotAtDesiredVersion",
			Message:            fmt.Sprintf("Racks %q are not in the desired version", strings.Join(racksInDifferentVersion, ", ")),
			ObservedGeneration: sdc.Generation,
		})

	case updatedMembers != desiredMembers:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "MembersNotUpdated",
			Message:            fmt.Sprintf("Only %d out of %d member(s) have been updated", updatedMembers, desiredMembers),
			ObservedGeneration: sdc.Generation,
		})

	case readyMembers != desiredMembers:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "MembersNotReady",
			Message:            fmt.Sprintf("Only %d out of %d member(s) are ready", readyMembers, desiredMembers),
			ObservedGeneration: sdc.Generation,
		})

	default:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sdc.Generation,
		})
	}
}
