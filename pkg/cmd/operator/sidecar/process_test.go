// Copyright (c) 2026 ScyllaDB.

package sidecar

import (
	"bufio"
	"os/exec"
	"testing"
	"time"
)

func TestScyllaProcess(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name             string
		script           string
		terminate        bool
		expectedExitCode int
	}{
		{
			name:             "clean exit",
			script:           "exit 0",
			expectedExitCode: 0,
		},
		{
			name:             "non-zero exit",
			script:           "exit 3",
			expectedExitCode: 3,
		},
		{
			name:             "killed by a signal",
			script:           "kill -KILL $$",
			expectedExitCode: 137,
		},
		{
			name:             "terminated",
			script:           "echo ready; exec sleep 60",
			terminate:        true,
			expectedExitCode: 143,
		},
		{
			name:             "terminated with a handler",
			script:           "trap 'exit 0' TERM; echo ready; sleep 60 & wait",
			terminate:        true,
			expectedExitCode: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command("/bin/sh", "-c", tc.script)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("can't get stdout pipe: %v", err)
			}

			p, err := startScyllaProcess(cmd)
			if err != nil {
				t.Fatalf("can't start process: %v", err)
			}

			if tc.terminate {
				// Wait for the script to set itself up, so the signal isn't delivered before its trap is installed.
				_, err = bufio.NewReader(stdout).ReadString('\n')
				if err != nil {
					t.Fatalf("can't read readiness line: %v", err)
				}

				err = p.Terminate()
				if err != nil {
					t.Fatalf("can't terminate process: %v", err)
				}
			}

			select {
			case <-p.Exited():
			case <-time.After(30 * time.Second):
				t.Fatalf("process hasn't exited")
			}

			if got := p.ExitCode(); got != tc.expectedExitCode {
				t.Errorf("expected exit code %d, got %d (wait error: %v)", tc.expectedExitCode, got, p.WaitErr())
			}

			err = p.Terminate()
			if err != nil {
				t.Errorf("terminating an exited process should be a no-op, got: %v", err)
			}
		})
	}
}
