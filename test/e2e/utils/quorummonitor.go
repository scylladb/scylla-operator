// Copyright (c) 2022 ScyllaDB

package utils

import (
	"context"

	"github.com/gocql/gocql"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
)

type QuorumMonitor struct {
	*DataInserter
}

func NewQuorumMonitor(hosts []string) (*QuorumMonitor, error) {
	di, err := NewDataInserter(hosts, false, WithClusterConfigModifier(clusterConfigModifier), WithReadConsistency(gocql.Quorum))
	if err != nil {
		return nil, err
	}
	return &QuorumMonitor{di}, nil
}

type quorumMonitorQueryObserver struct {
}

func (k quorumMonitorQueryObserver) ObserveQuery(_ context.Context, oq gocql.ObservedQuery) {
	if oq.Err != nil {
		framework.Warnf("Query failed. Host: %+v. Metrics: %+v. Err: %v.", oq.Host, oq.Metrics, oq.Err)
	}
}

var queryObserver quorumMonitorQueryObserver

func clusterConfigModifier(clusterConfig *gocql.ClusterConfig) *gocql.ClusterConfig {
	clusterConfig.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	clusterConfig.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: 1}
	clusterConfig.QueryObserver = queryObserver

	return clusterConfig
}
