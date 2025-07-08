// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"bytes"
	"errors"
	"fmt"
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
	newMockAuthToken := func() string {
		return mockAuthToken
	}

	getEmptyAuthToken := func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
		return nil, "", nil
	}

	newDefaultOptions := func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
		return GetScyllaDBManagerAgentAuthTokenConfigOptions{
			GetOptionalAgentAuthTokenFromCustomConfig: getEmptyAuthToken,
			GetOptionalAgentAuthTokenFromExisting:     getEmptyAuthToken,
		}
	}

	tt := []struct {
		name                          string
		options                       GetScyllaDBManagerAgentAuthTokenConfigOptions
		expectedProgressingConditions []metav1.Condition
		expected                      []byte
		expectedErr                   error
	}{
		{
			name:                          "no custom agent config, no existing auth token",
			options:                       newDefaultOptions(),
			expectedProgressingConditions: nil,
			expected:                      []byte(`auth_token: ` + mockAuthToken + "\n"),
			expectedErr:                   nil,
		},
		{
			name: "custom agent config, no existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "custom-auth-token", nil
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      []byte(`auth_token: custom-auth-token` + "\n"),
			expectedErr:                   nil,
		},
		{
			name: "progressing conditions from custom agent config func, no existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return []metav1.Condition{
						{
							Type:               "SecretControllerProgressing",
							Status:             metav1.ConditionTrue,
							Reason:             "WaitingForSecret",
							Message:            `Waiting for Secret "test/custom-agent-config" to exist.`,
							ObservedGeneration: 1,
						},
					}, "", nil
				}

				return opts
			}(),
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:               "SecretControllerProgressing",
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            `Waiting for Secret "test/custom-agent-config" to exist.`,
					ObservedGeneration: 1,
				},
			},
			expected:    nil,
			expectedErr: nil,
		},
		{
			name: "error from custom agent config func, no existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "", errors.New("can't get auth token from custom agent config")
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      nil,
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("can't get ScyllaDB Manager agent auth token from custom config secret: %w", errors.New("can't get auth token from custom agent config"))),
		},
		{
			name: "error from custom agent config extraction, no existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "custom-agent-config",
							Namespace: "test",
						},
						Data: map[string][]byte{
							// scylla-manager-agent.yaml is missing.
						},
					}

					authToken, err := extractFunc(secret)
					return nil, authToken, err
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      nil,
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("can't get ScyllaDB Manager agent auth token from custom config secret: %w", errors.New("secret \"test/custom-agent-config\" is missing \"scylla-manager-agent.yaml\" data"))),
		},
		{
			name: "no custom agent config, existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromExisting = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "existing-auth-token", nil
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      []byte(`auth_token: existing-auth-token` + "\n"),
			expectedErr:                   nil,
		},
		{
			name: "no custom agent config, progressing conditions from existing auth token func",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromExisting = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return []metav1.Condition{
						{
							Type:               "SecretControllerProgressing",
							Status:             metav1.ConditionTrue,
							Reason:             "WaitingForSecret",
							Message:            `Waiting for Secret "test/existing-auth-token" to exist.`,
							ObservedGeneration: 1,
						},
					}, "", nil
				}

				return opts
			}(),
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:               "SecretControllerProgressing",
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            `Waiting for Secret "test/existing-auth-token" to exist.`,
					ObservedGeneration: 1,
				},
			},
			expected:    nil,
			expectedErr: nil,
		},
		{
			name: "no custom agent config, error from existing auth token func",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromExisting = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "", errors.New("can't get existing auth token")
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      nil,
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("can't get ScyllaDB Manager agent auth token from existing secret: %w", errors.New("can't get existing auth token"))),
		},
		{
			name: "no custom agent config, error from existing auth token extraction",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromExisting = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "existing-auth-token",
							Namespace: "test",
						},
						Data: map[string][]byte{
							// auth-token.yaml is missing.
						},
					}

					authToken, err := extractFunc(secret)
					return nil, authToken, err
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      nil,
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("can't get ScyllaDB Manager agent auth token from existing secret: %w", errors.New("secret \"test/existing-auth-token\" is missing \"auth-token.yaml\" data"))),
		},
		{
			name: "custom agent config, existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "custom-auth-token", nil
				}
				opts.GetOptionalAgentAuthTokenFromExisting = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "existing-auth-token", nil
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      []byte(`auth_token: custom-auth-token` + "\n"),
			expectedErr:                   nil,
		},
		{
			name: "progressing conditions from custom agent config func, existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return []metav1.Condition{
						{
							Type:               "SecretControllerProgressing",
							Status:             metav1.ConditionTrue,
							Reason:             "WaitingForSecret",
							Message:            `Waiting for Secret "test/custom-agent-config" to exist.`,
							ObservedGeneration: 1,
						},
					}, "", nil
				}
				opts.GetOptionalAgentAuthTokenFromExisting = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "existing-auth-token", nil
				}

				return opts
			}(),
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:               "SecretControllerProgressing",
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            `Waiting for Secret "test/custom-agent-config" to exist.`,
					ObservedGeneration: 1,
				},
			},
			expected:    nil,
			expectedErr: nil,
		},
		{
			name: "error from custom agent config func, existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "", errors.New("can't get auth token from custom agent config")
				}
				opts.GetOptionalAgentAuthTokenFromExisting = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "existing-auth-token", nil
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      nil,
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("can't get ScyllaDB Manager agent auth token from custom config secret: %w", errors.New("can't get auth token from custom agent config"))),
		},
		{
			name: "error from custom agent config extraction, existing auth token",
			options: func() GetScyllaDBManagerAgentAuthTokenConfigOptions {
				opts := newDefaultOptions()

				opts.GetOptionalAgentAuthTokenFromCustomConfig = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "custom-agent-config",
							Namespace: "test",
						},
						Data: map[string][]byte{
							// scylla-manager-agent.yaml is missing.
						},
					}

					authToken, err := extractFunc(secret)
					return nil, authToken, err
				}
				opts.GetOptionalAgentAuthTokenFromExisting = func(extractFunc func(*corev1.Secret) (string, error)) ([]metav1.Condition, string, error) {
					return nil, "existing-auth-token", nil
				}

				return opts
			}(),
			expectedProgressingConditions: nil,
			expected:                      nil,
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", fmt.Errorf("can't get ScyllaDB Manager agent auth token from custom config secret: %w", errors.New("secret \"test/custom-agent-config\" is missing \"scylla-manager-agent.yaml\" data"))),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			progressingConditions, got, err := getScyllaDBManagerAgentAuthTokenConfig(
				newMockAuthToken,
				tc.options,
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

