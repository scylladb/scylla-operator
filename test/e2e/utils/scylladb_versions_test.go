// Copyright (C) 2026 ScyllaDB

package utils

import (
	"testing"
)

func TestFindScyllaDBUpdateAndUpgradeFromVersions(t *testing.T) {
	t.Parallel()

	tags := []string{
		"latest",
		"2026.3",
		"2025.4.2",
		"2025.4.3",
		"2026.1.0-rc0",
		"2026.1.0",
		"2026.2.5",
		"2026.2.6",
		"2026.2.7-rc0",
		"2026.3.0-rc0",
		"2026.3.0-rc1",
		"2026.3.0",
		"2026.3.3",
		"2026.3.4",
		"2026.4.0-rc0",
		"2026.4.0-rc1",
		"2026.4.0-rc9",
		"2026.4.0-rc10",
		"2026.4.0-rc.2",
		"2026.4.0-dev-0.20261020",
		"2027.1.0-rc0",
	}

	tt := []struct {
		target          string
		wantUpdateFrom  string
		wantUpgradeFrom string
	}{
		{
			target:          "2026.3.4",
			wantUpdateFrom:  "2026.3.3",
			wantUpgradeFrom: "2026.2.7-rc0",
		},
		{
			target:          "2026.3.0",
			wantUpdateFrom:  "2026.3.0-rc1",
			wantUpgradeFrom: "2026.2.7-rc0",
		},
		{
			target:          "2026.3.0-rc0",
			wantUpdateFrom:  "",
			wantUpgradeFrom: "2026.2.7-rc0",
		},
		{
			target:          "2026.4.0-rc1",
			wantUpdateFrom:  "2026.4.0-rc0",
			wantUpgradeFrom: "2026.3.4",
		},
		{
			target:          "2026.4.0-rc10",
			wantUpdateFrom:  "2026.4.0-rc9",
			wantUpgradeFrom: "2026.3.4",
		},
		{
			target:          "2026.4.0-rc11",
			wantUpdateFrom:  "2026.4.0-rc10",
			wantUpgradeFrom: "2026.3.4",
		},
		{
			target:          "2027.1.0-rc0",
			wantUpdateFrom:  "",
			wantUpgradeFrom: "2026.4.0-rc10",
		},
		{
			target:          "2026.1.0",
			wantUpdateFrom:  "2026.1.0-rc0",
			wantUpgradeFrom: "2025.4.3",
		},
		{
			target:          "2025.4.2",
			wantUpdateFrom:  "",
			wantUpgradeFrom: "",
		},
		{
			target:          "2026.4.0-rc1-0.20261026.09174ad4a5a7",
			wantUpdateFrom:  "2026.4.0-rc0",
			wantUpgradeFrom: "2026.3.4",
		},
		{
			target:          "2026.4.0-rc11-0.20261026.09174ad4a5a7-x86_64",
			wantUpdateFrom:  "2026.4.0-rc10",
			wantUpgradeFrom: "2026.3.4",
		},
		{
			target:          "2026.3.5-0.20261020.0a1b2c3d4e5f",
			wantUpdateFrom:  "2026.3.4",
			wantUpgradeFrom: "2026.2.7-rc0",
		},
		{
			target:          "2026.4.0-dev-0.20261020.0a1b2c3d4e5f",
			wantUpdateFrom:  "",
			wantUpgradeFrom: "2026.3.4",
		},
		{
			target:          "latest",
			wantUpdateFrom:  "",
			wantUpgradeFrom: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()

			updateFrom, found := FindScyllaDBUpdateFromVersion(tags, tc.target)
			if found != (len(tc.wantUpdateFrom) > 0) || updateFrom != tc.wantUpdateFrom {
				t.Errorf("expected update from %q, got %q (found: %t)", tc.wantUpdateFrom, updateFrom, found)
			}

			upgradeFrom, found := FindScyllaDBUpgradeFromVersion(tags, tc.target)
			if found != (len(tc.wantUpgradeFrom) > 0) || upgradeFrom != tc.wantUpgradeFrom {
				t.Errorf("expected upgrade from %q, got %q (found: %t)", tc.wantUpgradeFrom, upgradeFrom, found)
			}
		})
	}
}

func TestListImageRepositoryTagsRejectsNonRepositories(t *testing.T) {
	t.Parallel()

	for _, repository := range []string{
		"",
		"docker.io/scylladb/scylla:2026.3.4",
		"docker.io/scylladb/scylla@sha256:d864230da81af7f6cdcf68446a440e0e4a55a0827fcf49af00725f660e8647d3",
		"docker.io/scylladb/scylla:2026.3.4@sha256:d864230da81af7f6cdcf68446a440e0e4a55a0827fcf49af00725f660e8647d3",
	} {
		t.Run(repository, func(t *testing.T) {
			t.Parallel()

			_, err := ListImageRepositoryTags(t.Context(), repository)
			if err == nil {
				t.Errorf("expected an error for %q", repository)
			}
		})
	}
}
