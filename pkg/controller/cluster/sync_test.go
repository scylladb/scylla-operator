package cluster

import (
	"context"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/actions"
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
	clusterExistingRackScaleUp.Status.Racks["test-rack"] = &scyllav1alpha1.RackStatus{
		Members:      members - 1,
		ReadyMembers: members - 1,
	}

	clusterBeginRackScaleDown := cluster.DeepCopy()
	clusterBeginRackScaleDown.Status.Racks["test-rack"] = &scyllav1alpha1.RackStatus{
		Members:      members + 1,
		ReadyMembers: members + 1,
	}

	clusterResumeRackScaleDown := cluster.DeepCopy()
	scyllav1alpha1.SetRackCondition(clusterResumeRackScaleDown.Status.Racks["test-rack"], scyllav1alpha1.RackConditionTypeMemberLeaving)

	tests := []struct {
		name           string
		cluster        *scyllav1alpha1.Cluster
		expectedAction string
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
	}

	cc := &ClusterController{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Calculate next action
			a := cc.nextAction(context.Background(), test.cluster)
			if a == nil {
				t.Fatalf("No action taken, expected action %s", test.expectedAction)
			}
			if test.expectedAction != a.Name() {
				t.Fatalf("Action taken does not match expected action. Expected: %s, Got: %s", test.expectedAction, a.Name())
			}

		})
	}
}
