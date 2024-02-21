// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/exectest"
)

func TestApplyLoopDevice(t *testing.T) {
	t.Parallel()

	type deviceSymlink struct {
		symlinkPath string
		devicePath  string
	}

	tt := []struct {
		name                       string
		deviceName                 string
		imagePath                  func(tmpDir string) string
		sizeBytes                  int64
		populateFilesystem         func(tmpDir, devfsPath string)
		expectedCommands           func(tmpDir, devfsPath string) []exectest.Command
		expectedDevLoopsDirToExist bool
		expectedDeviceSymlinks     func(devfsPath string) []deviceSymlink
		expectedChanged            bool
		expectedErr                func(tmpDir, devfsPath string) error
	}{
		{
			name:       "creates image, loop device and symlink when no loop devices exists",
			deviceName: "foo",
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image")
			},
			sizeBytes:          1234,
			populateFilesystem: nil,
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd: "losetup",
						Args: []string{
							"--all",
							"--list",
							"--json",
						},
						Stdout: []byte(`{"loopdevices":[]}`),
						Stderr: nil,
						Err:    nil,
					},
					{
						Cmd: "losetup",
						Args: []string{
							"--show",
							"--find",
							path.Join(tmpDir, "image"),
						},
						Stdout: []byte(path.Join(devfsPath, "loop0")),
						Stderr: nil,
						Err:    nil,
					},
				}
			},
			expectedDevLoopsDirToExist: true,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return []deviceSymlink{
					{
						devicePath:  path.Join(devfsPath, "loop0"),
						symlinkPath: path.Join(devfsPath, "loops", "foo"),
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name:       "image is not created if it already exists and has correct size",
			deviceName: "foo",
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image")
			},
			sizeBytes: 1234,
			populateFilesystem: func(tmpDir, devfsPath string) {
				imagePath := path.Join(tmpDir, "image")
				f, err := os.Create(imagePath)
				if err != nil {
					t.Fatalf("can't create image file at %q: %v", imagePath, err)
				}

				err = f.Truncate(1234)
				if err != nil {
					t.Fatalf("can't truncate image file at %q: %v", imagePath, err)
				}

				err = f.Close()
				if err != nil {
					t.Fatalf("can't close image file at %q: %v", imagePath, err)
				}
			},
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd: "losetup",
						Args: []string{
							"--all",
							"--list",
							"--json",
						},
						Stdout: []byte(`{"loopdevices":[]}`),
						Stderr: nil,
						Err:    nil,
					},
					{
						Cmd: "losetup",
						Args: []string{
							"--show",
							"--find",
							path.Join(tmpDir, "image"),
						},
						Stdout: []byte(path.Join(devfsPath, "loop0")),
						Stderr: nil,
						Err:    nil,
					},
				}
			},
			expectedDevLoopsDirToExist: true,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return []deviceSymlink{
					{
						devicePath:  path.Join(devfsPath, "loop0"),
						symlinkPath: path.Join(devfsPath, "loops", "foo"),
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
		{
			name:       "error is returned when image file already exists and have different size than requested one",
			deviceName: "foo",
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image")
			},
			sizeBytes: 1234,
			populateFilesystem: func(tmpDir, devfsPath string) {
				imagePath := path.Join(tmpDir, "image")
				f, err := os.Create(imagePath)
				if err != nil {
					t.Fatalf("can't create image file at %q: %v", imagePath, err)
				}

				err = f.Truncate(4321)
				if err != nil {
					t.Fatalf("can't truncate image file at %q: %v", imagePath, err)
				}

				err = f.Close()
				if err != nil {
					t.Fatalf("can't close image file at %q: %v", imagePath, err)
				}
			},
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{}
			},
			expectedDevLoopsDirToExist: false,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return nil
			},
			expectedChanged: false,
			expectedErr: func(tmpDir, devfsPath string) error {
				return fmt.Errorf("can't make loop device image file: %w", fmt.Errorf("device image resize is not supported, existing image file %q has size 4321, different than expected 1234", path.Join(tmpDir, "image")))
			},
		},
		{
			name:       "creates symlink when image and loop device already exists but symlink is missing",
			deviceName: "foo",
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image")
			},
			sizeBytes: 1234,
			populateFilesystem: func(tmpDir, devfsPath string) {
				imagePath := path.Join(tmpDir, "image")
				f, err := os.Create(imagePath)
				if err != nil {
					t.Fatalf("can't create image file at %q: %v", imagePath, err)
				}

				err = f.Truncate(1234)
				if err != nil {
					t.Fatalf("can't truncate image file at %q: %v", imagePath, err)
				}

				err = f.Close()
				if err != nil {
					t.Fatalf("can't close image file at %q: %v", imagePath, err)
				}
			},
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd: "losetup",
						Args: []string{
							"--all",
							"--list",
							"--json",
						},
						Stdout: []byte(fmt.Sprintf(`{"loopdevices":[{"name":"%s","back-file":"%s"}]}`, path.Join(devfsPath, "loop0"), path.Join(tmpDir, "image"))),
						Stderr: nil,
						Err:    nil,
					},
				}
			},
			expectedDevLoopsDirToExist: true,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return []deviceSymlink{
					{
						devicePath:  path.Join(devfsPath, "loop0"),
						symlinkPath: path.Join(devfsPath, "loops", "foo"),
					},
				}
			},
			expectedChanged: true,
			expectedErr:     nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir := t.TempDir()
			devfsDir := t.TempDir()

			if tc.populateFilesystem != nil {
				tc.populateFilesystem(tmpDir, devfsDir)
			}

			expectedCommands := tc.expectedCommands(tmpDir, devfsDir)
			executor := exectest.NewFakeExec(expectedCommands...)
			imagePath := tc.imagePath(tmpDir)
			symlinkPath := path.Join(devfsDir, "loops", tc.deviceName)
			var expectedErr error
			if tc.expectedErr != nil {
				expectedErr = tc.expectedErr(tmpDir, devfsDir)
			}

			changed, err := ApplyLoopDevice(ctx, executor, imagePath, symlinkPath, tc.sizeBytes)
			if !reflect.DeepEqual(err, expectedErr) {
				t.Fatalf("expected %v error, got %v", expectedErr, err)
			}
			if !reflect.DeepEqual(changed, tc.expectedChanged) {
				t.Fatalf("expected changed %v, got %v", tc.expectedChanged, changed)
			}
			if executor.CommandCalls != len(expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(expectedCommands), executor.CommandCalls)
			}

			symlinksDir := path.Join(devfsDir, "loops")
			symlinksDirStat, err := os.Stat(symlinksDir)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("unexpected error on symlinksDir %q stat: %v", symlinksDir, err)
			}

			symlinksDirExists := symlinksDirStat != nil
			if !reflect.DeepEqual(tc.expectedDevLoopsDirToExist, symlinksDirExists) {
				t.Fatalf("expected symlinksDir to exists %v, got %v", tc.expectedDevLoopsDirToExist, symlinksDirExists)
			}

			if symlinksDirExists {
				var existingLoopDevicesSymlinks []deviceSymlink
				err = filepath.WalkDir(symlinksDir, func(fpath string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}

					if fpath == symlinksDir {
						return nil
					}

					if d.Type()&fs.ModeSymlink == 0 {
						return fmt.Errorf("expected to find only symlinks in symlinks directory, got %v filemode", d.Type())
					}

					device, err := os.Readlink(fpath)
					if err != nil {
						return fmt.Errorf("can't read symlink %q: %w", fpath, err)
					}

					existingLoopDevicesSymlinks = append(existingLoopDevicesSymlinks, deviceSymlink{
						symlinkPath: fpath,
						devicePath:  device,
					})

					return nil
				})
				if err != nil {
					t.Fatal(err)
				}

				expectedDeviceSymlinks := tc.expectedDeviceSymlinks(devfsDir)

				if !reflect.DeepEqual(expectedDeviceSymlinks, existingLoopDevicesSymlinks) {
					t.Fatalf("expected device symlinks %v, got %v", expectedDeviceSymlinks, existingLoopDevicesSymlinks)
				}
			}
		})
	}
}

