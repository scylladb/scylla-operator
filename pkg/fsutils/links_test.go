package fsutils

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestResolveSymlinks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	realDir := filepath.Join(tmpDir, "real")
	err := os.Mkdir(realDir, 0755)
	if err != nil {
		t.Error(err)
	}

	symlinkedDir := filepath.Join(tmpDir, "symlink")
	err = os.Symlink(realDir, symlinkedDir)
	if err != nil {
		t.Error(err)
	}

	nestedSymlinkedDir := filepath.Join(symlinkedDir, "symlink")
	err = os.Symlink(realDir, nestedSymlinkedDir)
	if err != nil {
		t.Error(err)
	}

	tt := []struct {
		name         string
		path         string
		expectedPath string
		expectedErr  error
	}{
		{
			name:         "existing path without symlinks",
			path:         realDir,
			expectedPath: realDir,
			expectedErr:  nil,
		},
		{
			name:         "existing path with symlinked parent",
			path:         filepath.Join(symlinkedDir, "foo"),
			expectedPath: filepath.Join(realDir, "foo"),
			expectedErr:  nil,
		},
		{
			name:         "existing symlink with symlinked parent",
			path:         nestedSymlinkedDir,
			expectedPath: realDir,
			expectedErr:  nil,
		},
		{
			name:         "non-existing path with non-existing parent and symlinked grandparent",
			path:         filepath.Join(symlinkedDir, "foo", "bar"),
			expectedPath: filepath.Join(realDir, "foo", "bar"),
			expectedErr:  nil,
		},
		{
			name:         "empty path",
			path:         "",
			expectedPath: ".",
			expectedErr:  nil,
		},
		{
			name:         "local dir path",
			path:         ".",
			expectedPath: ".",
			expectedErr:  nil,
		},
		{
			name:         "root path",
			path:         "/",
			expectedPath: "/",
			expectedErr:  nil,
		},
		{
			name:         "mangled root path",
			path:         "/////",
			expectedPath: "/",
			expectedErr:  nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveSymlinks(tc.path)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}
			if !reflect.DeepEqual(got, tc.expectedPath) {
				t.Errorf("expected and got paths differ:\n%s", cmp.Diff(tc.expectedPath, got))
			}
		})
	}

}
