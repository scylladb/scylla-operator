// Copyright (C) 2021 ScyllaDB

package operator

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	linuxproc "github.com/c9s/goprocinfo/linux"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
)

func perftuneCommand(ctx context.Context, cpuInfo *linuxproc.CPUInfo, procStatus *linuxproc.ProcessStatus) (*exec.Cmd, error) {
	fullCpuset, err := cpuset.Parse(fmt.Sprintf("0-%d", cpuInfo.NumCPU()-1))
	if err != nil {
		return nil, fmt.Errorf("parse full mask")
	}

	cpusAllowedMask := cpuset.ParseMaskFormat(procStatus.CpusAllowed)

	// TODO(maciej): consider banning IRQ hyperthreaded siblings in Scylla cpuset
	// Use all CPUs *not* assigned to Scylla container for IRQs.
	diffCpuset := fullCpuset.Difference(cpusAllowedMask)

	args := []string{
		"--tune", "net", "--irq-cpu-mask", diffCpuset.FormatMask(),
		"--tune", "disks", "--dir", naming.DataDir,
	}

	cmd := exec.CommandContext(ctx, "/opt/scylladb/scripts/perftune.py", args...)
	cmd.Env = append(cmd.Env, "SYSTEMD_IGNORE_CHROOT=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func runPerftune(ctx context.Context, logger log.Logger) error {
	cpuInfo, err := linuxproc.ReadCPUInfo("/proc/cpuinfo")
	if err != nil {
		return fmt.Errorf("parse cpuinfo: %w", err)
	}

	// Currently Scylla is a child process of sidecar, so look at 'self'.
	procStatus, err := linuxproc.ReadProcessStatus("/proc/self/status")
	if err != nil {
		return fmt.Errorf("parse proc self status: %w", err)
	}

	cmd, err := perftuneCommand(ctx, cpuInfo, procStatus)
	if err != nil {
		return fmt.Errorf("prepare perftune command: %w", err)
	}

	logger.Info(ctx, "Running perftune", "args", cmd.Args)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run perftune: %w", err)
	}

	return nil
}

func setWriteCache(ctx context.Context, logger log.Logger, dir, cache string) error {
	devPath, err := getMountDevicePath(dir)
	if err != nil {
		return err
	}

	physicalDevices, err := getPhysicalDevices(devPath)
	if err != nil {
		return err
	}

	logger.Debug(ctx, "Setting cache on physical devices", "devices", physicalDevices, "cache", cache)

	for _, dev := range physicalDevices {
		devFd, err := os.OpenFile(path.Join(dev, "queue", "write_cache"), os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(devFd, cache)
		if err != nil {
			return err
		}
	}

	return nil
}

func getMountDevicePath(dir string) (string, error) {
	s, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	stat := s.Sys().(*syscall.Stat_t)
	major, minor := stat.Dev>>8&0xff, stat.Dev&0xff

	blockPath := fmt.Sprintf("/sys/dev/block/%d:%d", major, minor)
	devPath, err := resolveSymlink(blockPath)
	if err != nil {
		return "", err
	}
	return devPath, nil

}

func getPhysicalDevices(devPath string) ([]string, error) {
	if !strings.Contains(devPath, "virtual") {
		return []string{devPath}, nil
	}

	var slaves []string
	slavesPath := path.Join(devPath, "slaves")
	err := filepath.WalkDir(slavesPath, func(p string, d fs.DirEntry, err error) error {
		if p == slavesPath {
			return nil
		}

		finfo, err := d.Info()
		if err != nil {
			return err
		}

		if finfo.Mode()&os.ModeSymlink != 0 {
			slaveDevPath, err := resolveSymlink(p)
			if err != nil {
				return err
			}

			slaves = append(slaves, slaveDevPath)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return slaves, nil
}

func resolveSymlink(linkPath string) (string, error) {
	resolvedPath, err := os.Readlink(linkPath)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath, err = filepath.Abs(path.Join(path.Dir(linkPath), resolvedPath))
		if err != nil {
			return "", err
		}
	}

	return resolvedPath, nil
}
