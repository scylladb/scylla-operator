// Slo types lifted from github.com/grafana/slo/pkg/generated/models/slo/slo_type_gen.go
package gapi

import (
	"encoding/json"
	"fmt"
)

var sloPath string = "/api/plugins/grafana-slo-app/resources/v1/slo"

type Slos struct {
	Slos []Slo `json:"slos"`
}

const (
	QueryTypeFreeform  QueryType = "freeform"
	QueryTypeHistogram QueryType = "histogram"
	QueryTypeRatio     QueryType = "ratio"
	QueryTypeThreshold QueryType = "threshold"
)

const (
	ThresholdOperatorEmpty      ThresholdOperator = "<"
	ThresholdOperatorEqualEqual ThresholdOperator = "=="
	ThresholdOperatorN1         ThresholdOperator = "<="
	ThresholdOperatorN2         ThresholdOperator = ">="
	ThresholdOperatorN3         ThresholdOperator = ">"
)

type Alerting struct {
	Annotations []Label           `json:"annotations,omitempty"`
	FastBurn    *AlertingMetadata `json:"fastBurn,omitempty"`
	Labels      []Label           `json:"labels,omitempty"`
	SlowBurn    *AlertingMetadata `json:"slowBurn,omitempty"`
}

type AlertingMetadata struct {
	Annotations []Label `json:"annotations,omitempty"`
	Labels      []Label `json:"labels,omitempty"`
}

type DashboardRef struct {
	UID string `json:"UID"`
}

type FreeformQuery struct {
	Query string `json:"query"`
}

type HistogramQuery struct {
	GroupByLabels []string  `json:"groupByLabels,omitempty"`
	Metric        MetricDef `json:"metric"`
	Percentile    float64   `json:"percentile"`
	Threshold     Threshold `json:"threshold"`
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MetricDef struct {
	PrometheusMetric string  `json:"prometheusMetric"`
	Type             *string `json:"type,omitempty"`
}

type Objective struct {
	Value  float64 `json:"value"`
	Window string  `json:"window"`
}

type Query struct {
	Freeform  *FreeformQuery  `json:"freeform,omitempty"`
	Histogram *HistogramQuery `json:"histogram,omitempty"`
	Ratio     *RatioQuery     `json:"ratio,omitempty"`
	Threshold *ThresholdQuery `json:"threshold,omitempty"`
	Type      QueryType       `json:"type"`
}

type QueryType string

type RatioQuery struct {
	GroupByLabels []string  `json:"groupByLabels,omitempty"`
	SuccessMetric MetricDef `json:"successMetric"`
	TotalMetric   MetricDef `json:"totalMetric"`
}

type Slo struct {
	Alerting              *Alerting     `json:"alerting,omitempty"`
	Description           string        `json:"description"`
	DrillDownDashboardRef *DashboardRef `json:"drillDownDashboardRef,omitempty"`
	Labels                []Label       `json:"labels,omitempty"`
	Name                  string        `json:"name"`
	Objectives            []Objective   `json:"objectives"`
	Query                 Query         `json:"query"`
	UUID                  string        `json:"uuid"`
}

type Threshold struct {
	Operator ThresholdOperator `json:"operator"`
	Value    float64           `json:"value"`
}

type ThresholdOperator string

type ThresholdQuery struct {
	GroupByLabels []string  `json:"groupByLabels,omitempty"`
	Metric        MetricDef `json:"metric"`
	Threshold     Threshold `json:"threshold"`
}

type CreateSLOResponse struct {
	Message string `json:"message,omitempty"`
	UUID    string `json:"uuid,omitempty"`
}

// ListSlos retrieves a list of all Slos
func (c *Client) ListSlos() (Slos, error) {
	var slos Slos

	if err := c.request("GET", sloPath, nil, nil, &slos); err != nil {
		return Slos{}, err
	}

	return slos, nil
}

// GetSLO returns a single Slo based on its uuid
func (c *Client) GetSlo(uuid string) (Slo, error) {
	var slo Slo
	path := fmt.Sprintf("%s/%s", sloPath, uuid)

	if err := c.request("GET", path, nil, nil, &slo); err != nil {
		return Slo{}, err
	}

	return slo, nil
}

// CreateSLO creates a single Slo
func (c *Client) CreateSlo(slo Slo) (CreateSLOResponse, error) {
	response := CreateSLOResponse{}

	data, err := json.Marshal(slo)
	if err != nil {
		return response, err
	}

	if err := c.request("POST", sloPath, nil, data, &response); err != nil {
		return CreateSLOResponse{}, err
	}

	return response, err
}

// DeleteSLO deletes the Slo with the passed in UUID
func (c *Client) DeleteSlo(uuid string) error {
	path := fmt.Sprintf("%s/%s", sloPath, uuid)
	return c.request("DELETE", path, nil, nil, nil)
}

// UpdateSLO updates the Slo with the passed in UUID and Slo
func (c *Client) UpdateSlo(uuid string, slo Slo) error {
	path := fmt.Sprintf("%s/%s", sloPath, uuid)

	data, err := json.Marshal(slo)
	if err != nil {
		return err
	}

	if err := c.request("PUT", path, nil, data, nil); err != nil {
		return err
	}

	return nil
}
