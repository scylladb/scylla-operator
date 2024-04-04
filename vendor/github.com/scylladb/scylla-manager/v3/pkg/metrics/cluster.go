// Copyright (C) 2017 ScyllaDB

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

type ClusterMetrics struct {
	name *prometheus.GaugeVec
}

func NewClusterMetrics() ClusterMetrics {
	g := gaugeVecCreator("cluster")

	return ClusterMetrics{
		name: g("Mapping from cluster ID to name.", "name", "cluster", "name"),
	}
}

// MustRegister shall be called to make the metrics visible by prometheus client.
func (m ClusterMetrics) MustRegister() ClusterMetrics {
	prometheus.MustRegister(
		m.name,
	)
	return m
}

// SetName updates "name" metric.
func (m ClusterMetrics) SetName(clusterID uuid.UUID, name string) {
	DeleteMatching(m.name, LabelMatcher("cluster", clusterID.String()))
	m.name.WithLabelValues(clusterID.String(), name).Add(0)
}
