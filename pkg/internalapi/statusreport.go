// Copyright (C) 2025 ScyllaDB

package internalapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
)

type NodeStatusReport struct {
	// ObservedNodes holds the list of node statuses as observed by this node.
	ObservedNodes []scyllav1alpha1.ObservedNodeStatus `json:"observedNodes,omitempty"`

	// Error holds an error message if the report could not be built.
	Error *string `json:"error,omitempty"`
}

func (nsr *NodeStatusReport) Decode(reader io.Reader) error {
	err := json.NewDecoder(reader).Decode(nsr)
	if err != nil {
		return fmt.Errorf("can't json decode node status report: %w", err)
	}

	return nil
}

func (nsr *NodeStatusReport) Encode() ([]byte, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(nsr)
	if err != nil {
		return nil, fmt.Errorf("can't json encode node status report: %w", err)
	}

	return buf.Bytes(), nil
}
