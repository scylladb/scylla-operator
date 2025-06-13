// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrepareSetAnnotationPatch(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		obj             metav1.Object
		annotationKey   string
		annotationValue *string
		expected        []byte
		expectedErr     error
	}{
		{
			name:            "add new annotation to empty annotations",
			obj:             &metav1.ObjectMeta{ResourceVersion: "123", Annotations: nil},
			annotationKey:   "my-annotation",
			annotationValue: pointer.Ptr("my-value"),
			expected:        []byte(`{"metadata":{"resourceVersion":"123","annotations":{"my-annotation":"my-value"}}}`),
			expectedErr:     nil,
		},
		{
			name:            "add new annotation to existing annotations",
			obj:             &metav1.ObjectMeta{ResourceVersion: "123", Annotations: map[string]string{"existing": "value"}},
			annotationKey:   "my-annotation",
			annotationValue: pointer.Ptr("my-value"),
			expected:        []byte(`{"metadata":{"resourceVersion":"123","annotations":{"existing":"value","my-annotation":"my-value"}}}`),
			expectedErr:     nil,
		},
		{
			name:            "update existing annotation",
			obj:             &metav1.ObjectMeta{ResourceVersion: "123", Annotations: map[string]string{"my-annotation": "old-value"}},
			annotationKey:   "my-annotation",
			annotationValue: pointer.Ptr("new-value"),
			expected:        []byte(`{"metadata":{"resourceVersion":"123","annotations":{"my-annotation":"new-value"}}}`),
			expectedErr:     nil,
		},
		{
			name:            "remove annotation by setting nil",
			obj:             &metav1.ObjectMeta{ResourceVersion: "123", Annotations: map[string]string{"my-annotation": "old-value"}},
			annotationKey:   "my-annotation",
			annotationValue: nil,
			expected:        []byte(`{"metadata":{"resourceVersion":"123","annotations":{"my-annotation":null}}}`),
			expectedErr:     nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			patch, err := PrepareSetAnnotationPatch(tc.obj, tc.annotationKey, tc.annotationValue)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if !equality.Semantic.DeepEqual(patch, tc.expected) {
				t.Errorf("expected and got differ: %s", cmp.Diff(tc.expected, patch))
			}
		})
	}
}

func TestHasMatchingAnnotation(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		obj             metav1.Object
		annotationKey   string
		annotationValue string
		expected        bool
	}{
		{
			name:            "empty annotations",
			obj:             &metav1.ObjectMeta{Annotations: nil},
			annotationKey:   "my-annotation",
			annotationValue: "my-value",
			expected:        false,
		},
		{
			name:            "annotation doesn't exist",
			obj:             &metav1.ObjectMeta{Annotations: map[string]string{"other": "value"}},
			annotationKey:   "my-annotation",
			annotationValue: "my-value",
			expected:        false,
		},
		{
			name:            "annotation exists with different value",
			obj:             &metav1.ObjectMeta{Annotations: map[string]string{"my-annotation": "different-value"}},
			annotationKey:   "my-annotation",
			annotationValue: "my-value",
			expected:        false,
		},
		{
			name:            "annotation exists with matching value",
			obj:             &metav1.ObjectMeta{Annotations: map[string]string{"my-annotation": "my-value"}},
			annotationKey:   "my-annotation",
			annotationValue: "my-value",
			expected:        true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := HasMatchingAnnotation(tc.obj, tc.annotationKey, tc.annotationValue)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
