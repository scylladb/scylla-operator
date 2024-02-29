// Copyright (c) 2023 ScyllaDB.

package losetup

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	oexec "github.com/scylladb/scylla-operator/pkg/util/exec"
	"k8s.io/utils/exec"
)

type losetupOutput struct {
	LoopDevices []*LoopDevice `json:"loopdevices"`
}

type LoopDevice struct {
	Name        string `json:"name"`
	BackingFile string `json:"back-file"`
}

func ListLoopDevices(ctx context.Context, executor exec.Interface) ([]*LoopDevice, error) {
	args := []string{
		"--all",
		"--list",
		"--json",
	}
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "losetup", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to run losetup with args: %v: %w, stdout: %q, stderr: %q", args, err, stdout, stderr.String())
	}

	// losetup may return nothing, when there are no loop devices.
	if stdout.Len() == 0 {
		return []*LoopDevice{}, nil
	}

	output := &losetupOutput{}
	err = json.Unmarshal(stdout.Bytes(), output)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal losetup output %q: %w", stdout.String(), err)
	}

	return output.LoopDevices, nil
}

func AttachLoopDevice(ctx context.Context, executor exec.Interface, imagePath string) (string, error) {
	args := []string{
		"--show",
		"--find",
		imagePath,
	}
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "losetup", args...)
	if err != nil {
		return "", fmt.Errorf("can't run lopsetup with args %v: %w, stdout: %q, stderr: %q", args, err, stdout.String(), stderr.String())
	}

	devicePath := strings.TrimSpace(stdout.String())

	return devicePath, nil
}

func DetachLoopDevice(ctx context.Context, executor exec.Interface, devicePath string) error {
	args := []string{
		"--detach", devicePath,
	}
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "losetup", args...)
	if err != nil {
		return fmt.Errorf("can't run lopsetup with args %v: %w, stdout: %q, stderr: %q", args, err, stdout.String(), stderr.String())
	}

	return nil
}
