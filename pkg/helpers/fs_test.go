package helpers

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTouchFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing")
	nonexistingFile := filepath.Join(tmpDir, "nonexisting")
	nonexistingFileWithoutParent := filepath.Join(tmpDir+"-foo", "nonexisting")

	setupErr := os.WriteFile(existingFile, nil, 0600)
	if setupErr != nil {
		t.Fatal(setupErr)
	}

	tt := []struct {
		name        string
		path        string
		expectedErr error
	}{
		{
			name:        "new file succeeds",
			path:        nonexistingFile,
			expectedErr: nil,
		},
		{
			name:        "existing file succeeds",
			path:        existingFile,
			expectedErr: nil,
		},
		{
			name: "file without a parent fails",
			path: nonexistingFileWithoutParent,
			expectedErr: fmt.Errorf(`can't create file %q: %w`, nonexistingFileWithoutParent, &fs.PathError{
				Op:   "open",
				Path: nonexistingFileWithoutParent,
				Err:  syscall.Errno(0x02),
			}),
		},
	}

	now := time.Now().Local()
	// Make sure the times are distinguishable.
	time.Sleep(100 * time.Millisecond)

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var previousModTime time.Time
			var initialFI os.FileInfo
			var err error
			initialFI, err = os.Stat(tc.path)
			if err == nil {
				previousModTime = initialFI.ModTime()
			} else if os.IsNotExist(err) {
				previousModTime = now
			} else {
				t.Fatal(err)
			}

			err = TouchFile(tc.path)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected error %#+v, got %#+v, diff :%s", tc.expectedErr, err, cmp.Diff(tc.expectedErr, err))
			}

			if tc.expectedErr != nil {
				return
			}

			var finalFI os.FileInfo
			finalFI, err = os.Stat(tc.path)
			if err != nil {
				t.Fatal(err)
			}

			lastModTime := finalFI.ModTime()
			if !lastModTime.After(previousModTime) {
				t.Errorf("file mod time %v is not after the initial file mod time %v", lastModTime, previousModTime)
			}
		})
	}
}
