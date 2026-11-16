// Copyright (c) 2026 ScyllaDB.

package images

import (
	"testing"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestPlatformsMatch(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		a        imgspecv1.Platform
		b        imgspecv1.Platform
		expected bool
	}{
		{
			name:     "same platform",
			a:        imgspecv1.Platform{OS: "linux", Architecture: "amd64"},
			b:        imgspecv1.Platform{OS: "linux", Architecture: "amd64"},
			expected: true,
		},
		{
			name:     "different architecture",
			a:        imgspecv1.Platform{OS: "linux", Architecture: "amd64"},
			b:        imgspecv1.Platform{OS: "linux", Architecture: "arm64"},
			expected: false,
		},
		{
			name:     "different OS",
			a:        imgspecv1.Platform{OS: "linux", Architecture: "amd64"},
			b:        imgspecv1.Platform{OS: "windows", Architecture: "amd64"},
			expected: false,
		},
		{
			name:     "arm64 v8 variant matches an empty variant",
			a:        imgspecv1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			b:        imgspecv1.Platform{OS: "linux", Architecture: "arm64"},
			expected: true,
		},
		{
			name:     "different arm variants",
			a:        imgspecv1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			b:        imgspecv1.Platform{OS: "linux", Architecture: "arm", Variant: "v6"},
			expected: false,
		},
		{
			name:     "arm v7 variant doesn't match an empty variant",
			a:        imgspecv1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			b:        imgspecv1.Platform{OS: "linux", Architecture: "arm"},
			expected: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := platformsMatch(tc.a, tc.b)
			if got != tc.expected {
				t.Errorf("expected %t, got %t", tc.expected, got)
			}
		})
	}
}
