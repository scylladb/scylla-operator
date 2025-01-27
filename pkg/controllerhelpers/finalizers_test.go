// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddFinalizerPatch(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		obj           metav1.Object
		finalizer     string
		expectedPatch []byte
		expectedError error
	}{
		{
			name:          "object has empty finalizers",
			obj:           &metav1.ObjectMeta{ResourceVersion: "123", Finalizers: []string{}},
			finalizer:     "my-finalizer",
			expectedPatch: []byte(`{"metadata":{"resourceVersion":"123","finalizers":["my-finalizer"]}}`),
			expectedError: nil,
		},
		{
			name:          "duplicate finalizer",
			obj:           &metav1.ObjectMeta{ResourceVersion: "123", Finalizers: []string{"a", "my-finalizer", "c"}},
			finalizer:     "my-finalizer",
			expectedPatch: nil,
			expectedError: nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			patch, err := AddFinalizerPatch(tc.obj, tc.finalizer)
			if err != tc.expectedError {
				t.Errorf("expected error %s, got %s", tc.expectedError, err)
			}
			if !equality.Semantic.DeepEqual(patch, tc.expectedPatch) {
				t.Errorf("expected patch %s, got %s", string(tc.expectedPatch), string(patch))
			}
		})
	}
}

func TestRemoveFinalizerPatch(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name          string
		obj           metav1.Object
		finalizer     string
		expectedPatch []byte
		expectedError error
	}{
		{
			name:          "object is missing given finalizer",
			obj:           &metav1.ObjectMeta{ResourceVersion: "123", Finalizers: []string{}},
			finalizer:     "my-finalizer",
			expectedPatch: nil,
			expectedError: nil,
		},
		{
			name:          "patch removes finalizer",
			obj:           &metav1.ObjectMeta{ResourceVersion: "123", Finalizers: []string{"a", "my-finalizer", "c"}},
			finalizer:     "my-finalizer",
			expectedPatch: []byte(`{"metadata":{"resourceVersion":"123","finalizers":["a","c"]}}`),
			expectedError: nil,
		},
		{
			name:          "patch removes last finalizer",
			obj:           &metav1.ObjectMeta{ResourceVersion: "123", Finalizers: []string{"my-finalizer"}},
			finalizer:     "my-finalizer",
			expectedPatch: []byte(`{"metadata":{"resourceVersion":"123","finalizers":null}}`),
			expectedError: nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			patch, err := RemoveFinalizerPatch(tc.obj, tc.finalizer)
			if err != tc.expectedError {
				t.Errorf("expected error %s, got %s", tc.expectedError, err)
			}
			if !equality.Semantic.DeepEqual(patch, tc.expectedPatch) {
				t.Errorf("expected patch %s, got %s", string(tc.expectedPatch), string(patch))
			}
		})
	}
}

func TestHasFinalizer(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		obj            metav1.Object
		finalizer      string
		expectedResult bool
	}{
		{
			name:           "false when empty finalizer list",
			obj:            &metav1.ObjectMeta{Finalizers: []string{}},
			finalizer:      "my-finalizer",
			expectedResult: false,
		},
		{
			name:           "true when object contains finalizer",
			obj:            &metav1.ObjectMeta{Finalizers: []string{"a", "b", "my-finalizer", "c"}},
			finalizer:      "my-finalizer",
			expectedResult: true,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := HasFinalizer(tc.obj, tc.finalizer)
			if got != tc.expectedResult {
				t.Errorf("expected %v, got %v", tc.expectedResult, got)
			}
		})
	}
}
