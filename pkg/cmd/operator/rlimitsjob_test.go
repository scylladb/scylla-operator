// Copyright (c) 2024 ScyllaDB.

package operator

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"testing"
)

func TestGetSysctlFsNrOpen(t *testing.T) {
	t.Parallel()

	makeFSNROpenFunc := func(fileContent string) func(procFs string) {
		return func(procFs string) {
			procSysFsPath := filepath.Join(procFs, "/sys/fs/")
			err := os.MkdirAll(procSysFsPath, 0777)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(filepath.Join(procSysFsPath, "/nr_open"), []byte(fileContent), 0777)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	tt := []struct {
		name            string
		makeFS          func(procFs string)
		expectedValue   uint64
		makeExpectedErr func(procFs string) error
	}{
		{
			name:          "spaces around value",
			makeFS:        makeFSNROpenFunc(" 123   "),
			expectedValue: 123,
			makeExpectedErr: func(procFs string) error {
				return nil
			},
		},
		{
			name:          "new line after value",
			makeFS:        makeFSNROpenFunc("321\n"),
			expectedValue: 321,
			makeExpectedErr: func(procFs string) error {
				return nil
			},
		},
		{
			name:          "empty file",
			makeFS:        makeFSNROpenFunc(""),
			expectedValue: 0,
			makeExpectedErr: func(procFs string) error {
				return fmt.Errorf(`can't parse "" to uint64: %w`, &strconv.NumError{Func: "ParseUint", Num: "", Err: strconv.ErrSyntax})
			},
		},
		{
			name: "missing file",
			makeFS: func(tempDir string) {

			},
			expectedValue: 0,
			makeExpectedErr: func(procFs string) error {
				fsNROpenFilePath := filepath.Join(procFs, "/sys/fs/nr_open")
				return fmt.Errorf("can't read %q: %w", fsNROpenFilePath, &os.PathError{Op: "open", Path: fsNROpenFilePath, Err: syscall.Errno(0x02)})
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			procFS := t.TempDir()
			tc.makeFS(procFS)
			expectedErr := tc.makeExpectedErr(procFS)
			v, err := getSysctlFSNROpen(procFS)
			if !reflect.DeepEqual(expectedErr, err) {
				t.Errorf("expected %v error, got %v", expectedErr, err)
			}
			if v != tc.expectedValue {
				t.Errorf("expected %d, got %d", tc.expectedValue, v)
			}
		})
	}
}
