package scylladbcluster_test

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/controller/scylladbcluster"
)

func TestMakeRemoteKindControllerDatacenterConditionFunc(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		kind              string
		conditionType     string
		dcName            string
		expectedCondition string
	}{
		{
			name:              "condition having provided kind and condition type and dc name",
			kind:              "ConfigMap",
			conditionType:     "Progressing",
			dcName:            "dc1",
			expectedCondition: "RemoteConfigMapControllerDatacenterdc1Progressing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCondition := scylladbcluster.MakeRemoteKindControllerDatacenterConditionFunc(tc.kind, tc.conditionType)(tc.dcName)
			if gotCondition != tc.expectedCondition {
				t.Errorf("expected condition %q, got %q", tc.expectedCondition, gotCondition)
			}
		})
	}
}
