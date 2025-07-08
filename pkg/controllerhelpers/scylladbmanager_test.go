// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_getScyllaDBManagerAgentAuthTokenConfig(t *testing.T) {
	t.Parallel()

	const mockAuthToken = "mock-auth-token"
	generateMockAuthToken := func() string {
		return mockAuthToken
	}

	getNilSecret := func() ([]metav1.Condition, *corev1.Secret, error) {
		return nil, nil, nil
	}

	tt := []struct {
		name                               string
		getOptionalCustomAgentConfigSecret func() ([]metav1.Condition, *corev1.Secret, error)
		getOptionalExistingAuthTokenSecret func() ([]metav1.Condition, *corev1.Secret, error)
		continueOnCustomAgentConfigError   bool
		expectedProgressingConditions      []metav1.Condition
		expected                           []byte
		expectedErr                        error
	}{
		{
			name:                               "no custom agent config, no existing auth token",
			getOptionalCustomAgentConfigSecret: getNilSecret,
			getOptionalExistingAuthTokenSecret: getNilSecret,
			continueOnCustomAgentConfigError:   false,
			expectedProgressingConditions:      nil,
			expected:                           []byte(`auth_token: ` + mockAuthToken + "\n"),
			expectedErr:                        nil,
		},
		// TODO: add test cases
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			progressingConditions, got, err := getScyllaDBManagerAgentAuthTokenConfig(
				tc.getOptionalCustomAgentConfigSecret,
				tc.getOptionalExistingAuthTokenSecret,
				tc.continueOnCustomAgentConfigError,
				generateMockAuthToken,
			)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !equality.Semantic.DeepEqual(progressingConditions, tc.expectedProgressingConditions) {
				t.Errorf("expected and got progressing conditions differ:\n%s\n", cmp.Diff(tc.expectedProgressingConditions, progressingConditions))
			}

			if !bytes.Equal(got, tc.expected) {
				t.Errorf("expected auth token config: %q, got %q", tc.expected, got)
			}
		})
	}
}
