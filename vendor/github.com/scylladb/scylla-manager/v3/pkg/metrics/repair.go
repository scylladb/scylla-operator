// Copyright (C) 2017 ScyllaDB

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

type RepairMetrics struct {
	progress            *prometheus.GaugeVec
	tokenRangesTotal    *prometheus.GaugeVec
	tokenRangesSuccess  *prometheus.GaugeVec
	tokenRangesError    *prometheus.GaugeVec
	inFlightJobs        *prometheus.GaugeVec
	inFlightTokenRanges *prometheus.GaugeVec
}

func NewRepairMetrics() RepairMetrics {
	g := gaugeVecCreator("repair")

	return RepairMetrics{
		progress: g("Total percentage repair progress.", "progress", "cluster"),
		tokenRangesTotal: g("Total number of token ranges to repair.",
			"token_ranges_total", "cluster", "keyspace", "table", "host"),
		tokenRangesSuccess: g("Number of repaired token ranges.",
			"token_ranges_success", "cluster", "keyspace", "table", "host"),
		tokenRangesError: g("Number of segments that failed to repair.",
			"token_ranges_error", "cluster", "keyspace", "table", "host"),
		inFlightJobs: g("Number of currently running Scylla repair jobs.",
			"inflight_jobs", "cluster", "host"),
		inFlightTokenRanges: g("Number of token ranges that are being repaired.",
			"inflight_token_ranges", "cluster", "host"),
	}
}

func (m RepairMetrics) all() []prometheus.Collector {
	return []prometheus.Collector{
		m.progress,
		m.tokenRangesTotal,
		m.tokenRangesSuccess,
		m.tokenRangesError,
		m.inFlightJobs,
		m.inFlightTokenRanges,
	}
}

// MustRegister shall be called to make the metrics visible by prometheus client.
func (m RepairMetrics) MustRegister() RepairMetrics {
	prometheus.MustRegister(m.all()...)
	return m
}

// ResetClusterMetrics resets all metrics labeled with the cluster.
func (m RepairMetrics) ResetClusterMetrics(clusterID uuid.UUID) {
	for _, c := range m.all() {
		setGaugeVecMatching(c.(*prometheus.GaugeVec), unspecifiedValue, clusterMatcher(clusterID))
	}
}

// SetTokenRanges updates "token_ranges_{total,success,error}" metrics.
func (m RepairMetrics) SetTokenRanges(clusterID uuid.UUID, keyspace, table, host string, total, success, errcnt int64) {
	l := prometheus.Labels{
		"cluster":  clusterID.String(),
		"keyspace": keyspace,
		"table":    table,
		"host":     host,
	}
	m.tokenRangesTotal.With(l).Set(float64(total))
	m.tokenRangesSuccess.With(l).Set(float64(success))
	m.tokenRangesError.With(l).Set(float64(errcnt))
}

// AddJob updates "inflight_{jobs,token_ranges}" metrics.
func (m RepairMetrics) AddJob(clusterID uuid.UUID, host string, tokenRanges int) {
	l := prometheus.Labels{
		"cluster": clusterID.String(),
		"host":    host,
	}
	m.inFlightJobs.With(l).Add(1)
	m.inFlightTokenRanges.With(l).Add(float64(tokenRanges))
}

// SubJob updates "inflight_{jobs,token_ranges}" metrics.
func (m RepairMetrics) SubJob(clusterID uuid.UUID, host string, tokenRanges int) {
	l := prometheus.Labels{
		"cluster": clusterID.String(),
		"host":    host,
	}
	m.inFlightJobs.With(l).Sub(1)
	m.inFlightTokenRanges.With(l).Sub(float64(tokenRanges))
}

// SetProgress sets "progress" metric.
func (m RepairMetrics) SetProgress(clusterID uuid.UUID, progress float64) {
	l := prometheus.Labels{
		"cluster": clusterID.String(),
	}
	m.progress.With(l).Set(progress)
}

// AddProgress updates "progress" metric.
func (m RepairMetrics) AddProgress(clusterID uuid.UUID, delta float64) {
	l := prometheus.Labels{
		"cluster": clusterID.String(),
	}
	m.progress.With(l).Add(delta)
}
