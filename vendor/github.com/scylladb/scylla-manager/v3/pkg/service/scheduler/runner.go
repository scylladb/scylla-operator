// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"encoding/json"

	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// Runner is a glue between scheduler and agents doing work.
// There can be one Runner per TaskType registered in Service.
// Run ID needs to be preserved whenever agent wants to persist the running state.
// The run can be stopped by cancelling the context.
// In that case runner must return error reported by the context.
type Runner interface {
	Run(ctx context.Context, clusterID, taskID, runID uuid.UUID, properties json.RawMessage) error
}
