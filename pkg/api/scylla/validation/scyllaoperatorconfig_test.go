// Copyright (c) 2023 ScyllaDB.

package validation_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateScyllaOperatorConfig(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                 string
		ScyllaOperatorConfig *scyllav1alpha1.ScyllaOperatorConfig
		expectedErrorList    field.ErrorList
		expectedErrorString  string
	}{
		{
			name:                 "empty config is valid",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{},
			expectedErrorList:    nil,
			expectedErrorString:  "",
		},
		{
			name: "UnsupportedGrafanaImageOverride can't be empty",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedGrafanaImageOverride: pointer.Ptr(""),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.unsupportedGrafanaImageOverride",
					BadValue: "",
					Detail:   "image reference can't be empty",
				},
			},
			expectedErrorString: `spec.unsupportedGrafanaImageOverride: Required value: image reference can't be empty`,
		},
		{
			name: "UnsupportedGrafanaImageOverride can't contain only spaces",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedGrafanaImageOverride: pointer.Ptr(" "),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.unsupportedGrafanaImageOverride",
					BadValue: " ",
					Detail:   "unable to parse image: invalid reference format",
				},
			},
			expectedErrorString: `spec.unsupportedGrafanaImageOverride: Invalid value: " ": unable to parse image: invalid reference format`,
		},
		{
			name: "UnsupportedGrafanaImageOverride accepts valid image",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedGrafanaImageOverride: pointer.Ptr("docker.io/grafana/grafana"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "UnsupportedGrafanaImageOverride accepts valid image with a tag",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedGrafanaImageOverride: pointer.Ptr("docker.io/grafana/grafana:9.5.12"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "UnsupportedGrafanaImageOverride accepts valid image using sha",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedGrafanaImageOverride: pointer.Ptr("docker.io/grafana/grafana:9.5.12@sha256:7d2f2a8b7aebe30bf3f9ae0f190e508e571b43f65753ba3b1b1adf0800bc9256"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "UnsupportedBashToolsImageOverride can't be empty",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedBashToolsImageOverride: pointer.Ptr(""),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.unsupportedBashToolsImageOverride",
					BadValue: "",
					Detail:   "image reference can't be empty",
				},
			},
			expectedErrorString: `spec.unsupportedBashToolsImageOverride: Required value: image reference can't be empty`,
		},
		{
			name: "UnsupportedBashToolsImageOverride can't contain only spaces",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedBashToolsImageOverride: pointer.Ptr(" "),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.unsupportedBashToolsImageOverride",
					BadValue: " ",
					Detail:   "unable to parse image: invalid reference format",
				},
			},
			expectedErrorString: `spec.unsupportedBashToolsImageOverride: Invalid value: " ": unable to parse image: invalid reference format`,
		},
		{
			name: "UnsupportedBashToolsImageOverride accepts valid image",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedBashToolsImageOverride: pointer.Ptr("docker.io/grafana/grafana"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "UnsupportedBashToolsImageOverride accepts valid image with a tag",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedBashToolsImageOverride: pointer.Ptr("docker.io/grafana/grafana:9.5.12"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "UnsupportedBashToolsImageOverride accepts valid image using sha",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedBashToolsImageOverride: pointer.Ptr("docker.io/grafana/grafana:9.5.12@sha256:7d2f2a8b7aebe30bf3f9ae0f190e508e571b43f65753ba3b1b1adf0800bc9256"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "UnsupportedPrometheusImageOverride can't be empty",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedPrometheusVersionOverride: pointer.Ptr(""),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.unsupportedPrometheusVersionOverride",
					BadValue: "",
					Detail:   "version can't be empty",
				},
			},
			expectedErrorString: `spec.unsupportedPrometheusVersionOverride: Required value: version can't be empty`,
		},
		{
			name: "UnsupportedPrometheusVersionOverride can't contain only spaces",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedPrometheusVersionOverride: pointer.Ptr(" "),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.unsupportedPrometheusVersionOverride",
					BadValue: " ",
					Detail:   "version contains only spaces",
				},
			},
			expectedErrorString: `spec.unsupportedPrometheusVersionOverride: Invalid value: " ": version contains only spaces`,
		},
		{
			name: "UnsupportedPrometheusVersionOverride accepts valid version",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					UnsupportedPrometheusVersionOverride: pointer.Ptr("v2.42.0"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "ConfiguredClusterDomain accepts valid domain",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					ConfiguredClusterDomain: pointer.Ptr("cluster.local"),
				},
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "ConfiguredClusterDomain can't be empty",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					ConfiguredClusterDomain: pointer.Ptr(""),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.configuredClusterDomain",
					BadValue: "",
				},
			},
			expectedErrorString: `spec.configuredClusterDomain: Required value`,
		},
		{
			name: "ConfiguredClusterDomain must match label value regex",
			ScyllaOperatorConfig: &scyllav1alpha1.ScyllaOperatorConfig{
				Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
					ConfiguredClusterDomain: pointer.Ptr("-foo"),
				},
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.configuredClusterDomain",
					BadValue: "-foo",
					Detail:   `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
				},
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.configuredClusterDomain",
					BadValue: "-foo",
					Detail:   `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`,
				},
			},
			expectedErrorString: `[spec.configuredClusterDomain: Invalid value: "-foo": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'), spec.configuredClusterDomain: Invalid value: "-foo": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := validation.ValidateScyllaOperatorConfig(tc.ScyllaOperatorConfig)
			if !equality.Semantic.DeepEqual(errList, tc.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(tc.expectedErrorList, errList))
			}

			errStr := ""
			agg := errList.ToAggregate()
			if agg != nil {
				errStr = agg.Error()
			}
			if !equality.Semantic.DeepEqual(errStr, tc.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}
		})
	}
}
