package exec

import (
	"bytes"
	"context"

	"k8s.io/utils/exec"
)

func RunCommand(ctx context.Context, executor exec.Interface, command string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	cmd := executor.CommandContext(ctx, command, args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetStdout(stdout)
	cmd.SetStderr(stderr)

	err := cmd.Run()
	if err != nil {
		return stdout, stderr, err
	}
	return stdout, stderr, nil
}
