// Copyright (c) 2026 ScyllaDB.

package reconcilecritic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace/critic"
)

func TestClaudeCLIModel(t *testing.T) {
	t.Parallel()

	// A stand-in CLI echoing its arguments and standard input.
	dir := t.TempDir()
	binary := filepath.Join(dir, "claude")
	err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'cwd=%s\\n' \"$PWD\"\nprintf 'arg=%s\\n' \"$@\"\nprintf 'stdin='\ncat\n"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	m := ClaudeCLIModel{Binary: binary, Model: "some-model", Dir: dir}
	out, err := m.Complete(context.Background(), critic.Prompt{System: "be terse", User: "the timeline"})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"cwd=" + dir,
		"arg=--print",
		"arg=--append-system-prompt\narg=be terse",
		"arg=--tools\narg=Read,Grep,Glob",
		"arg=--permission-mode\narg=dontAsk",
		"arg=--strict-mcp-config",
		"arg=--model\narg=some-model",
		"stdin=the timeline",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	}
}

func TestClaudeCLIModelReportsFailures(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "claude")
	err := os.WriteFile(binary, []byte("#!/bin/sh\necho 'not logged in' >&2\nexit 1\n"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ClaudeCLIModel{Binary: binary}.Complete(context.Background(), critic.Prompt{})
	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("expected the CLI's error output in the error, got %v", err)
	}
}
