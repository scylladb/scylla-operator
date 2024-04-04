// Copyright (C) 2017 ScyllaDB

package rcserver

import (
	"github.com/prometheus/client_golang/prometheus"
)

var agentUnexposedAccess = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "scylla_manager",
	Subsystem: "agent",
	Name:      "unexposed_access",
	Help:      "Attempted access to the unexposed endpoint of the agent",
}, []string{"addr", "path"})

func init() {
	prometheus.MustRegister(
		agentUnexposedAccess,
	)
}
