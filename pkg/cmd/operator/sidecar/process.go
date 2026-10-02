// Copyright (c) 2026 ScyllaDB.

package sidecar

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// scyllaProcess is the ScyllaDB entrypoint running as a child of the sidecar.
// With supervisord-based images the child is the entrypoint waiting on supervisord, with newer images
// the entrypoint execs into ScyllaDB, so the child is ScyllaDB itself.
type scyllaProcess struct {
	cmd     *exec.Cmd
	exited  chan struct{}
	waitErr error
}

func startScyllaProcess(cmd *exec.Cmd) (*scyllaProcess, error) {
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	p := &scyllaProcess{
		cmd:    cmd,
		exited: make(chan struct{}),
	}

	go func() {
		defer close(p.exited)
		p.waitErr = cmd.Wait()
	}()

	return p, nil
}

// Exited is closed once the process has exited and has been reaped.
func (p *scyllaProcess) Exited() <-chan struct{} {
	return p.exited
}

// Terminate asks the process to shut down. It's a no-op if the process has already exited.
func (p *scyllaProcess) Terminate() error {
	err := p.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("can't send SIGTERM to the scylla process: %w", err)
	}

	return nil
}

// ExitCode returns the exit code of the process, following the shell convention of 128+N for processes killed by
// signal N. It must only be called after Exited is closed.
func (p *scyllaProcess) ExitCode() int {
	if p.waitErr == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if !errors.As(p.waitErr, &exitErr) {
		return 1
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if ok && status.Signaled() {
		return 128 + int(status.Signal())
	}

	return exitErr.ExitCode()
}

// WaitErr returns the error of waiting for the process. It must only be called after Exited is closed.
func (p *scyllaProcess) WaitErr() error {
	return p.waitErr
}
