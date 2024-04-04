// Copyright (C) 2017 ScyllaDB

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

func gaugeVecCreator(subsystem string) func(help, name string, labels ...string) *prometheus.GaugeVec {
	return func(help, name string, labels ...string) *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "scylla_manager",
			Subsystem: subsystem,
			Name:      name,
			Help:      help,
		}, labels)
	}
}
