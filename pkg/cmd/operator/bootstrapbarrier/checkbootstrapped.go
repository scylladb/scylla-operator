// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type bootstrappedQueryResult []struct {
	Bootstrapped string `json:"bootstrapped"`
}

type CheckBootstrappedOptions struct {
	SSTableBootstrappedQueryResultPath string

	Bootstrapped bool
}

func (o *CheckBootstrappedOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.SSTableBootstrappedQueryResultPath, "sstable-bootstrapped-query-result-path", "", o.SSTableBootstrappedQueryResultPath, "Path to containing the JSON result of scylla-sstable query.")
}

func (o *CheckBootstrappedOptions) Validate() error {
	var errs []error

	if len(o.SSTableBootstrappedQueryResultPath) == 0 {
		errs = append(errs, fmt.Errorf("sstable-bootstrapped-query-result-path must not be empty"))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *CheckBootstrappedOptions) Complete() error {
	var err error

	var rawData []byte
	rawData, err = os.ReadFile(o.SSTableBootstrappedQueryResultPath)
	if err != nil {
		return fmt.Errorf("can't read scylla-sstable query result file %q: %w", o.SSTableBootstrappedQueryResultPath, err)
	}

	bootstrapped, err := parseSSTableBootstrappedQueryResult(rawData)
	if err != nil {
		return fmt.Errorf("can't parse scylla-sstable query result file %q: %w", o.SSTableBootstrappedQueryResultPath, err)
	}
	o.Bootstrapped = bootstrapped

	return nil
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
