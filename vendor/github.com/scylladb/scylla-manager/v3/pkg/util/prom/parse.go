// Copyright (C) 2017 ScyllaDB

package prom

import (
	"io"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// MetricFamily represent single prometheus metric.
type MetricFamily = dto.MetricFamily

// ParseText parses text format of prometheus metrics from input reader.
func ParseText(r io.Reader) (map[string]*MetricFamily, error) {
	var p expfmt.TextParser
	return p.TextToMetricFamilies(r)
}
