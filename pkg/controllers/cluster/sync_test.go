package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/actions"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNextAction(t *testing.T) {

	const (
		operatorImage = "scylladb/scylla-operator:latest"
	)

	members := int32(3)
	cluster := unit.NewSingleRackCluster(members)
	rack, err := resource.StatefulSetForRack(cluster.Spec.Datacenter.Racks[0], cluster, operatorImage)
	if err != nil {
		t.Fatal(err)
	}

	clusterNewRackCreate := cluster.DeepCopy()
	clusterNewRackCreate.Spec.Datacenter.Racks = append(
		clusterNewRackCreate.Spec.Datacenter.Racks,
		scyllav1.RackSpec{
			Name:    "new-rack",
			Members: 2,
		},
	)

	clusterExistingRackScaleUp := cluster.DeepCopy()
	clusterExistingRackScaleUp.Status.Racks["test-rack"] = scyllav1.RackStatus{
		Members:      members - 1,
		ReadyMembers: members - 1,
	}

	clusterBeginRackScaleDown := cluster.DeepCopy()
	clusterBeginRackScaleDown.Status.Racks["test-rack"] = scyllav1.RackStatus{
		Members:      members + 1,
		ReadyMembers: members + 1,
	}

	clusterResumeRackScaleDown := cluster.DeepCopy()
	testRackStatus := clusterResumeRackScaleDown.Status.Racks["test-rack"]
	scyllav1.SetRackCondition(&testRackStatus, scyllav1.RackConditionTypeMemberLeaving)
	clusterResumeRackScaleDown.Status.Racks["test-rack"] = testRackStatus

	clusterBeginVersionUpgrade := cluster.DeepCopy()
	version := semver.MustParse(clusterBeginRackScaleDown.Spec.Version)
	version.Patch++
	clusterBeginVersionUpgrade.Spec.Version = version.String()

	clusterResumeVersionUpgrade := cluster.DeepCopy()
	testRackStatus = clusterResumeVersionUpgrade.Status.Racks["test-rack"]
	scyllav1.SetRackCondition(&testRackStatus, scyllav1.RackConditionTypeUpgrading)
	clusterResumeVersionUpgrade.Status.Racks["test-rack"] = testRackStatus

	clusterSidecarUpgrade := cluster.DeepCopy()
	rackWithOldSidecarImage := rack.DeepCopy()
	idx, err := naming.FindSidecarInjectorContainer(rackWithOldSidecarImage.Spec.Template.Spec.InitContainers)
	if err != nil {
		t.Fatal(err)
	}
	rackWithOldSidecarImage.Spec.Template.Spec.InitContainers[idx].Image = "scylladb/scylla-operator:old"

	tests := []struct {
		name           string
		cluster        *scyllav1.ScyllaCluster
		objects        []runtime.Object
		expectedAction string
		expectNoAction bool
	}{
		{
			name:           "create rack",
			cluster:        clusterNewRackCreate,
			objects:        []runtime.Object{rack},
			expectedAction: actions.RackCreateAction,
		},
		{
			name:           "scale up existing rack",
			cluster:        clusterExistingRackScaleUp,
			objects:        []runtime.Object{rack},
			expectedAction: actions.RackScaleUpAction,
		},
		{
			name:           "scale down begin",
			cluster:        clusterBeginRackScaleDown,
			objects:        []runtime.Object{rack},
			expectedAction: actions.RackScaleDownAction,
		},
		{
			name:           "scale down resume",
			cluster:        clusterResumeRackScaleDown,
			objects:        []runtime.Object{rack},
			expectedAction: actions.RackScaleDownAction,
		},
		{
			name:           "patch upgrade begin",
			cluster:        clusterBeginVersionUpgrade,
			objects:        []runtime.Object{rack},
			expectedAction: actions.ClusterVersionUpgradeAction,
		},
		{
			name:           "patch upgrade in-progress",
			cluster:        clusterResumeVersionUpgrade,
			objects:        []runtime.Object{rack},
			expectedAction: actions.ClusterVersionUpgradeAction,
		},
		{
			name:           "sidecar upgrade",
			cluster:        clusterSidecarUpgrade,
			objects:        []runtime.Object{rackWithOldSidecarImage},
			expectedAction: fmt.Sprintf("%s%s", actions.RackSynchronizedActionPrefix, actions.SidecarVersionUpgradeAction),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			cc := &ClusterReconciler{
				KubeClient:    fake.NewSimpleClientset(test.objects...),
				OperatorImage: operatorImage,
			}

			// Calculate next action
			a, err := cc.nextAction(context.Background(), test.cluster)
			if err != nil {
				t.Errorf("expected non nil err, got %s", err)
			}
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