func Test_GetScyllaDBManagerAgentAuthTokenFromSecret(t *testing.T) {
	t.Parallel()

	getMockAuthTokenSecret := func() ([]metav1.Condition, *corev1.Secret, error) {
		return nil, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock-auth-token",
				Namespace: "test",
			},
			Data: map[string][]byte{
				"mock": []byte("mock-auth-token"),
			},
			Type: corev1.SecretTypeOpaque,
		}, nil
	}

	extractMockAuthTokenFromSecret := func(secret *corev1.Secret) (string, error) {
		if secret == nil {
			return "", fmt.Errorf("secret is nil")
		}
		authToken, ok := secret.Data["mock"]
		if !ok {
			return "", fmt.Errorf("auth token is missing")
		}

		return string(authToken), nil
	}

	tt := []struct {
		name                          string
		getOptionalAuthTokenSecret    func() ([]metav1.Condition, *corev1.Secret, error)
		extractAuthTokenFromSecret    func(*corev1.Secret) (string, error)
		expectedProgressingConditions []metav1.Condition
		expected                      string
		expectedErr                   error
	}{
		{
			name: "no auth token secret",
			getOptionalAuthTokenSecret: func() ([]metav1.Condition, *corev1.Secret, error) {
				return nil, nil, nil
			},
			extractAuthTokenFromSecret:    extractMockAuthTokenFromSecret,
			expectedProgressingConditions: nil,
			expected:                      "",
			expectedErr:                   nil,
		},
		{
			name:                          "auth token secret with valid data",
			getOptionalAuthTokenSecret:    getMockAuthTokenSecret,
			extractAuthTokenFromSecret:    extractMockAuthTokenFromSecret,
			expectedProgressingConditions: nil,
			expected:                      "mock-auth-token",
			expectedErr:                   nil,
		},
		{
			name: "error from auth token secret func",
			getOptionalAuthTokenSecret: func() ([]metav1.Condition, *corev1.Secret, error) {
				return nil, nil, fmt.Errorf("can't get auth token secret")
			},
			extractAuthTokenFromSecret:    extractMockAuthTokenFromSecret,
			expectedProgressingConditions: nil,
			expected:                      "",
			expectedErr:                   fmt.Errorf("can't get ScyllaDB Manager agent auth token secret: %w", fmt.Errorf("can't get auth token secret")),
		},
		{
			name: "progressing conditions from auth token secret func",
			getOptionalAuthTokenSecret: func() ([]metav1.Condition, *corev1.Secret, error) {
				return []metav1.Condition{
					{
						Type:               "SecretControllerProgressing",
						Status:             metav1.ConditionTrue,
						Reason:             "WaitingForSecret",
						Message:            `Waiting for Secret "test/mock-auth-token" to exist.`,
						ObservedGeneration: 1,
					},
				}, nil, nil
			},
			extractAuthTokenFromSecret: extractMockAuthTokenFromSecret,
			expectedProgressingConditions: []metav1.Condition{
				{
					Type:               "SecretControllerProgressing",
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            `Waiting for Secret "test/mock-auth-token" to exist.`,
					ObservedGeneration: 1,
				},
			},
			expected:    "",
			expectedErr: nil,
		},
		{
			name: "error from auth token extraction",
			getOptionalAuthTokenSecret: func() ([]metav1.Condition, *corev1.Secret, error) {
				return nil, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-auth-token",
						Namespace: "test",
					},
					Data: map[string][]byte{
						// "mock" key is missing.
					},
					Type: corev1.SecretTypeOpaque,
				}, nil
			},
			extractAuthTokenFromSecret:    extractMockAuthTokenFromSecret,
			expectedProgressingConditions: nil,
			expected:                      "",
			expectedErr:                   fmt.Errorf(`can't extract ScyllaDB Manager agent auth token from Secret "test/mock-auth-token": %w`, fmt.Errorf("auth token is missing")),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			progressingConditions, got, err := GetScyllaDBManagerAgentAuthTokenFromSecret(tc.getOptionalAuthTokenSecret, tc.extractAuthTokenFromSecret)

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
