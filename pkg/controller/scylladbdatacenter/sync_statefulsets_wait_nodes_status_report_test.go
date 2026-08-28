package scylladbdatacenter

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_waitForNodesStatusReportController(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		conditions      []metav1.Condition
		expectedReasons []string
	}{
		{
			name:            "proceeds when the controller is settled",
			conditions:      []metav1.Condition{{Type: scyllaDBDatacenterNodesStatusReportControllerProgressingCondition, Status: metav1.ConditionFalse}},
			expectedReasons: nil,
		},
		{
			name:            "waits when the controller is progressing",
			conditions:      []metav1.Condition{{Type: scyllaDBDatacenterNodesStatusReportControllerProgressingCondition, Status: metav1.ConditionTrue}},
			expectedReasons: []string{reasonWaitingForScyllaDBDatacenterNodesStatusReportController},
		},
		{
			name:            "waits when the controller is degraded",
			conditions:      []metav1.Condition{{Type: scyllaDBDatacenterNodesStatusReportControllerDegradedCondition, Status: metav1.ConditionTrue}},
			expectedReasons: []string{reasonWaitingForScyllaDBDatacenterNodesStatusReportController},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sdcc := &Controller{}
			res, err := sdcc.waitForNodesStatusReportController(context.Background(), &statefulSetSyncContext{
				sdc:    newScyllaDBDatacenter(),
				status: &scyllav1alpha1.ScyllaDBDatacenterStatus{Conditions: tc.conditions},
			})
			conditions := res.progressingConditions
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var reasons []string
			for _, cond := range conditions {
				reasons = append(reasons, cond.Reason)
			}
			if diff := cmp.Diff(tc.expectedReasons, reasons); diff != "" {
				t.Errorf("condition reasons differ (-want +got):\n%s", diff)
			}
		})
	}
}