func TestDeleteLoopDevice(t *testing.T) {
	t.Parallel()

	type deviceSymlink struct {
		symlinkPath string
		devicePath  string
	}

	tt := []struct {
		name                       string
		deviceSymlinkPath          func(devfsPath string) string
		devicePath                 func(devfsPath string) string
		imagePath                  func(tmpDir string) string
		populateFilesystem         func(tmpDir, devfsPath string)
		expectedCommands           func(tmpDir, devfsPath string) []exectest.Command
		expectedDevLoopsDirToExist bool
		expectedDeviceSymlinks     func(devfsPath string) []deviceSymlink
		expectedErr                func(tmpDir, devfsPath string) error
	}{
		{
			name: "detaches loop device, removes image and symlink together with symlinks directory when it's the last device",
			deviceSymlinkPath: func(devfsPath string) string {
				return path.Join(devfsPath, "loops", "foo")
			},
			devicePath: func(devfsPath string) string {
				return path.Join(devfsPath, "loop0")
			},
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image")
			},
			populateFilesystem: func(tmpDir, devfsPath string) {
				imagePath := path.Join(tmpDir, "image")
				f, err := os.Create(imagePath)
				if err != nil {
					t.Fatalf("can't create image file at %q: %v", imagePath, err)
				}
				err = f.Close()
				if err != nil {
					t.Fatalf("can't close image file at %q: %v", imagePath, err)
				}

				devicePath := path.Join(devfsPath, "loop0")
				f, err = os.Create(devicePath)
				if err != nil {
					t.Fatalf("can't create device file at %q: %v", devicePath, err)
				}

				err = f.Close()
				if err != nil {
					t.Fatalf("can't close device file at %q: %v", devicePath, err)
				}

				symlinksDir := path.Join(devfsPath, "loops")
				err = os.Mkdir(symlinksDir, 0770)
				if err != nil {
					t.Fatalf("can't create symlinks dir at %q: %v", symlinksDir, err)
				}

				symlinkPath := path.Join(symlinksDir, "foo")

				err = os.Symlink(devicePath, symlinkPath)
				if err != nil {
					t.Fatalf("can't create symlink file at %q: %v", symlinkPath, err)
				}
			},
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd: "losetup",
						Args: []string{
							"--all",
							"--list",
							"--json",
						},
						Stdout: []byte(fmt.Sprintf(`{"loopdevices":[{"name": "%s","back-file": "%s"}]}`, path.Join(devfsPath, "loop0"), path.Join(tmpDir, "image"))),
						Stderr: nil,
						Err:    nil,
					},
					{
						Cmd: "losetup",
						Args: []string{
							"--detach",
							path.Join(devfsPath, "loop0"),
						},
						Stdout: nil,
						Stderr: nil,
						Err:    nil,
					},
				}
			},
			expectedDevLoopsDirToExist: false,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return nil
			},
			expectedErr: nil,
		},
		{
			name: "detaches loop device, removes image and symlink together but keeps symlinks directory with other devices when they still exist",
			deviceSymlinkPath: func(devfsPath string) string {
				return path.Join(devfsPath, "loops", "loop0")
			},
			devicePath: func(devfsPath string) string {
				return path.Join(devfsPath, "loop0")
			},
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image0")
			},
			populateFilesystem: func(tmpDir, devfsPath string) {
				for _, imageName := range []string{"image0", "image1"} {
					imagePath := path.Join(tmpDir, imageName)
					f, err := os.Create(imagePath)
					if err != nil {
						t.Fatalf("can't create image file at %q: %v", imagePath, err)
					}
					err = f.Close()
					if err != nil {
						t.Fatalf("can't close image file at %q: %v", imagePath, err)
					}
				}

				for _, deviceName := range []string{"loop0", "loop1"} {
					devicePath := path.Join(devfsPath, deviceName)
					f, err := os.Create(devicePath)
					if err != nil {
						t.Fatalf("can't create device file at %q: %v", devicePath, err)
					}

					err = f.Close()
					if err != nil {
						t.Fatalf("can't close device file at %q: %v", devicePath, err)
					}
				}

				symlinksDir := path.Join(devfsPath, "loops")
				err := os.Mkdir(symlinksDir, 0770)
				if err != nil {
					t.Fatalf("can't create symlinks dir at %q: %v", symlinksDir, err)
				}

				for _, deviceName := range []string{"loop0", "loop1"} {
					devicePath := path.Join(devfsPath, deviceName)
					symlinkPath := path.Join(symlinksDir, deviceName)
					err = os.Symlink(devicePath, symlinkPath)
					if err != nil {
						t.Fatalf("can't create symlink file at %q: %v", symlinkPath, err)
					}
				}
			},
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd: "losetup",
						Args: []string{
							"--all",
							"--list",
							"--json",
						},
						Stdout: []byte(fmt.Sprintf(`{"loopdevices":[{"name": "%s","back-file": "%s"}]}`, path.Join(devfsPath, "loop0"), path.Join(tmpDir, "image"))),
						Stderr: nil,
						Err:    nil,
					},
					{
						Cmd: "losetup",
						Args: []string{
							"--detach",
							path.Join(devfsPath, "loop0"),
						},
						Stdout: nil,
						Stderr: nil,
						Err:    nil,
					},
				}
			},
			expectedDevLoopsDirToExist: true,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return []deviceSymlink{
					{
						symlinkPath: path.Join(devfsPath, "loops", "loop1"),
						devicePath:  path.Join(devfsPath, "loop1"),
					},
				}
			},
			expectedErr: nil,
		},
		{
			name: "no error when device, image and symlink doesn't exist",
			deviceSymlinkPath: func(devfsPath string) string {
				return path.Join(devfsPath, "loops", "loop0")
			},
			devicePath: func(devfsPath string) string {
				return path.Join(devfsPath, "loop0")
			},
			imagePath: func(tmpDir string) string {
				return path.Join(tmpDir, "image0")
			},
			expectedCommands: func(tmpDir, devfsPath string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd: "losetup",
						Args: []string{
							"--all",
							"--list",
							"--json",
						},
						Stdout: []byte(`{"loopdevices":[]}`),
						Stderr: nil,
						Err:    nil,
					},
				}
			},
			expectedDevLoopsDirToExist: false,
			expectedDeviceSymlinks: func(devfsPath string) []deviceSymlink {
				return []deviceSymlink{}
			},
			expectedErr: nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tmpDir, err := os.MkdirTemp(os.TempDir(), "delete-loop-device-tmp-")
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.RemoveAll(tmpDir)
				if err != nil {
					t.Fatal(err)
				}
			}()

			devfsDir, err := os.MkdirTemp(os.TempDir(), "delete-loop-device-devfs-")
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.RemoveAll(devfsDir)
				if err != nil {
					t.Fatal(err)
				}
			}()

			if tc.populateFilesystem != nil {
				tc.populateFilesystem(tmpDir, devfsDir)
			}

			expectedCommands := tc.expectedCommands(tmpDir, devfsDir)

			executor := exectest.NewFakeExec(expectedCommands...)
			deviceSymlinkPath := tc.deviceSymlinkPath(devfsDir)
			imagePath := tc.imagePath(tmpDir)
			devicePath := tc.devicePath(devfsDir)

			var expectedErr error
			if tc.expectedErr != nil {
				expectedErr = tc.expectedErr(tmpDir, devfsDir)
			}

			err = DeleteLoopDevice(ctx, executor, deviceSymlinkPath, devicePath, imagePath)
			if !reflect.DeepEqual(err, expectedErr) {
				t.Fatalf("expected %v error, got %v", expectedErr, err)
			}
			if executor.CommandCalls != len(expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(expectedCommands), executor.CommandCalls)
			}

			symlinksDir := path.Join(devfsDir, "loops")
			symlinksDirStat, err := os.Stat(symlinksDir)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("unexpected error on symlinksDir %q stat: %v", symlinksDir, err)
			}

			symlinksDirExists := symlinksDirStat != nil
			if !reflect.DeepEqual(tc.expectedDevLoopsDirToExist, symlinksDirExists) {
				t.Fatalf("expected symlinksDir %q to exists %v, got %v", symlinksDir, tc.expectedDevLoopsDirToExist, symlinksDirExists)
			}

			if symlinksDirExists {
				var existingLoopDevicesSymlinks []deviceSymlink
				err = filepath.WalkDir(symlinksDir, func(fpath string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}

					if fpath == symlinksDir {
						return nil
					}

					if d.Type()&fs.ModeSymlink == 0 {
						return fmt.Errorf("expected to find only symlinks in symlinks directory, got %v filemode", d.Type())
					}

					device, err := os.Readlink(fpath)
					if err != nil {
						return fmt.Errorf("can't read symlink %q: %w", fpath, err)
					}

					existingLoopDevicesSymlinks = append(existingLoopDevicesSymlinks, deviceSymlink{
						symlinkPath: fpath,
						devicePath:  device,
					})

					return nil
				})
				if err != nil {
					t.Fatal(err)
				}

				expectedDeviceSymlinks := tc.expectedDeviceSymlinks(devfsDir)

				if !reflect.DeepEqual(expectedDeviceSymlinks, existingLoopDevicesSymlinks) {
					t.Fatalf("expected device symlinks %v, got %v", expectedDeviceSymlinks, existingLoopDevicesSymlinks)
				}
			}
		})
	}
}
