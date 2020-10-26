package cluster

import (
	"context"
	"testing"

	"github.com/blang/semver"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/actions"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
)

func TestNextAction(t *testing.T) {

	members := int32(3)
	cluster := unit.NewSingleRackCluster(members)

	clusterNewRackCreate := cluster.DeepCopy()
	clusterNewRackCreate.Spec.Datacenter.Racks = append(
		clusterNewRackCreate.Spec.Datacenter.Racks,
		scyllav1alpha1.RackSpec{
			Name:    "new-rack",
			Members: 2,
		},
	)

	clusterExistingRackScaleUp := cluster.DeepCopy()
	clusterExistingRackScaleUp.Status.Racks["test-rack"] = scyllav1alpha1.RackStatus{
		Members:      members - 1,
		ReadyMembers: members - 1,
	}

	clusterBeginRackScaleDown := cluster.DeepCopy()
	clusterBeginRackScaleDown.Status.Racks["test-rack"] = scyllav1alpha1.RackStatus{
		Members:      members + 1,
		ReadyMembers: members + 1,
	}

	clusterResumeRackScaleDown := cluster.DeepCopy()
	testRackStatus := clusterResumeRackScaleDown.Status.Racks["test-rack"]
	scyllav1alpha1.SetRackCondition(&testRackStatus, scyllav1alpha1.RackConditionTypeMemberLeaving)
	clusterResumeRackScaleDown.Status.Racks["test-rack"] = testRackStatus

	clusterBeginVersionUpgrade := cluster.DeepCopy()
	version := semver.MustParse(clusterBeginRackScaleDown.Spec.Version)
	version.Patch++
	clusterBeginVersionUpgrade.Spec.Version = version.String()

	clusterResumeVersionUpgrade := cluster.DeepCopy()
	testRackStatus = clusterResumeVersionUpgrade.Status.Racks["test-rack"]
	scyllav1alpha1.SetRackCondition(&testRackStatus, scyllav1alpha1.RackConditionTypeUpgrading)
	clusterResumeVersionUpgrade.Status.Racks["test-rack"] = testRackStatus

	tests := []struct {
		name           string
		cluster        *scyllav1alpha1.ScyllaCluster
		expectedAction string
		expectNoAction bool
	}{
		{
			name:           "create rack",
			cluster:        clusterNewRackCreate,
			expectedAction: actions.RackCreateAction,
		},
		{
			name:           "scale up existing rack",
			cluster:        clusterExistingRackScaleUp,
			expectedAction: actions.RackScaleUpAction,
		},
		{
			name:           "scale down begin",
			cluster:        clusterBeginRackScaleDown,
			expectedAction: actions.RackScaleDownAction,
		},
		{
			name:           "scale down resume",
			cluster:        clusterResumeRackScaleDown,
			expectedAction: actions.RackScaleDownAction,
		},
		{
			name:           "patch upgrade begin",
			cluster:        clusterBeginVersionUpgrade,
			expectedAction: actions.ClusterVersionUpgradeAction,
		},
		{
			name:           "patch upgrade in-progress",
			cluster:        clusterResumeVersionUpgrade,
			expectNoAction: true,
		},
	}

	cc := &ClusterReconciler{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Calculate next action
			a := cc.nextAction(context.Background(), test.cluster)
			if a == nil {
				if test.expectNoAction {
					return
				}
				t.Fatalf("No action taken, expected action %s", test.expectedAction)
			}
			if test.expectedAction != a.Name() {
				t.Fatalf("Action taken does not match expected action. Expected: %s, Got: %s", test.expectedAction, a.Name())
			}

		})
	}
}
