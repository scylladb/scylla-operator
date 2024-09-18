// Copyright (c) 2024 ScyllaDB.

package internalapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type UpgradePhase string

const (
	PreHooksUpgradePhase    UpgradePhase = "PreHooks"
	RolloutInitUpgradePhase UpgradePhase = "RolloutInit"
	RolloutRunUpgradePhase  UpgradePhase = "RolloutRun"
	PostHooksUpgradePhase   UpgradePhase = "PostHooks"
)

type DatacenterUpgradeContext struct {
	State             UpgradePhase `json:"state"`
	FromVersion       string       `json:"fromVersion"`
	ToVersion         string       `json:"toVersion"`
	SystemSnapshotTag string       `json:"systemSnapshotTag"`
	DataSnapshotTag   string       `json:"dataSnapshotTag"`
}

func (uc *DatacenterUpgradeContext) Decode(reader io.Reader) error {
	err := json.NewDecoder(reader).Decode(uc)
	if err != nil {
		return fmt.Errorf("can't json decode ugprade context: %w", err)
	}
	return nil
}

func (uc *DatacenterUpgradeContext) Encode() ([]byte, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(uc)
	if err != nil {
		return nil, fmt.Errorf("can't json encode upgrade context: %w", err)
	}
	return buf.Bytes(), nil
}
