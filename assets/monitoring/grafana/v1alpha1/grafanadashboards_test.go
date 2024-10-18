package v1alpha1

import (
	"embed"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewGrafanaDashboardsFromFS(t *testing.T) {
	tt := []struct {
		name         string
		filesystem   embed.FS
		root         string
		expectedErr  error
		validateFunc func(*testing.T, GrafanaDashboardsFoldersMap)
	}{
		{
			name:       "can parse platform dashboards",
			filesystem: grafanaDashboardsPlatformFS,
			root:       "dashboards/platform",
			validateFunc: func(t *testing.T, dfs GrafanaDashboardsFoldersMap) {
				t.Helper()

				if len(dfs) == 0 {
					t.Errorf("no platform dashboards found")
				}
			},
			expectedErr: nil,
		},
		{
			name:       "can parse saas dashboards",
			filesystem: grafanaDashboardsSAASFS,
			root:       "dashboards/saas",
			validateFunc: func(t *testing.T, dfs GrafanaDashboardsFoldersMap) {
				t.Helper()

				if len(dfs) == 0 {
					t.Errorf("no saas dashboards found")
				}
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dashboardsFoldersMap, err := NewGrafanaDashboardsFromFS(tc.filesystem, tc.root)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err))
			}

			tc.validateFunc(t, dashboardsFoldersMap)
		})
	}
}
