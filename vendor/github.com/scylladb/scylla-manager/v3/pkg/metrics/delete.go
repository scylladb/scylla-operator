// Copyright (C) 2017 ScyllaDB

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// CollectorDeleter extends prometheus.Collector with Delete.
type CollectorDeleter interface {
	prometheus.Collector
	Delete(labels prometheus.Labels) bool
}

// DeleteMatching removes metric instances with matching labels.
func DeleteMatching(c CollectorDeleter, matcher func(*dto.Metric) bool) {
	var data dto.Metric
	var toDelete []prometheus.Labels

	for m := range collect(c) {
		if err := m.Write(&data); err != nil {
			continue
		}
		if matcher(&data) {
			toDelete = append(toDelete, makeLabels(data.Label))
		}
	}

	for _, labels := range toDelete {
		c.Delete(labels)
	}
}

const unspecifiedValue = float64(-1)

// setGaugeVecMatching sets metric instances with matching labels to the
// given value.
func setGaugeVecMatching(c *prometheus.GaugeVec, value float64, matcher func(*dto.Metric) bool) { // nolint: unparam
	var (
		data   dto.Metric
		labels []prometheus.Labels
	)

	for m := range collect(c) {
		if err := m.Write(&data); err != nil {
			continue
		}
		if matcher(&data) {
			labels = append(labels, makeLabels(data.Label))
		}
	}

	for _, l := range labels {
		m, err := c.GetMetricWith(l)
		if err != nil {
			panic(err)
		}
		m.Set(value)
	}
}

func collect(c prometheus.Collector) chan prometheus.Metric {
	ch := make(chan prometheus.Metric)
	go func() {
		c.Collect(ch)
		close(ch)
	}()
	return ch
}

func makeLabels(pairs []*dto.LabelPair) prometheus.Labels {
	labels := make(prometheus.Labels)

	for _, kv := range pairs {
		if kv != nil {
			labels[kv.GetName()] = kv.GetValue()
		}
	}

	return labels
}
