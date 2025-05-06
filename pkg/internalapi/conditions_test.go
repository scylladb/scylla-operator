package internalapi_test

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/internalapi"
)

func TestMakeDatacenterConditionFunc(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		conditionType     string
		dcName            string
		expectedCondition string
	}{
		{
			name:              "datacenter condition having provided condition type and dc name",
			conditionType:     "Progressing",
			dcName:            "dc1",
			expectedCondition: "Datacenterdc1Progressing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCondition := internalapi.MakeDatacenterConditionFunc(tc.conditionType)(tc.dcName)
			if gotCondition != tc.expectedCondition {
				t.Errorf("expected condition %q, got %q", tc.expectedCondition, gotCondition)
			}
		})
	}
}

func TestMakeKindControllerCondition(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		kind              string
		conditionType     string
		expectedCondition string
	}{
		{
			name:              "returns condition for kind controller having provided condition type and kind",
			kind:              "ConfigMap",
			conditionType:     "Progressing",
			expectedCondition: "ConfigMapControllerProgressing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCondition := internalapi.MakeKindControllerCondition(tc.kind, tc.conditionType)
			if gotCondition != tc.expectedCondition {
				t.Errorf("expected condition %q, got %q", tc.expectedCondition, gotCondition)
			}
		})
	}
}

func TestMakeKindFinalizerCondition(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		kind              string
		conditionType     string
		expectedCondition string
	}{
		{
			name:              "returns condition for kind finalizer having provided condition type and kind",
			kind:              "ConfigMap",
			conditionType:     "Progressing",
			expectedCondition: "ConfigMapFinalizerProgressing",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCondition := internalapi.MakeKindFinalizerCondition(tc.kind, tc.conditionType)
			if gotCondition != tc.expectedCondition {
				t.Errorf("expected condition %q, got %q", tc.expectedCondition, gotCondition)
			}
		})
	}
}
