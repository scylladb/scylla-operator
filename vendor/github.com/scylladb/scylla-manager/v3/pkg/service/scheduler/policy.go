// Copyright (C) 2017 ScyllaDB

package scheduler

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// Policy decides if given task can be run.
type Policy interface {
	PreRun(clusterID, taskID, runID uuid.UUID) error
	PostRun(clusterID, taskID, runID uuid.UUID)
}

// PolicyRunner is a runner that uses policy to check if a task can be run.
type PolicyRunner struct {
	Policy Policy
	Runner Runner
}

// Run implements Runner.
func (pr PolicyRunner) Run(ctx context.Context, clusterID, taskID, runID uuid.UUID, properties json.RawMessage) error {
	if err := pr.Policy.PreRun(clusterID, taskID, runID); err != nil {
		return err
	}
	defer pr.Policy.PostRun(clusterID, taskID, runID)
	return pr.Runner.Run(ctx, clusterID, taskID, runID, properties)
}

var errClusterBusy = errors.New("another task is running")

// LockClusterPolicy is a policy that can execute only one task at a time.
type LockClusterPolicy struct {
	mu   sync.Mutex
	busy map[uuid.UUID]struct{}
}

func NewLockClusterPolicy() *LockClusterPolicy {
	return &LockClusterPolicy{
		busy: make(map[uuid.UUID]struct{}),
	}
}

// PreRun implements Policy.
func (p *LockClusterPolicy) PreRun(clusterID, _, _ uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.busy[clusterID]; ok {
		return errClusterBusy
	}

	p.busy[clusterID] = struct{}{}
	return nil
}

// PostRun implements Policy.
func (p *LockClusterPolicy) PostRun(clusterID, _, _ uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.busy, clusterID)
}
