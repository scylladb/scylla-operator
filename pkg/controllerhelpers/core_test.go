package controllerhelpers

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
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

// https://github.com/kubernetes/apimachinery/blob/f7c43800319c674eecce7c80a6ac7521a9b50aa8/pkg/apis/meta/v1/types.go#L1642
var reasonRegex = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`)

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
				Reason:             "AsExpected",
				Message:            "AsExpected",
			},
			expectedCondition: metav1.Condition{
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 42,
				Reason:             "AsExpected",
				Message:            "AsExpected",
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
				Reason:             "UnknownAvailableFooReason,UnknownAvailableBarReason",
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
				Reason:             "FooAvailableReason,BarAvailableReason",
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
					Reason:  "UnknownDegradedBarReason",
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
				Reason:             "UnknownDegradedFooReason,UnknownDegradedBarReason",
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
				Reason:             "FooDegradedReason,BarDegradedReason",
				Message:            "FooDegraded: FooDegraded error\nBarDegraded: BarDegraded error",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "aggregated condition's message exceeds API limit",
			conditions: []metav1.Condition{
				{
					Type:    "FooDegraded",
					Status:  metav1.ConditionTrue,
					Reason:  "FooDegradedReason",
					Message: strings.Repeat("a", maxMessageLength/2),
				},
				{
					Type:    "BarDegraded",
					Status:  metav1.ConditionTrue,
					Reason:  "BarDegradedReason",
					Message: strings.Repeat("b", maxMessageLength/2+1),
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
				Reason:             "FooDegradedReason,BarDegradedReason",
				Message:            "FooDegraded: " + strings.Repeat("a", maxMessageLength/2) + "\n(1 more omitted)",
				ObservedGeneration: 42,
			},
			expectedError: nil,
		},
		{
			name: "aggregated condition's reason exceeds API limit",
			conditions: []metav1.Condition{
				{
					Type:    "FooDegraded",
					Status:  metav1.ConditionTrue,
					Reason:  strings.Repeat("a", maxReasonLength/2),
					Message: "FooDegraded error",
				},
				{
					Type:    "BarDegraded",
					Status:  metav1.ConditionTrue,
					Reason:  strings.Repeat("b", maxReasonLength/2+1),
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
				Message:            "FooDegraded: FooDegraded error\nBarDegraded: BarDegraded error",
				Reason:             strings.Repeat("a", maxReasonLength/2) + ",And1MoreOmitted",
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

			if err == nil && !reasonRegex.MatchString(got.Reason) {
				t.Errorf("aggregated reason %q doesn't match regex %q", got.Reason, reasonRegex.String())
			}
		})
	}
}

func TestJoinWithLimit(t *testing.T) {
	t.Parallel()

	const testSep = ","

	tt := []struct {
		name      string
		elems     []string
		separator string
		limit     int
		expected  string
	}{
		{
			name:      "empty slice",
			elems:     []string{},
			separator: testSep,
			limit:     1,
			expected:  "",
		},
		{
			name:      "nil slice",
			elems:     nil,
			separator: testSep,
			limit:     1,
			expected:  "",
		},
		{
			name: "single element exceeds limit",
			elems: []string{
				"A_20__CharactersLong",
			},
			separator: testSep,
			limit:     20 - 1, // 1 less than the length of the element
			expected:  "And1MoreOmitted",
		},
		{
			name: "joined length is less than limit",
			elems: []string{
				"A_20__CharactersLong",
				"B_20__CharactersLong",
			},
			separator: testSep,
			limit:     20*2 + 1 + 10, // 10 more than sum of elements length + separator
			expected:  "A_20__CharactersLong,B_20__CharactersLong",
		},
		{
			name: "joined length is equal to limit",
			elems: []string{
				"A_20__CharactersLong",
				"B_20__CharactersLong",
			},
			separator: testSep,
			limit:     20*2 + 1, // sum of elements length + separator
			expected:  "A_20__CharactersLong,B_20__CharactersLong",
		},
		{
			name: "joined length is greater than limit",
			elems: []string{
				"A_20__CharactersLong",
				"B_20__CharactersLong",
			},
			separator: testSep,
			limit:     20 * 2, // 1 less than the sum of elements length + separator
			expected:  "A_20__CharactersLong,And1MoreOmitted",
		},
		{
			name: "joined length is greater than limit and omission indicator doesn't fit",
			elems: []string{
				"A_20__CharactersLong",
				// This elem will be omitted as well as the next omission indicator wouldn't fit if we added it.
				"B_20__CharactersLong",
				"C_20__CharactersLong",
			},
			separator: testSep,
			limit:     20*2 + 1, // sum of two first elements length + separator, but not enough space for the omission indicator
			expected:  "A_20__CharactersLong,And2MoreOmitted",
		},
		{
			name: "joined length is greater than limit and omission indicator fits perfectly",
			elems: []string{
				"A_20__CharactersLong",
				// This elem will be included because the next omission indicator fits just fine.
				"B_20__CharactersLong",
				"C_20__CharactersLong",
			},
			separator: testSep,
			limit:     20*2 + 1 + 16, // sum of two first elements length + separator + omission indicator
			expected:  "A_20__CharactersLong,B_20__CharactersLong,And1MoreOmitted",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := joinWithLimit(tc.elems, joinWithLimitOptions{
				separator:            tc.separator,
				limit:                tc.limit,
				omissionIndicatorFmt: reasonOmissionIndicator,
			})
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
			if len(got) > tc.limit {
				t.Errorf("expected joined length to be less than limit of %d, got %d", tc.limit, len(got))
			}
		})
	}
}
