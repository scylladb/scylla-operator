// Copyright (C) 2017 ScyllaDB

package metrics

import (
	dto "github.com/prometheus/client_model/go"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// LabelMatcher returns a matcher checking only single label.
func LabelMatcher(name, value string) func(m *dto.Metric) bool {
	return func(m *dto.Metric) bool {
		for _, l := range m.GetLabel() {
			if l.GetName() == name && l.GetValue() == value {
				return true
			}
		}
		return false
	}
}

func clusterMatcher(clusterID uuid.UUID) func(m *dto.Metric) bool {
	return LabelMatcher("cluster", clusterID.String())
}
