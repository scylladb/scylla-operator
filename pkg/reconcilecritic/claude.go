// Copyright (c) 2026 ScyllaDB.

// Package reconcilecritic plugs the operator into the reconcile critic of controller-runtime's reconciletrace: a
// model that answers through the claude CLI, and the domain context of the operator's controllers. It is an
// experiment for reviewing traces recorded in tests; nothing in the operator binary uses it.
package reconcilecritic

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace/critic"
)

// ClaudeCLIModel answers prompts by running the claude CLI in print mode. The CLI runs with read-only tools in Dir, so
// that the model can read the controller's source to name the code behind a finding, and with no MCP servers and no
// persisted session.
type ClaudeCLIModel struct {
	// Binary is the claude CLI. Defaults to "claude" on PATH.
	Binary string
	// Model is passed as --model when set.
	Model string
	// Dir is the working directory of the CLI, usually the repository root. The tools can read only below it.
	Dir string
	// Tools are the tools the model may use. Defaults to Read, Grep and Glob.
	Tools []string
}

var _ critic.Model = ClaudeCLIModel{}

// Args returns the arguments the CLI is run with for p.
func (m ClaudeCLIModel) Args(p critic.Prompt) []string {
	tools := m.Tools
	if tools == nil {
		tools = []string{"Read", "Grep", "Glob"}
	}

	args := []string{
		"--print",
		"--output-format", "text",
		"--append-system-prompt", p.System,
		"--tools", strings.Join(tools, ","),
		"--permission-mode", "dontAsk",
		"--strict-mcp-config",
		"--no-session-persistence",
	}
	if m.Model != "" {
		args = append(args, "--model", m.Model)
	}

	return args
}

// Complete implements critic.Model. The user prompt goes to the CLI's standard input.
func (m ClaudeCLIModel) Complete(ctx context.Context, p critic.Prompt) (string, error) {
	binary := m.Binary
	if binary == "" {
		binary = "claude"
	}

	cmd := exec.CommandContext(ctx, binary, m.Args(p)...)
	cmd.Dir = m.Dir
	cmd.Stdin = strings.NewReader(p.User)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("can't run %s: %w: %s", binary, err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
