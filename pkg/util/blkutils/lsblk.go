package blkutils

import (
	"context"
	"fmt"

	oexec "github.com/scylladb/scylla-operator/pkg/util/exec"
	apimachineryutiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/exec"
)

type lsblkOutput struct {
	BlockDevices []*BlockDevice `json:"blockdevices"`
}

type BlockDevice struct {
	Name     string `json:"name"`
	Model    string `json:"model"`
	FSType   string `json:"fstype"`
	PartUUID string `json:"partuuid"`
}

var (
	lsblkListBlockDevicesArgs = []string{
		"--json",
		"--nodeps",
		"--paths",
		"--fs",
		"--output=NAME,MODEL,FSTYPE,PARTUUID",
	}
)

func IsBlockDevice(ctx context.Context, executor exec.Interface, device string) (bool, error) {
	const (
		notABlockDeviceExitCode = 32
	)
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "lsblk", device)
	if err != nil {
		exitErr, ok := err.(exec.ExitError)
		if ok && exitErr.ExitStatus() == notABlockDeviceExitCode {
			return false, nil
		}

		return false, fmt.Errorf("failed to run lsblk with args: %v: %w, stdout: %q, stderr: %q", device, err, stdout.String(), stderr.String())
	}

	return true, nil
}

func GetBlockDevice(ctx context.Context, executor exec.Interface, device string) (*BlockDevice, error) {
	blockDevices, err := ListBlockDevices(ctx, executor, device)
	if err != nil {
		return nil, fmt.Errorf("can't list block device %q: %w", device, err)
	}

	if len(blockDevices) < 1 {
		return nil, fmt.Errorf("device %q not found", device)
	}

	return blockDevices[0], nil
}

func ListBlockDevices(ctx context.Context, executor exec.Interface, devices ...string) ([]*BlockDevice, error) {
	args := append(lsblkListBlockDevicesArgs, devices...)

	stdout, stderr, err := oexec.RunCommand(ctx, executor, "lsblk", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to run lsblk with args: %v: %w, stdout: %q, stderr: %q", args, err, stdout, stderr.String())
	}

	output := &lsblkOutput{}
	err = apimachineryutiljson.Unmarshal(stdout.Bytes(), output)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal lsblk output %q: %w", stdout.String(), err)
	}

	return output.BlockDevices, nil
}

func GetPartitionUUID(ctx context.Context, executor exec.Interface, device string) (string, error) {
	bd, err := GetBlockDevice(ctx, executor, device)
	if err != nil {
		return "", fmt.Errorf("can't get block device %q: %w", device, err)
	}

	return bd.PartUUID, nil
}

func GetFilesystemType(ctx context.Context, executor exec.Interface, device string) (string, error) {
	bd, err := GetBlockDevice(ctx, executor, device)
	if err != nil {
		return "", fmt.Errorf("can't get block device %q: %w", device, err)
	}

	return bd.FSType, nil
}
