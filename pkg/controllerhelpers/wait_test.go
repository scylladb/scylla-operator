package controllerhelpers

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func hasKeyFunc(name string) func(*corev1.ConfigMap) (bool, error) {
	return func(cm *corev1.ConfigMap) (bool, error) {
		_, found := cm.Data[name]
		return found, nil
	}
}

func failedFunc() func(*corev1.ConfigMap) (bool, error) {
	return func(cm *corev1.ConfigMap) (bool, error) {
		return false, fmt.Errorf("an error has occurred")
	}
}

func TestAggregatedConditions_Condition(t *testing.T) {
	t.Parallel()
	obj := &corev1.ConfigMap{
		Data: map[string]string{
			"alpha": "foo",
			"beta":  "bar",
		},
	}

	tt := []struct {
		name           string
		ac             *AggregatedConditions[*corev1.ConfigMap]
		expectedDone   bool
		expectedString string
		expectedErr    error
	}{
		{
			name:           "single done condition is done",
			ac:             NewAggregatedConditions[*corev1.ConfigMap](hasKeyFunc("alpha")),
			expectedDone:   true,
			expectedString: "[true]",
			expectedErr:    nil,
		},
		{
			name:           "done+undone condition is undone",
			ac:             NewAggregatedConditions[*corev1.ConfigMap](hasKeyFunc("alpha"), hasKeyFunc("doesnotexist")),
			expectedDone:   false,
			expectedString: "[true,false]",
			expectedErr:    nil,
		},
		{
			name:           "done+failed+done condition is undone",
			ac:             NewAggregatedConditions[*corev1.ConfigMap](hasKeyFunc("alpha"), failedFunc(), hasKeyFunc("alpha")),
			expectedDone:   false,
			expectedString: "[true,false,<nil>]",
			expectedErr:    fmt.Errorf("an error has occurred"),
		},
		{
			name:           "done+undone+done condition is undone",
			ac:             NewAggregatedConditions[*corev1.ConfigMap](hasKeyFunc("alpha"), hasKeyFunc("doesnotexist"), hasKeyFunc("beta")),
			expectedDone:   false,
			expectedString: "[true,false,true]",
			expectedErr:    nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.ac.Condition(obj)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v: diff:\n%s", tc.expectedErr, err, cmp.Diff(tc.expectedErr, got))
			}

			if got != tc.expectedDone {
				t.Errorf("expected done %t, got %t", tc.expectedDone, got)
			}

			s := tc.ac.GetStateString()
			if s != tc.expectedString {
				t.Errorf("expected string %q, got %q", tc.expectedString, s)
			}
		})
	}
}
