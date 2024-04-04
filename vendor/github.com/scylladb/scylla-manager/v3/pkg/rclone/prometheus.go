// Copyright (C) 2017 ScyllaDB

package rclone

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/fshttp"
)

// MustRegisterPrometheusMetrics registers rclone metrics with prometheus.
func MustRegisterPrometheusMetrics(namespace string) {
	// Accounting
	a := accounting.NewRcloneCollector(context.Background(), namespace)
	prometheus.MustRegister(a)

	// HTTP level metrics
	m := fshttp.NewMetrics(namespace)
	for _, c := range m.Collectors() {
		prometheus.MustRegister(c)
	}
	fshttp.DefaultMetrics = m
}
