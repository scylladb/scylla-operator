// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getScyllaDBManagerAgentAuthToken(t *testing.T) {
	t.Parallel()

	const mockGeneratedAuthToken = "mock-auth-token"
	newMockGeneratedAuthToken := func() string {
		return mockGeneratedAuthToken
	}

	tt := []struct {
		name                          string
		getAuthTokens                 []func() ([]metav1.Condition, string, error)
		expectedProgressingConditions []metav1.Condition
		expected                      string
		expectedErr                   error
	}{
		{
			name:                          "no auth token sources",
			expectedProgressingConditions: nil,
			expected:                      mockGeneratedAuthToken,
			expectedErr:                   nil,
		},
		{
			name: "single auth token source returns empty auth token",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "", nil
				},
			},
			expectedProgressingConditions: nil,
			expected:                      mockGeneratedAuthToken,
		},
		{
			name: "multiple auth token sources return empty auth token",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "", nil
				},
				func() ([]metav1.Condition, string, error) {
					return nil, "", nil
				},
			},
			expectedProgressingConditions: nil,
			expected:                      mockGeneratedAuthToken,
			expectedErr:                   nil,
		},
		{
			name: "first auth token source returns non-empty auth token and takes precedence",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "first-auth-token", nil
				},
				func() ([]metav1.Condition, string, error) {
					return nil, "", fmt.Errorf("second auth token source should not be called")
				},
			},
			expectedProgressingConditions: nil,
			expected:                      "first-auth-token",
			expectedErr:                   nil,
		},
		{
			name: "first auth token source returns progressing conditions",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return []metav1.Condition{
						{
							Type:               "SecretControllerProgressing",
							Status:             metav1.ConditionTrue,
							Reason:             "WaitingForSecret",
							Message:            `Waiting for Secret "test/first" to exist.`,
							ObservedGeneration: 1,
						},
					}, "", nil
				},
				func() ([]metav1.Condition, string, error) {
					return nil, "second-auth-token", nil
				},
			},
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:               "SecretControllerProgressing",
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            `Waiting for Secret "test/first" to exist.`,
					ObservedGeneration: 1,
				},
			},
			expected:    "",
			expectedErr: nil,
		},
		{
			name: "first auth token source returns error",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "", fmt.Errorf("first auth token source error")
				},
				func() ([]metav1.Condition, string, error) {
					return nil, "second-auth-token", nil
				},
			},
			expectedProgressingConditions: nil,
			expected:                      "",
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("first auth token source error")),
		},
		{
			name: "first auth token source returns empty auth token, second auth token source returns non-empty auth token",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "", nil
				},
				func() ([]metav1.Condition, string, error) {
					return nil, "second-auth-token", nil
				},
			},
			expectedProgressingConditions: nil,
			expected:                      "second-auth-token",
			expectedErr:                   nil,
		},
		{
			name: "first auth token source returns empty auth token, second auth token source returns progressing conditions",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "", nil
				},
				func() ([]metav1.Condition, string, error) {
					return []metav1.Condition{
						{
							Type:               "SecretControllerProgressing",
							Status:             metav1.ConditionTrue,
							Reason:             "WaitingForSecret",
							Message:            `Waiting for Secret "test/second" to exist.`,
							ObservedGeneration: 1,
						},
					}, "", nil
				},
			},
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:               "SecretControllerProgressing",
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            `Waiting for Secret "test/second" to exist.`,
					ObservedGeneration: 1,
				},
			},
			expected:    "",
			expectedErr: nil,
		},
		{
			name: "first auth token source returns empty auth token, second auth token source returns error",
			getAuthTokens: []func() ([]metav1.Condition, string, error){
				func() ([]metav1.Condition, string, error) {
					return nil, "", nil
				},
				func() ([]metav1.Condition, string, error) {
					return nil, "", fmt.Errorf("second auth token source error")
				},
			},
			expectedProgressingConditions: nil,
			expected:                      "",
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("second auth token source error")),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			progressingConditions, got, err := getScyllaDBManagerAgentAuthToken(
				newMockGeneratedAuthToken,
				tc.getAuthTokens...,
			)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !equality.Semantic.DeepEqual(progressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ:\n%s\n", cmp.Diff(tc.expectedProgressingConditions, progressingConditions))
			}

			if got != tc.expected {
				t.Errorf("expected auth token: %q, got %q", tc.expected, got)
			}
		})
	}
}
