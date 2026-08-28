package scylladbdatacenter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_setStatefulSetsAvailableStatusCondition(t *testing.T) {
	t.Parallel()

	const image = "docker.io/scylladb/scylla:6.2.0"

	newSDC := func(nodes int32, racks ...string) *scyllav1alpha1.ScyllaDBDatacenter {
		sdc := newScyllaDBDatacenter()
		sdc.Generation = 7
		sdc.Spec.ScyllaDB.Image = image
		sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{Nodes: new(nodes)}
		for _, rack := range racks {
			sdc.Spec.Racks = append(sdc.Spec.Racks, scyllav1alpha1.RackSpec{Name: rack})
		}
		return sdc
	}

	newRackStatus := func(name, version string, ready, updated int32, stale bool) scyllav1alpha1.RackStatus {
		return scyllav1alpha1.RackStatus{
			Name:           name,
			CurrentVersion: version,
			ReadyNodes:     new(ready),
			UpdatedNodes:   new(updated),
			Stale:          new(stale),
		}
	}

	tt := []struct {
		name              string
		sdc               *scyllav1alpha1.ScyllaDBDatacenter
		racks             []scyllav1alpha1.RackStatus
		expectedCondition metav1.Condition
	}{
		{
			name: "available when all racks are at the desired version, updated and ready",
			sdc:  newSDC(2, "a", "b"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 2, false),
				newRackStatus("b", "6.2.0", 2, 2, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionTrue,
				Reason:             internalapi.AsExpectedReason,
				Message:            "",
				ObservedGeneration: 7,
			},
		},
		{
			name: "not available when a rack is at a different version",
			sdc:  newSDC(2, "a", "b"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 2, false),
				newRackStatus("b", "6.1.0", 2, 2, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "RacksNotAtDesiredVersion",
				Message:            `Racks "b" are not in the desired version`,
				ObservedGeneration: 7,
			},
		},
		{
			name: "not available when members are not updated",
			sdc:  newSDC(2, "a"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 1, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotUpdated",
				Message:            "Only 1 out of 2 member(s) have been updated",
				ObservedGeneration: 7,
			},
		},
		{
			name: "not available when members are not ready",
			sdc:  newSDC(2, "a"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 1, 2, false),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotReady",
				Message:            "Only 1 out of 2 member(s) are ready",
				ObservedGeneration: 7,
			},
		},
		{
			name: "stale rack statuses don't count towards updated members",
			sdc:  newSDC(2, "a"),
			racks: []scyllav1alpha1.RackStatus{
				newRackStatus("a", "6.2.0", 2, 2, true),
			},
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotUpdated",
				Message:            "Only 0 out of 2 member(s) have been updated",
				ObservedGeneration: 7,
			},
		},
		{
			name:  "a rack without a status is skipped",
			sdc:   newSDC(2, "a"),
			racks: nil,
			expectedCondition: metav1.Condition{
				Type:               statefulSetControllerAvailableCondition,
				Status:             metav1.ConditionFalse,
				Reason:             "MembersNotUpdated",
				Message:            "Only 0 out of 2 member(s) have been updated",
				ObservedGeneration: 7,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			status := &scyllav1alpha1.ScyllaDBDatacenterStatus{Racks: tc.racks}
			sdcc.setStatefulSetsAvailableStatusCondition(tc.sdc, status)

			if len(status.Conditions) != 1 {
				t.Fatalf("expected exactly one condition, got %v", status.Conditions)
			}
			got := status.Conditions[0]
			got.LastTransitionTime = metav1.Time{}
			if diff := cmp.Diff(tc.expectedCondition, got); diff != "" {
				t.Errorf("condition differs (-want +got):\n%s", diff)
			}
		})
	}
}
