package scylladbcluster_test

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/controller/scylladbcluster"
)

func TestMakeRemoteKindControllerDatacenterConditionFormat(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		kind              string
		conditionType     string
		expectedCondition string
	}{
		{
			name:              "format string of remote kind controller datacenter condition having provided kind and condition type",
			kind:              "ConfigMap",
			conditionType:     "Progressing",
			expectedCondition: "RemoteConfigMapControllerDatacenter%sProgressing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCondition := scylladbcluster.MakeRemoteKindControllerDatacenterConditionFormat(tc.kind, tc.conditionType)
			if gotCondition != tc.expectedCondition {
				t.Errorf("expected condition %q, got %q", tc.expectedCondition, gotCondition)
			}
		})
	}
}
