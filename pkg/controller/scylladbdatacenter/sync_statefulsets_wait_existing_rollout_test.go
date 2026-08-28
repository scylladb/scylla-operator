package scylladbdatacenter

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_waitForExistingStatefulSetsRollout(t *testing.T) {
	t.Parallel()

	newRollingStatefulSet := func(name string, replicas int32, rolledOut bool) *appsv1.StatefulSet {
		sts := newStatefulSet(name)
		sts.Generation = 2
		sts.Spec.Replicas = new(replicas)
		sts.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
		sts.Status.ObservedGeneration = 1
		if rolledOut {
			sts.Status.ObservedGeneration = 2
			sts.Status.Replicas = replicas
			sts.Status.ReadyReplicas = replicas
			sts.Status.AvailableReplicas = replicas
			sts.Status.UpdatedReplicas = replicas
			sts.Status.CurrentRevision = "rev"
			sts.Status.UpdateRevision = "rev"
		}
		return sts
	}

	tt := []struct {
		name               string
		required           []*appsv1.StatefulSet
		existing           map[string]*appsv1.StatefulSet
		expectedConditions []metav1.Condition
	}{
		{
			name:               "ignores missing StatefulSets",
			required:           []*appsv1.StatefulSet{newRollingStatefulSet("foo", 1, false)},
			existing:           map[string]*appsv1.StatefulSet{},
			expectedConditions: nil,
		},
		{
			name:               "no condition for a rolled out StatefulSet",
			required:           []*appsv1.StatefulSet{newRollingStatefulSet("foo", 1, true)},
			existing:           map[string]*appsv1.StatefulSet{"foo": newRollingStatefulSet("foo", 1, true)},
			expectedConditions: nil,
		},
		{
			name:     "waits for a StatefulSet that is not rolled out",
			required: []*appsv1.StatefulSet{newRollingStatefulSet("foo", 1, true)},
			existing: map[string]*appsv1.StatefulSet{"foo": newRollingStatefulSet("foo", 1, false)},
			expectedConditions: []metav1.Condition{
				{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForStatefulSetRollout",
					Message:            `Waiting for StatefulSet "default/foo" to roll out.`,
					ObservedGeneration: 0,
				},
			},
		},
		{
			name:               "skips a StatefulSet that is about to be scaled",
			required:           []*appsv1.StatefulSet{newRollingStatefulSet("foo", 2, true)},
			existing:           map[string]*appsv1.StatefulSet{"foo": newRollingStatefulSet("foo", 1, false)},
			expectedConditions: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			conditions, err := sdcc.waitForExistingStatefulSetsRollout(context.Background(), &statefulSetSyncContext{
				sdc:                  newScyllaDBDatacenter(),
				requiredStatefulSets: tc.required,
				existingStatefulSets: tc.existing,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedConditions, conditions); diff != "" {
				t.Errorf("conditions differ (-want +got):\n%s", diff)
			}
		})
	}
}
