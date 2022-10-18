package controllerhelpers

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFindStatusConditionsWithSuffix(t *testing.T) {
	tt := []struct {
		name       string
		conditions []metav1.Condition
		suffix     string
		expected   []metav1.Condition
	}{
		{
			name:       "won't panic with nil array",
			conditions: nil,
			suffix:     "",
			expected:   nil,
		},
		{
			name:   "finds matching elements",
			suffix: "Progressing",
			conditions: []metav1.Condition{
				{
					Type:    "FooProgressing",
					Status:  metav1.ConditionTrue,
					Reason:  "FooProgressingReason",
					Message: "FooProgressing message",
				},
				{
					Type:    "Progressing",
					Status:  metav1.ConditionTrue,
					Reason:  "ProgressingReason",
					Message: "Progressing message",
				},
				{
					Type:    "BarProgressing",
					Status:  metav1.ConditionFalse,
					Reason:  "BarProgressingReason",
					Message: "BarProgressing message",
				},
			},
			expected: []metav1.Condition{
				{
					Type:    "FooProgressing",
					Status:  metav1.ConditionTrue,
					Reason:  "FooProgressingReason",
					Message: "FooProgressing message",
				},
				{
					Type:    "BarProgressing",
					Status:  metav1.ConditionFalse,
					Reason:  "BarProgressingReason",
					Message: "BarProgressing message",
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := FindStatusConditionsWithSuffix(tc.conditions, tc.suffix)
			if !apiequality.Semantic.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestAggregateStatusConditions(t *testing.T) {
	tt := []struct {
		name              string
		conditions        []metav1.Condition
		condition         metav1.Condition
		expectedCondition metav1.Condition
		expectedError     error
	}{
		{
			name:       "won't panic with nil array",
			conditions: nil,
			condition: metav1.Condition{
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name:       "will error on unknown initial condition",
			conditions: nil,
			condition: metav1.Condition{
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{},
			expectedError:     errors.New(`unsupported default value "Unknown"`),
		},
		{
			name: "positive condition with all true results in the initial condition",
			conditions: []metav1.Condition{
				{
					Type:    "FooAvailable",
					Status:  metav1.ConditionTrue,
					Reason:  "FooAvailableReason",
					Message: "FooAvailable ok",
				},
				{
					Type:    "BarAvailable",
					Status:  metav1.ConditionTrue,
					Reason:  "BarAvailableReason",
					Message: "BarAvailable ok",
				},
			},
			condition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "positive condition with true and unknown results in an unknown condition",
			conditions: []metav1.Condition{
				{
					Type:    "FooAvailable",
					Status:  metav1.ConditionTrue,
					Reason:  "FooAvailableReason",
					Message: "FooAvailable ok",
				},
				{
					Type:    "BarAvailable",
					Status:  metav1.ConditionTrue,
					Reason:  "BarAvailableReason",
					Message: "BarAvailable ok",
				},
				{
					Type:    "UnknownAvailableFoo",
					Status:  metav1.ConditionUnknown,
					Reason:  "UnknownAvailableFooReason",
					Message: "UnknownAvailableFoo message",
				},
				{
					Type:    "UnknownAvailableBar",
					Status:  metav1.ConditionUnknown,
					Reason:  "UnknownAvailableBarReason",
					Message: "UnknownAvailableBar message",
				},
			},
			condition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionUnknown,
				Reason:             "UnknownAvailableFoo_UnknownAvailableFooReason,UnknownAvailableBar_UnknownAvailableBarReason",
				Message:            "UnknownAvailableFoo: UnknownAvailableFoo message\nUnknownAvailableBar: UnknownAvailableBar message",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "positive condition with some false aggregates the false states",
			conditions: []metav1.Condition{
				{
					Type:    "FooAvailable",
					Status:  metav1.ConditionFalse,
					Reason:  "FooAvailableReason",
					Message: "FooAvailable error",
				},
				{
					Type:    "GoodAvailable",
					Status:  metav1.ConditionTrue,
					Reason:  "GoodAvailableReason",
					Message: "GoodAvailable ok",
				},
				{
					Type:    "UnknownAvailable",
					Status:  metav1.ConditionTrue,
					Reason:  "UnknownAvailableReason",
					Message: "UnknownAvailable ok",
				},
				{
					Type:    "BarAvailable",
					Status:  metav1.ConditionFalse,
					Reason:  "BarAvailableReason",
					Message: "BarAvailable error",
				},
			},
			condition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionFalse,
				Reason:             "FooAvailable_FooAvailableReason,BarAvailable_BarAvailableReason",
				Message:            "FooAvailable: FooAvailable error\nBarAvailable: BarAvailable error",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "negative condition with all false results in the initial condition",
			conditions: []metav1.Condition{
				{
					Type:    "FooDegraded",
					Status:  metav1.ConditionFalse,
					Reason:  "FooDegradedReason",
					Message: "FooDegraded ok",
				},
				{
					Type:    "BarDegraded",
					Status:  metav1.ConditionFalse,
					Reason:  "BarDegradedReason",
					Message: "BarDegraded ok",
				},
			},
			condition: metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionFalse,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionFalse,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "negative condition with false and unknown results in an unknown condition",
			conditions: []metav1.Condition{
				{
					Type:    "FooDegraded",
					Status:  metav1.ConditionFalse,
					Reason:  "FooDegradedReason",
					Message: "FooDegraded ok",
				},
				{
					Type:    "BarDegraded",
					Status:  metav1.ConditionFalse,
					Reason:  "BarDegradedReason",
					Message: "BarDegraded ok",
				},
				{
					Type:    "UnknownDegradedFoo",
					Status:  metav1.ConditionUnknown,
					Reason:  "UnknownDegradedFooReason",
					Message: "UnknownDegradedFoo message",
				},
				{
					Type:    "UnknownDegradedBar",
					Status:  metav1.ConditionUnknown,
					Reason:  "UnknownDegradedBar reason",
					Message: "UnknownDegradedBar message",
				},
			},
			condition: metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionFalse,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionUnknown,
				Reason:             "UnknownDegradedFoo_UnknownDegradedFooReason,UnknownDegradedBar_UnknownDegradedBar reason",
				Message:            "UnknownDegradedFoo: UnknownDegradedFoo message\nUnknownDegradedBar: UnknownDegradedBar message",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "negative condition with some true aggregates the true states",
			conditions: []metav1.Condition{
				{
					Type:    "FooDegraded",
					Status:  metav1.ConditionTrue,
					Reason:  "FooDegradedReason",
					Message: "FooDegraded error",
				},
				{
					Type:    "GoodDegraded",
					Status:  metav1.ConditionFalse,
					Reason:  "GoodDegradedReason",
					Message: "GoodDegraded ok",
				},
				{
					Type:    "UnknownDegraded",
					Status:  metav1.ConditionFalse,
					Reason:  "UnknownDegradedReason",
					Message: "UnknownDegraded ok",
				},
				{
					Type:    "BarDegraded",
					Status:  metav1.ConditionTrue,
					Reason:  "BarDegradedReason",
					Message: "BarDegraded error",
				},
			},
			condition: metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionFalse,
				Reason:             "AsExpected",
				Message:            "AsExpected",
				ObservedGeneration: 42,
			},
			expectedCondition: metav1.Condition{
				Type:               "Degraded",
				Status:             metav1.ConditionTrue,
				Reason:             "FooDegraded_FooDegradedReason,BarDegraded_BarDegradedReason",
				Message:            "FooDegraded: FooDegraded error\nBarDegraded: BarDegraded error",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AggregateStatusConditions(tc.conditions, tc.condition)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#v, got %#v", tc.expectedError, err)
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedCondition) {
				t.Errorf("expected and got differ: %s", cmp.Diff(tc.expectedCondition, got))
			}
		})
	}
}
