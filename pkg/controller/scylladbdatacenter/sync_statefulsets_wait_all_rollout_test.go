package scylladbdatacenter

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
)

func Test_waitForAllStatefulSetsRollout(t *testing.T) {
	t.Parallel()

	newRollingStatefulSet := func(name string, rolledOut bool) *appsv1.StatefulSet {
		sts := newStatefulSet(name)
		sts.Generation = 1
		sts.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
		if rolledOut {
			sts.Status.ObservedGeneration = 1
			sts.Status.Replicas = 1
			sts.Status.ReadyReplicas = 1
			sts.Status.AvailableReplicas = 1
			sts.Status.UpdatedReplicas = 1
			sts.Status.CurrentRevision = "rev"
			sts.Status.UpdateRevision = "rev"
		}
		return sts
	}

	t.Run("proceeds when all StatefulSets are rolled out", func(t *testing.T) {
		t.Parallel()

		sdcc := &Controller{}
		res, err := sdcc.waitForAllStatefulSetsRollout(context.Background(), &statefulSetSyncContext{
			sdc:                  newScyllaDBDatacenter(),
			requiredStatefulSets: []*appsv1.StatefulSet{newStatefulSet("a"), newStatefulSet("b")},
			existingStatefulSets: map[string]*appsv1.StatefulSet{"a": newRollingStatefulSet("a", true), "b": newRollingStatefulSet("b", true)},
		})
		conditions := res.progressingConditions
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(conditions) != 0 {
			t.Errorf("expected no conditions, got %v", conditions)
		}
	})

	t.Run("waits for the first StatefulSet that is not rolled out", func(t *testing.T) {
		t.Parallel()

		sdcc := &Controller{}
		res, err := sdcc.waitForAllStatefulSetsRollout(context.Background(), &statefulSetSyncContext{
			sdc:                  newScyllaDBDatacenter(),
			requiredStatefulSets: []*appsv1.StatefulSet{newStatefulSet("a"), newStatefulSet("b")},
			existingStatefulSets: map[string]*appsv1.StatefulSet{"a": newRollingStatefulSet("a", true), "b": newRollingStatefulSet("b", false)},
		})
		conditions := res.progressingConditions
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(conditions) != 1 || conditions[0].Reason != reasonWaitingForStatefulSetRollout || conditions[0].Message != `Waiting for StatefulSet "default/b" to roll out.` {
			t.Errorf("expected a single rollout condition for b, got %v", conditions)
		}
	})
}
