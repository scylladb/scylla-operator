package scylladbdatacenter

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
)

func Test_detectVersionUpgrade(t *testing.T) {
	t.Parallel()

	newVersionedStatefulSet := func(version string) *appsv1.StatefulSet {
		sts := newStatefulSet("sts")
		if len(version) != 0 {
			sts.Labels = map[string]string{naming.ScyllaVersionLabel: version}
		}
		return sts
	}

	tt := []struct {
		name            string
		required        string
		existing        string
		expectedUpgrade bool
		expectedFrom    string
		expectedTo      string
		expectedErr     bool
	}{
		{
			name:            "no upgrade without version labels",
			required:        "",
			existing:        "6.2.0",
			expectedUpgrade: false,
		},
		{
			name:            "no upgrade for the same version",
			required:        "6.2.0",
			existing:        "6.2.0",
			expectedUpgrade: false,
		},
		{
			name:            "no upgrade for a patch version change",
			required:        "6.2.1",
			existing:        "6.2.0",
			expectedUpgrade: false,
		},
		{
			name:            "upgrade for a minor version change",
			required:        "6.3.0",
			existing:        "6.2.0",
			expectedUpgrade: true,
			expectedFrom:    "6.2.0",
			expectedTo:      "6.3.0",
		},
		{
			name:            "upgrade for a major version change",
			required:        "2025.1.0",
			existing:        "6.2.0",
			expectedUpgrade: true,
			expectedFrom:    "6.2.0",
			expectedTo:      "2025.1.0",
		},
		{
			name:        "fails on an unparsable version",
			required:    "latest",
			existing:    "6.2.0",
			expectedErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upgrade, from, to, err := detectVersionUpgrade(newVersionedStatefulSet(tc.required), newVersionedStatefulSet(tc.existing))
			if (err != nil) != tc.expectedErr {
				t.Fatalf("expected error: %t, got: %v", tc.expectedErr, err)
			}
			if upgrade != tc.expectedUpgrade || from != tc.expectedFrom || to != tc.expectedTo {
				t.Errorf("expected (%t, %q, %q), got (%t, %q, %q)", tc.expectedUpgrade, tc.expectedFrom, tc.expectedTo, upgrade, from, to)
			}
		})
	}
}
