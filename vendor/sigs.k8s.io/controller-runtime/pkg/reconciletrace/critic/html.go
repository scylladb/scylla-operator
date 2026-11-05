/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package critic

import (
	_ "embed"
	"html/template"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ReportGroup is a set of reviews shown together, e.g. the episodes recorded while one test ran.
type ReportGroup struct {
	Name string
	// Status is shown next to the name, e.g. "passed" or "failed". Empty shows nothing.
	Status string
	// Notes are extra lines shown under the name, e.g. the namespaces the group's traces come from.
	Notes   []string
	Reports []*Report
	// Errors are the reviews that failed, as text.
	Errors []string
}

// HTMLOptions configure the HTML report.
type HTMLOptions struct {
	Title    string
	Subtitle string
	// Generated is shown in the header. Defaults to now.
	Generated time.Time
}

var severityOrder = map[string]int{"high": 0, "medium": 1, "low": 2}

type htmlFinding struct {
	Finding
	EvidenceLines []string
}

type htmlReport struct {
	*Report
	Findings []htmlFinding
	Signals  []signalRow
	Lines    int
}

type signalRow struct {
	Name  string
	Value string
	// Notable highlights a row worth a look.
	Notable bool
}

type htmlGroup struct {
	ReportGroup
	ID         int
	Reports    []htmlReport
	Severities map[string]int
	Findings   int
	Reviewed   bool
	Reconciles int
	// ReconcileErrors doesn't shadow the embedded group's Errors.
	ReconcileErrors int
	NoOps           int
	SelfIdle        int
	Conflicts       int
}

type htmlData struct {
	HTMLOptions
	Groups     []htmlGroup
	Failed     int
	Episodes   int
	Reviewed   int
	Findings   int
	Severities map[string]int
	Categories []categoryCount
}

type categoryCount struct {
	Name  string
	Count int
}

func signalRows(s Signals) []signalRow {
	repeated := make([]string, 0, len(s.RepeatedlyWrittenObjects))
	for _, rw := range s.RepeatedlyWrittenObjects {
		repeated = append(repeated, rw.Object+" ("+itoa(rw.Reconciles)+" reconciles)")
	}
	repeatedValue := "none"
	if len(repeated) > 0 {
		repeatedValue = strings.Join(repeated, "\n")
	}

	return []signalRow{
		{Name: "Reconciles", Value: itoa(s.Reconciles) + " over " + s.Span.Round(time.Millisecond).String() + ", " + itoa(s.IdleReconciles) + " wrote nothing"},
		{Name: "Reconcile time", Value: "total " + s.TotalReconcile.Round(time.Millisecond).String() + ", max " + s.MaxReconcile.Round(time.Millisecond).String() + ", max queue wait " + s.MaxQueueWait.Round(time.Millisecond).String()},
		{Name: "Requeues", Value: formatCounts(s.Requeues)},
		{Name: "Reconcile errors", Value: formatIDs(s.ReconcileErrors), Notable: len(s.ReconcileErrors) > 0},
		{Name: "No-op writes", Value: formatIDs(s.NoOpWrites), Notable: len(s.NoOpWrites) > 0},
		{Name: "Conflicts", Value: formatIDs(s.Conflicts), Notable: len(s.Conflicts) > 0},
		{Name: "Other failed writes", Value: formatIDs(s.FailedWrites), Notable: len(s.FailedWrites) > 0},
		{Name: "Self-triggered idle reconciles", Value: formatIDs(s.SelfTriggeredIdleReconciles), Notable: len(s.SelfTriggeredIdleReconciles) > 0},
		{Name: "Objects written in several reconciles", Value: repeatedValue},
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// WriteHTML writes a self-contained HTML page with one collapsible section per group, one per episode in it, and one
// per finding.
func WriteHTML(w io.Writer, opts HTMLOptions, groups []ReportGroup) error {
	if opts.Generated.IsZero() {
		opts.Generated = time.Now()
	}

	data := htmlData{
		HTMLOptions: opts,
		Severities:  map[string]int{},
	}
	categories := map[string]int{}

	for i, g := range groups {
		hg := htmlGroup{ReportGroup: g, ID: i, Severities: map[string]int{}}
		if g.Status == "failed" {
			data.Failed++
		}
		for _, r := range g.Reports {
			hr := htmlReport{Report: r, Signals: signalRows(r.Signals), Lines: strings.Count(r.Timeline.Text, "\n")}
			findings := slices.Clone(r.Findings)
			slices.SortStableFunc(findings, func(a, b Finding) int {
				return severityRank(a.Severity) - severityRank(b.Severity)
			})
			for _, f := range findings {
				ids := slices.Clone(f.Evidence)
				slices.Sort(ids)
				var lines []string
				for _, id := range ids {
					lines = append(lines, r.Timeline.Lines[id])
				}
				hr.Findings = append(hr.Findings, htmlFinding{Finding: f, EvidenceLines: lines})
				hg.Severities[f.Severity]++
				data.Severities[f.Severity]++
				categories[f.Category]++
			}
			hg.Findings += len(r.Findings)
			hg.Reviewed = hg.Reviewed || r.Reviewed
			hg.Reconciles += r.Signals.Reconciles
			hg.ReconcileErrors += len(r.Signals.ReconcileErrors)
			hg.NoOps += len(r.Signals.NoOpWrites)
			hg.SelfIdle += len(r.Signals.SelfTriggeredIdleReconciles)
			hg.Conflicts += len(r.Signals.Conflicts)
			hg.Reports = append(hg.Reports, hr)

			data.Episodes++
			if r.Reviewed {
				data.Reviewed++
			}
		}
		data.Findings += hg.Findings
		data.Groups = append(data.Groups, hg)
	}

	for _, name := range slices.Sorted(maps.Keys(categories)) {
		data.Categories = append(data.Categories, categoryCount{Name: name, Count: categories[name]})
	}
	slices.SortStableFunc(data.Categories, func(a, b categoryCount) int { return b.Count - a.Count })

	return htmlTemplate.Execute(w, data)
}

func severityRank(s string) int {
	if r, ok := severityOrder[s]; ok {
		return r
	}

	return len(severityOrder)
}

//go:embed report.html.tmpl
var htmlTemplateText string

var htmlTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"sev":   func(m map[string]int, s string) int { return m[s] },
	"lower": strings.ToLower,
	"join":  strings.Join,
	"fmtTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04 MST")
	},
}).Parse(htmlTemplateText))
