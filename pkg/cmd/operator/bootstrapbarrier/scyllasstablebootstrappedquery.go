// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

type bootstrappedQueryResult []struct {
	Bootstrapped string `json:"bootstrapped"`
}

type scyllaSSTableBootstrappedQueryOptions struct {
	scyllaDataDir string
}

func (o *scyllaSSTableBootstrappedQueryOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.scyllaDataDir, "scylla-data-dir", "", o.scyllaDataDir, "Path to ScyllaDB data directory.")
}

func (o *scyllaSSTableBootstrappedQueryOptions) Validate() error {
	var errs []error

	if len(o.scyllaDataDir) == 0 {
		errs = append(errs, fmt.Errorf("scylla-data-dir must not be empty"))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *scyllaSSTableBootstrappedQueryOptions) Complete() error {
	return nil
}

func (o *scyllaSSTableBootstrappedQueryOptions) IsBootstrapped(ctx context.Context) (bool, error) {
	scyllaDBSystemLocalDataPattern := path.Join(o.scyllaDataDir, "/system/local-*/*-Data.db")
	files, err := filepath.Glob(scyllaDBSystemLocalDataPattern)
	if err != nil {
		return false, fmt.Errorf("can't glob files matching system local data pattern %q: %w", scyllaDBSystemLocalDataPattern, err)
	}
	if len(files) == 0 {
		klog.V(4).InfoS("No system local data sstable files found. Assuming that the node requires bootstrap.", "GlobPattern", scyllaDBSystemLocalDataPattern)
		return false, nil
	}

	args := []string{
		"sstable",
		"query",
		"--system-schema",
		"--keyspace=system",
		"--table=local",
		"--output-format=json",
		fmt.Sprintf("--scylla-data-dir=%s", o.scyllaDataDir),
		"--query=SELECT bootstrapped FROM scylla_sstable.local",
	}
	args = append(args, files...)

	cmd := exec.CommandContext(ctx, "/usr/bin/scylla", args...)

	klog.V(5).InfoS("Executing scylla-sstable query command", "command", cmd.String())
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return false, fmt.Errorf("can't execute scylla-sstable query command: %w", err)
		}

		klog.V(4).ErrorS(exitErr, "Failed to query bootstrap status with scylla-sstable query. Assuming the node requires bootstrap.", "stderr", strings.TrimSpace(string(exitErr.Stderr)))
		return false, nil
	}

	bootstrapped, err := parseSSTableBootstrappedQueryResult(bytes.TrimSpace(out))
	if err != nil {
		return false, fmt.Errorf("can't parse scylla-sstable query result: %w", err)
	}

	return bootstrapped, nil
}

func parseSSTableBootstrappedQueryResult(rawData []byte) (bool, error) {
	var data bootstrappedQueryResult
	err := json.Unmarshal(rawData, &data)
	if err != nil {
		return false, fmt.Errorf("can't unmarshall scylla-sstable query result: %w", err)
	}

	for _, res := range data {
		if res.Bootstrapped == "COMPLETED" {
			return true, nil
		}
	}

	return false, nil
}
