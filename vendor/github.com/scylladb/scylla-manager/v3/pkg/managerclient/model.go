// Copyright (C) 2017 ScyllaDB

package managerclient

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/pkg/errors"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient/table"
	"github.com/scylladb/scylla-manager/v3/pkg/service/scheduler"
	"github.com/scylladb/scylla-manager/v3/pkg/util/inexlist"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-manager/v3/pkg/util/version"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
	"github.com/scylladb/termtables"
)

// ErrorResponse is returned in case of an error.
type ErrorResponse = models.ErrorResponse

// TableRenderer is the interface that components need to implement
// if they can render themselves as tables.
type TableRenderer interface {
	Render(io.Writer) error
}

// Cluster is cluster.Cluster representation.
type Cluster = models.Cluster

// ClusterSlice is []*cluster.Cluster representation.
type ClusterSlice []*models.Cluster

// Render renders ClusterSlice in a tabular format.
func (cs ClusterSlice) Render(w io.Writer) error {
	t := table.New("ID", "Name", "Port", "CQL credentials")
	for _, c := range cs {
		p := "default"
		if c.Port != 0 {
			p = fmt.Sprint(c.Port)
		}
		creds := "not set"
		if c.Username != "" && c.Password != "" {
			creds = "set"
		}
		t.AddRow(c.ID, c.Name, p, creds)
	}
	if _, err := w.Write([]byte(t.String())); err != nil {
		return err
	}

	return nil
}

// ClusterStatus contains cluster status info.
type ClusterStatus models.ClusterStatus

func (cs ClusterStatus) tableHeaders() []interface{} {
	headers := []interface{}{"Address", "Uptime", "CPUs", "Memory", "Scylla", "Agent", "Host ID"}
	apis := []interface{}{"CQL", "REST"}

	if cs.hasAnyAlternator() {
		apis = append([]interface{}{"Alternator"}, apis...)
	}

	return append([]interface{}{""}, append(apis, headers...)...)
}

func (cs ClusterStatus) hasAnyAlternator() bool {
	for _, s := range cs {
		if s.AlternatorStatus != "" {
			return true
		}
	}
	return false
}

func (cs ClusterStatus) addRow(t *table.Table, rows ...interface{}) {
	unpacked := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		switch v := r.(type) {
		case []interface{}:
			unpacked = append(unpacked, v...)
		default:
			unpacked = append(unpacked, v)
		}
	}
	headers := cs.tableHeaders()
	t.AddRow(unpacked[:len(headers)]...)
}

// Render renders ClusterStatus in a tabular format.
func (cs ClusterStatus) Render(w io.Writer) error {
	if len(cs) == 0 {
		return nil
	}

	var (
		dc     = cs[0].Dc
		t      = table.New(cs.tableHeaders()...)
		errors []string
	)

	for _, s := range cs {
		if s.Dc != dc {
			fmt.Fprintf(w, "Datacenter: %s\n%s", dc, t)
			dc = s.Dc
			t = table.New(cs.tableHeaders()...)

			if len(errors) > 0 {
				fmt.Fprintf(w, "Errors:\n- %s\n\n", strings.Join(errors, "\n- "))
				errors = nil
			}
		}

		var apiStatuses []interface{}

		if s.AlternatorStatus != "" {
			status := s.AlternatorStatus
			if s.Ssl {
				status += " SSL"
			}
			apiStatuses = append(apiStatuses, fmt.Sprintf("%s (%.0fms)", status, s.AlternatorRttMs))
		} else if cs.hasAnyAlternator() {
			apiStatuses = append(apiStatuses, "-")
		}
		if s.AlternatorCause != "" {
			errors = append(errors, fmt.Sprintf("%s alternator: %s", s.Host, s.AlternatorCause))
		}

		if s.CqlStatus != "" {
			status := s.CqlStatus
			if s.Ssl {
				status += " SSL"
			}
			apiStatuses = append(apiStatuses, fmt.Sprintf("%s (%.0fms)", status, s.CqlRttMs))
		} else {
			apiStatuses = append(apiStatuses, "-")
		}
		if s.CqlCause != "" {
			errors = append(errors, fmt.Sprintf("%s CQL: %s", s.Host, s.CqlCause))
		}

		if s.RestStatus != "" {
			apiStatuses = append(apiStatuses, fmt.Sprintf("%s (%.0fms)", s.RestStatus, s.RestRttMs))
		} else {
			apiStatuses = append(apiStatuses, "-")
		}
		if s.RestCause != "" {
			errors = append(errors, fmt.Sprintf("%s REST: %s", s.Host, s.RestCause))
		}

		var (
			cpus          = "-"
			mem           = "-"
			scyllaVersion = "-"
			agentVersion  = "-"
			uptime        = "-"
		)
		if s.CPUCount > 0 {
			cpus = fmt.Sprintf("%d", s.CPUCount)
		}
		if s.TotalRAM > 0 {
			mem = FormatSizeSuffix(s.TotalRAM)
		}
		if s.ScyllaVersion != "" {
			scyllaVersion = version.Short(s.ScyllaVersion)
		}
		if s.AgentVersion != "" {
			agentVersion = version.Short(s.AgentVersion)
		}
		if s.Uptime > 0 {
			uptime = (time.Duration(s.Uptime) * time.Second).String()
		}

		cs.addRow(t, s.Status, apiStatuses, s.Host, uptime, cpus, mem, scyllaVersion, agentVersion, s.HostID)
	}

	fmt.Fprintf(w, "Datacenter: %s\n%s", dc, t)
	if len(errors) > 0 {
		fmt.Fprintf(w, "Errors:\n- %s\n\n", strings.Join(errors, "\n- "))
	}
	return nil
}

// Task is a scheduler.Task representation.
type Task = models.Task

func makeTaskUpdate(t *Task) *models.TaskUpdate {
	return &models.TaskUpdate{
		Type:       t.Type,
		Enabled:    t.Enabled,
		Name:       t.Name,
		Schedule:   t.Schedule,
		Tags:       t.Tags,
		Properties: t.Properties,
	}
}

// TaskInfo allows for rendering of Task information i.e. schedule and properties.
type TaskInfo struct {
	*TaskListItem
}

const taskInfoTemplate = `Name:	{{ TaskID .TaskListItem }}
Cron:   {{ CronDesc .TaskListItem }}
{{ if .Schedule.Window -}}
Window:	{{ WindowDesc .Schedule.Window }}
{{ end -}}
{{ if .Schedule.Timezone -}}
Tz:	{{ .Schedule.Timezone }}
{{ end -}}
{{ if .Schedule.NumRetries -}}
Retry:	{{ .Schedule.NumRetries }} {{ if .Schedule.RetryWait }}(initial backoff {{ .Schedule.RetryWait }}){{ end }}
{{ end -}}

{{ if .Properties }}
Properties:
{{- range $key, $val := .Properties }}
- {{ FormatKey $key }}: {{ FormatValue $val -}}
{{ end }}
{{ end -}}

`

// Render implements Renderer interface.
func (t TaskInfo) Render(w io.Writer) error {
	temp := template.Must(template.New("target").Funcs(template.FuncMap{
		"TaskID": func(i *TaskListItem) string {
			if i.Name != "" {
				return taskJoin(i.Type, i.Name)
			}
			return taskJoin(i.Type, i.ID)
		},
		"CronDesc": func(i *TaskListItem) string {
			s := i.Schedule.Cron
			if s == "" {
				s = i.Schedule.Interval
			}

			d := DescribeCron(s)
			if d == "" {
				nextStr := FormatTimePointer(i.NextActivation)
				if nextStr != "" {
					d = "next activation date: " + nextStr
				} else {
					d = "no activations scheduled"
				}
			}

			if s != "" {
				return s + " (" + d + ")"
			}
			return d
		},
		"WindowDesc": func(s []string) string {
			v := ""
			for i := 0; i < len(s)/2; i++ {
				v += fmt.Sprint(s[i*2:(i+1)*2]) + " "
			}
			return v
		},
		"FormatKey":   formatKey,
		"FormatValue": formatValue,
	}).Parse(taskInfoTemplate))
	return temp.Execute(w, t)
}

func formatKey(k string) string {
	return strings.ReplaceAll(k, "_", "-")
}

func formatValue(v any) string {
	switch t := v.(type) {
	case []string:
		return "'" + strings.Join(t, ",") + "'"
	case []interface{}:
		s := make([]string, len(t))
		for i := range t {
			s[i] = fmt.Sprint(t[i])
		}
		return "'" + strings.Join(s, ",") + "'"
	default:
		return fmt.Sprint(t)
	}
}

// RepairTarget is a representing results of dry running repair task.
type RepairTarget struct {
	models.RepairTarget
	Schedule   *Schedule
	ShowTables int
}

const repairTargetTemplate = `{{ if .Schedule.Cron -}}
Cron:	{{ .Schedule.Cron }} {{ CronDesc .Schedule.Cron }}
{{ if .Schedule.Timezone -}}
Tz:	{{ .Schedule.Timezone }}
{{ end }}
{{ end -}}
{{ if .Host -}}

Host: {{ .Host }}

{{ end -}}
{{ if .IgnoreHosts -}}

Ignore Hosts:
{{ range .IgnoreHosts -}}
  - {{ . }}
{{ end }}
{{ end -}}

Data Centers:
{{ range .Dc }}  - {{ . }}
{{ end }}
Keyspaces:
{{- range .Units }}
  - {{ .Keyspace }} {{ FormatTables .Tables .AllTables -}}
{{ end }}

`

// Render implements Renderer interface.
func (t RepairTarget) Render(w io.Writer) error {
	temp := template.Must(template.New("target").Funcs(template.FuncMap{
		"FormatTables": func(tables []string, all bool) string {
			return FormatTables(t.ShowTables, tables, all)
		},
		"CronDesc": func(s string) string {
			d := DescribeCron(s)
			if d != "" {
				d = "(" + d + ")"
			}
			return d
		},
	}).Parse(repairTargetTemplate))
	return temp.Execute(w, t)
}

// BackupTarget is a representing results of dry running backup task.
type BackupTarget struct {
	models.BackupTarget
	Schedule   *Schedule
	ShowTables int
}

const backupTargetTemplate = `{{ if .Schedule.Cron -}}
Cron:	{{ .Schedule.Cron }} {{ CronDesc .Schedule.Cron }}
{{ if .Schedule.Timezone -}}
Tz:	{{ .Schedule.Timezone }}
{{ end }}
{{ end -}}
Data Centers:
{{ range .Dc }}  - {{ . }}
{{ end }}
Keyspaces:
{{- range .Units }}
  - {{ .Keyspace }} {{ FormatTables .Tables .AllTables }}
{{- end }}

Disk size: ~{{ FormatSizeSuffix .Size }}

Locations:
{{- range .Location }}
  - {{ . }}
{{- end }}

Bandwidth Limits:
{{- if .RateLimit -}}
{{ range .RateLimit }}
  - {{ . }} MiB/s
{{- end }}
{{- else }}
  - Unlimited
{{- end }}

Snapshot Parallel Limits:
{{- if .SnapshotParallel -}}
{{- range .SnapshotParallel }}
  - {{ . }}
{{- end }}
{{- else }}
  - All hosts in parallel
{{- end }}

Upload Parallel Limits:
{{- if .UploadParallel -}}
{{- range .UploadParallel }}
  - {{ . }}
{{- end }}
{{- else }}
  - All hosts in parallel
{{- end }}

Retention Policy:
{{ FormatRetentionPolicy .Retention .RetentionDays }}
`

// Render implements Renderer interface.
func (t BackupTarget) Render(w io.Writer) error {
	temp := template.Must(template.New("target").Funcs(template.FuncMap{
		"FormatSizeSuffix": FormatSizeSuffix,
		"FormatTables": func(tables []string, all bool) string {
			return FormatTables(t.ShowTables, tables, all)
		},
		"CronDesc": func(s string) string {
			d := DescribeCron(s)
			if d != "" {
				d = "(" + d + ")"
			}
			return d
		},
		"FormatRetentionPolicy": FormatRetentionPolicy,
	}).Parse(backupTargetTemplate))
	return temp.Execute(w, t)
}

// RestoreTarget is a representing results of dry running restore task.
type RestoreTarget struct {
	models.RestoreTarget
	Schedule   *Schedule
	ShowTables int
}

const restoreTargetTemplate = `{{ if .Schedule.Cron -}}
Cron:	{{ .Schedule.Cron }} {{ CronDesc .Schedule.Cron }}
{{ if .Schedule.Timezone -}}
Tz:	{{ .Schedule.Timezone }}
{{ end }}
{{ end -}}
Restored tables:
{{- range .Units }}
  - {{ .Keyspace }}: {{ FormatSizeSuffix .Size }} {{ FormatRestoreTables .Tables }}
{{- end }}
{{ if .Views }}
Restored views:
{{- range .Views }}
  - {{ .Keyspace }}.{{ .View }}
{{- end }}
{{ end }}
Disk size: ~{{ FormatSizeSuffix .Size }}

Locations:
{{- range .Location }}
  - {{ . }}
{{- end }}

Snapshot Tag:	{{ .SnapshotTag }}
Batch Size:     {{ .BatchSize }}
Parallel:       {{ .Parallel }}
`

// Render implements Renderer interface.
func (t RestoreTarget) Render(w io.Writer) error {
	temp := template.Must(template.New("target").Funcs(template.FuncMap{
		"FormatSizeSuffix": FormatSizeSuffix,
		"FormatRestoreTables": func(tables []*models.RestoreTable) string {
			return FormatRestoreTables(t.ShowTables, tables)
		},
		"CronDesc": func(s string) string {
			d := DescribeCron(s)
			if d != "" {
				d = "(" + d + ")"
			}
			return d
		},
	}).Parse(restoreTargetTemplate))
	return temp.Execute(w, t)
}

// TaskListItem is a representation of scheduler.Task with additional fields from scheduler.
type TaskListItem = models.TaskListItem

// TaskListItemSlice is a representation of a slice of scheduler.Task with additional fields from scheduler.
type TaskListItemSlice = []*models.TaskListItem

// TaskListItems is a representation of []*scheduler.Task with additional fields from scheduler.
type TaskListItems struct {
	TaskListItemSlice
	All       bool
	ShowIDs   bool
	ShowProps bool
}

// Render renders TaskListItems in a tabular format.
func (li TaskListItems) Render(w io.Writer) error {
	var doneRestoreTask bool

	columns := []any{"Task", "Schedule", "Window", "Timezone", "Success", "Error", "Last Success", "Last Error", "Status", "Next"}
	if li.ShowProps {
		columns = append(columns, "Properties")
	}
	p := table.New(columns...)
	for _, t := range li.TaskListItemSlice {
		if t.Type == RestoreTask && t.Status == TaskStatusDone {
			doneRestoreTask = true
		}

		var id string
		if t.Name != "" && !li.ShowIDs {
			id = taskJoin(t.Type, t.Name)
		} else {
			id = taskJoin(t.Type, t.ID)
		}
		if li.All && !t.Enabled {
			id = "*" + id
		}

		emptySpec := scheduler.CronSpecification{}
		bytesEmptySpec, err := json.Marshal(emptySpec)
		if err != nil {
			return errors.New("cannot marshall empty cron specification object")
		}

		var schedule string
		if t.Schedule.Cron != "" && t.Schedule.Cron != string(bytesEmptySpec) {
			var cronSpec scheduler.CronSpecification
			err := json.Unmarshal([]byte(t.Schedule.Cron), &cronSpec)
			if err != nil {
				schedule = t.Schedule.Cron
			} else {
				schedule = cronSpec.Spec
				if cronSpec.StartDate.After(timeutc.Now()) {
					c := scheduler.MustCron(cronSpec.Spec, cronSpec.StartDate)
					schedule += fmt.Sprintf(" with first activation after %s",
						c.Next(cronSpec.StartDate).Format("2006-01-02 15:04:05"))
				}
			}
		} else if t.Schedule.Interval != "" {
			schedule = t.Schedule.Interval
		}

		status := t.Status
		if status == TaskStatusError && t.Retry > 0 {
			status += fmt.Sprintf(" (%d/%d)", t.Retry-1, t.Schedule.NumRetries)
		}

		var next string
		if t.Suspended {
			next = "[SUSPENDED]"
		} else {
			next = FormatTimePointer(t.NextActivation)
		}

		row := []any{
			id, schedule, strings.Join(t.Schedule.Window, ","), t.Schedule.Timezone,
			t.SuccessCount, t.ErrorCount, FormatTimePointer(t.LastSuccess), FormatTimePointer(t.LastError),
			status, next,
		}
		if li.ShowProps {
			props, ok := t.Properties.(map[string]any)
			if !ok {
				return errors.New("can't cast task properties into map[string]any")
			}

			var fmtProps []string
			for k, v := range props {
				fmtProps = append(fmtProps, formatKey(k)+": "+formatValue(v))
			}

			row = append(row, strings.Join(fmtProps, ", "))
		}

		p.AddRow(row...)
	}
	fmt.Fprint(w, p)

	if doneRestoreTask {
		fmt.Fprint(w, " NOTE: all completed restore tasks require specific follow-up action - see docs for more information\n")
	}

	return nil
}

// Schedule is a scheduler.Schedule representation.
type Schedule = models.Schedule

// TaskRun is a scheduler.TaskRun representation.
type TaskRun = models.TaskRun

// TaskRunSlice is a []*scheduler.TaskRun representation.
type TaskRunSlice []*TaskRun

// Render renders TaskRunSlice in a tabular format.
func (tr TaskRunSlice) Render(w io.Writer, printCause bool) error {
	t := table.New("ID", "Start time", "Duration", "Status")
	for _, r := range tr {
		s := r.Status
		if printCause && r.Cause != "" {
			t.LimitColumnLength(3)
			s += " " + r.Cause
		}
		t.AddRow(r.ID, FormatTime(r.StartTime), FormatDuration(r.StartTime, r.EndTime), s)
	}
	if _, err := w.Write([]byte(t.String())); err != nil {
		return err
	}
	return nil
}

// RepairProgress contains shard progress info.
type RepairProgress struct {
	*models.TaskRunRepairProgress
	Task       *Task
	Detailed   bool
	ShowTables int

	hostFilter     inexlist.InExList
	keyspaceFilter inexlist.InExList
}

// SetHostFilter adds filtering rules used for rendering for host details.
func (rp *RepairProgress) SetHostFilter(filters []string) (err error) {
	rp.hostFilter, err = inexlist.ParseInExList(filters)
	return
}

// SetKeyspaceFilter adds filtering rules used for rendering for keyspace details.
func (rp *RepairProgress) SetKeyspaceFilter(filters []string) (err error) {
	rp.keyspaceFilter, err = inexlist.ParseInExList(filters)
	return
}

// hideKeyspace returns true if provided keyspace should be hidden.
func (rp RepairProgress) hideKeyspace(keyspace string) bool {
	if rp.keyspaceFilter.Size() > 0 {
		if rp.keyspaceFilter.FirstMatch(keyspace) == -1 {
			return true
		}
	}
	return false
}

func (rp RepairProgress) hideHost(host string) bool {
	if rp.hostFilter.Size() > 0 {
		return rp.hostFilter.FirstMatch(host) == -1
	}
	return false
}

// Render renders *RepairProgress in a tabular format.
func (rp RepairProgress) Render(w io.Writer) error {
	if err := rp.addHeader(w); err != nil {
		return err
	}

	if rp.Progress == nil {
		return nil
	}

	t := table.New()
	rp.addRepairTableProgress(t)
	t.SetColumnAlignment(termtables.AlignRight, 1)
	if _, err := io.WriteString(w, t.String()); err != nil {
		return err
	}

	if rp.Detailed {
		for _, h := range rp.Progress.Hosts {
			if rp.hideHost(h.Host) {
				continue
			}

			fmt.Fprintf(w, "\nHost: %s\n", h.Host)
			d := table.New()
			d.AddRow("Keyspace", "Table", "Progress", "Token Ranges", "Success", "Error", "Started at", "Completed at", "Duration")
			d.AddSeparator()
			ks := ""
			for _, t := range h.Tables {
				if rp.hideKeyspace(t.Keyspace) {
					continue
				}

				if ks == "" {
					ks = t.Keyspace
				} else if ks != t.Keyspace {
					ks = t.Keyspace
					d.AddSeparator()
				}

				rp.addRepairTableDetailedProgress(d, t)
			}
			d.SetColumnAlignment(termtables.AlignRight, 2, 3, 4, 5, 6, 7, 8)
			if _, err := w.Write([]byte(d.String())); err != nil {
				return err
			}
		}
	}
	return nil
}

var repairProgressTemplate = `{{ with .Run -}}
Run:		{{ .ID }}
Status:		{{ .Status }}
{{- if .Cause }}
Cause:		{{ FormatError .Cause }}

{{- end }}
{{- if not (isZero .StartTime) }}
Start time:	{{ FormatTime .StartTime }}
{{- end -}}
{{- if not (isZero .EndTime) }}
End time:	{{ FormatTime .EndTime }}
{{- end }}
Duration:	{{ FormatDuration .StartTime .EndTime }} 
{{- end }}
{{- with .Progress }}
Progress:	{{ FormatTotalRepairProgress .SuccessPercentage .ErrorPercentage }}
Intensity:	{{ FormatRepairIntensity .Intensity .MaxIntensity }}
Parallel:	{{ FormatRepairParallel .Parallel .MaxParallel }}
{{ if .Host }}Host:	{{ .Host }}
{{ end -}}
{{ if .Dcs }}Datacenters:	{{ range .Dcs }}
  - {{ . }}
{{- end }}
{{ end -}}
{{ else }}
Progress:	-
{{ end }}
`

func (rp RepairProgress) addHeader(w io.Writer) error {
	temp := template.Must(template.New("repair_progress").Funcs(template.FuncMap{
		"isZero":                    isZero,
		"FormatTime":                FormatTime,
		"FormatDuration":            FormatDuration,
		"FormatError":               FormatError,
		"FormatRepairProgress":      FormatRepairProgress,
		"FormatTotalRepairProgress": FormatTotalRepairProgress,
		"FormatRepairIntensity":     FormatRepairIntensity,
		"FormatRepairParallel":      FormatRepairParallel,
	}).Parse(repairProgressTemplate))
	return temp.Execute(w, rp)
}

func (rp RepairProgress) addRepairTableProgress(d *table.Table) {
	if len(rp.Progress.Tables) > 0 {
		d.AddRow("Keyspace", "Table", "Progress", "Duration")
		d.AddSeparator()
	}

	ks := ""
	for _, t := range rp.Progress.Tables {
		if rp.hideKeyspace(t.Keyspace) {
			continue
		}
		if ks == "" {
			ks = t.Keyspace
		} else if ks != t.Keyspace {
			ks = t.Keyspace
			d.AddSeparator()
		}
		p := "-"
		if t.TokenRanges > 0 {
			p = FormatRepairProgress(t.TokenRanges, t.Success, t.Error)
		}

		d.AddRow(t.Keyspace, t.Table, p, FormatMsDuration(t.DurationMs))
	}
}

func (rp RepairProgress) addRepairTableDetailedProgress(d *table.Table, t *models.TableRepairProgress) {
	d.AddRow(t.Keyspace,
		t.Table,
		FormatRepairProgress(t.TokenRanges, t.Success, t.Error),
		t.TokenRanges,
		t.Success,
		t.Error,
		FormatTimePointer(t.StartedAt),
		FormatTimePointer(t.CompletedAt),
		FormatMsDuration(t.DurationMs),
	)
}

// BackupProgress contains shard progress info.
type BackupProgress struct {
	*models.TaskRunBackupProgress
	Task     *Task
	Detailed bool
	Errors   []string

	hostFilter     inexlist.InExList
	keyspaceFilter inexlist.InExList
}

// SetHostFilter adds filtering rules used for rendering for host details.
func (bp *BackupProgress) SetHostFilter(filters []string) (err error) {
	bp.hostFilter, err = inexlist.ParseInExList(filters)
	return
}

// SetKeyspaceFilter adds filtering rules used for rendering for keyspace details.
func (bp *BackupProgress) SetKeyspaceFilter(filters []string) (err error) {
	bp.keyspaceFilter, err = inexlist.ParseInExList(filters)
	return
}

// AggregateErrors collects all errors from the table progress.
func (bp *BackupProgress) AggregateErrors() {
	if bp.Progress == nil || bp.Run.Status != TaskStatusError {
		return
	}
	for i := range bp.Progress.Hosts {
		for j := range bp.Progress.Hosts[i].Keyspaces {
			for _, t := range bp.Progress.Hosts[i].Keyspaces[j].Tables {
				if t.Error != "" {
					bp.Errors = append(bp.Errors, t.Error)
				}
			}
		}
	}
}

// Render renders *BackupProgress in a tabular format.
func (bp BackupProgress) Render(w io.Writer) error {
	if err := bp.addHeader(w); err != nil {
		return err
	}

	if bp.Progress != nil && bp.Progress.Size > 0 {
		t := table.New()
		bp.addHostProgress(t)
		if _, err := io.WriteString(w, t.String()); err != nil {
			return err
		}
	}

	if bp.Detailed && bp.Progress != nil && bp.Progress.Size > 0 {
		if err := bp.addKeyspaceProgress(w); err != nil {
			return err
		}
	}
	return nil
}

func (bp BackupProgress) addHostProgress(t *table.Table) {
	t.AddRow("Host", "Progress", "Size", "Success", "Deduplicated", "Failed")
	t.AddSeparator()
	for _, h := range bp.Progress.Hosts {
		if bp.hideHost(h.Host) {
			continue
		}
		p := "-"
		if len(h.Keyspaces) > 0 {
			p = FormatUploadProgress(h.Size, h.Uploaded, h.Skipped, h.Failed)
		}
		success := h.Uploaded + h.Skipped
		t.AddRow(h.Host, p,
			FormatSizeSuffix(h.Size),
			FormatSizeSuffix(success),
			FormatSizeSuffix(h.Skipped),
			FormatSizeSuffix(h.Failed),
		)
	}
	t.SetColumnAlignment(termtables.AlignRight, 1, 2, 3, 4, 5)
}

func (bp BackupProgress) addKeyspaceProgress(w io.Writer) error {
	for _, h := range bp.Progress.Hosts {
		if bp.hideHost(h.Host) {
			continue
		}
		fmt.Fprintf(w, "\nHost: %s\n", h.Host)

		t := table.New("Keyspace", "Table", "Progress", "Size", "Success", "Deduplicated", "Failed", "Started at", "Completed at")
		for i, ks := range h.Keyspaces {
			if bp.hideKeyspace(ks.Keyspace) {
				break
			}
			if i > 0 {
				t.AddSeparator()
			}

			rowAdded := false
			for _, tbl := range ks.Tables {
				startedAt := strfmt.DateTime{}
				completedAt := strfmt.DateTime{}
				if tbl.StartedAt != nil {
					startedAt = *tbl.StartedAt
				}
				if tbl.CompletedAt != nil {
					completedAt = *tbl.CompletedAt
				}
				success := tbl.Uploaded + tbl.Skipped
				t.AddRow(
					ks.Keyspace,
					tbl.Table,
					FormatUploadProgress(tbl.Size,
						tbl.Uploaded,
						tbl.Skipped,
						tbl.Failed),
					FormatSizeSuffix(tbl.Size),
					FormatSizeSuffix(success),
					FormatSizeSuffix(tbl.Skipped),
					FormatSizeSuffix(tbl.Failed),
					FormatTime(startedAt),
					FormatTime(completedAt),
				)
				rowAdded = true
			}
			if !rowAdded {
				// Separate keyspaces with no table rows
				t.AddRow("-", "-", "-", "-", "-", "-", "-", "-", "-")
			}
		}
		t.SetColumnAlignment(termtables.AlignRight, 2, 3, 4, 5, 6)
		if _, err := w.Write([]byte(t.String())); err != nil {
			return err
		}
	}
	return nil
}

func (bp BackupProgress) hideHost(host string) bool {
	if bp.hostFilter.Size() > 0 {
		return bp.hostFilter.FirstMatch(host) == -1
	}
	return false
}

func (bp BackupProgress) hideKeyspace(keyspace string) bool {
	if bp.keyspaceFilter.Size() > 0 {
		return bp.keyspaceFilter.FirstMatch(keyspace) == -1
	}
	return false
}

var backupProgressTemplate = `{{ with .Run -}}
Run:		{{ .ID }}
Status:		{{ status }}
{{- if .Cause }}
Cause:		{{ FormatError .Cause }}

{{- end }}
{{- if not (isZero .StartTime) }}
Start time:	{{ FormatTime .StartTime }}
{{- end -}}
{{- if not (isZero .EndTime) }}
End time:	{{ FormatTime .EndTime }}
{{- end }}
Duration:	{{ FormatDuration .StartTime .EndTime }}
{{ end -}}
{{ with .Progress }}Progress:	{{ if ne .Size 0 }}{{ FormatUploadProgress .Size .Uploaded .Skipped .Failed }}{{else}}-{{ end }}
{{- if ne .SnapshotTag "" }}
Snapshot Tag:	{{ .SnapshotTag }}
{{- end }}
{{ if .Dcs -}}
Datacenters:	{{ range .Dcs }}
  - {{ . }}
{{- end }}
{{ end -}}
{{ else }}Progress:	0%
{{ end }}
{{- if .Errors -}}
Errors:	{{ range .Errors }}
  - {{ . }}
{{- end }}
{{ end }}
`

func (bp BackupProgress) addHeader(w io.Writer) error {
	temp := template.Must(template.New("backup_progress").Funcs(template.FuncMap{
		"isZero":               isZero,
		"FormatTime":           FormatTime,
		"FormatDuration":       FormatDuration,
		"FormatError":          FormatError,
		"FormatUploadProgress": FormatUploadProgress,
		"status":               bp.status,
	}).Parse(backupProgressTemplate))
	return temp.Execute(w, bp)
}

// status returns task status with optional backup stage.
func (bp BackupProgress) status() string {
	stage := BackupStageName(bp.Progress.Stage)
	s := bp.Run.Status
	if s != TaskStatusNew && s != TaskStatusDone && stage != "" {
		s += " (" + stage + ")"
	}
	return s
}

// RestoreProgress contains shard progress info.
type RestoreProgress struct {
	*models.TaskRunRestoreProgress
	Task     *Task
	Detailed bool
	Errors   []string

	KeyspaceFilter inexlist.InExList
}

var restoreProgressTemplate = `{{ with .Run -}}
Run:		{{ .ID }}
Status:		{{ status }}
{{- if .Cause }}
Cause:		{{ FormatError .Cause }}

{{- end }}
{{- if not (isZero .StartTime) }}
Start time:	{{ FormatTime .StartTime }}
{{- end -}}
{{- if not (isZero .EndTime) }}
End time:	{{ FormatTime .EndTime }}
{{- end }}
Duration:	{{ FormatDuration .StartTime .EndTime }}
{{ end -}}
{{ with .Progress }}Progress:	{{ if ne .Size 0 }}{{ FormatRestoreProgress .Size .Restored .Downloaded .Failed }}{{else}}-{{ end }}
Snapshot Tag:	{{ .SnapshotTag }}
{{ else }}Progress:	0%
{{ end }}
{{- if .Errors -}}
Errors:	{{ range .Errors }}
  - {{ . }}
{{- end }}
{{ end }}
`

func (rp RestoreProgress) addHeader(w io.Writer) error {
	temp := template.Must(template.New("restore_progress").Funcs(template.FuncMap{
		"isZero":                isZero,
		"FormatTime":            FormatTime,
		"FormatDuration":        FormatDuration,
		"FormatError":           FormatError,
		"FormatRestoreProgress": FormatRestoreProgress,
		"status":                rp.status,
	}).Parse(restoreProgressTemplate))
	return temp.Execute(w, rp)
}

// status returns task status with optional restore stage.
func (rp RestoreProgress) status() string {
	s := rp.Run.Status
	if rp.Progress == nil {
		return s
	}
	if s == TaskStatusDone {
		if len(rp.Progress.Keyspaces) == 1 && rp.Progress.Keyspaces[0].Keyspace == "system_schema" {
			return "DONE - restart required (see restore docs)"
		}
	}
	stage := RestoreStageName(rp.Progress.Stage)
	if s != TaskStatusNew && s != TaskStatusDone && stage != "" {
		s += " (" + stage + ")"
	}
	return s
}

func (rp RestoreProgress) hideKeyspace(keyspace string) bool {
	if rp.KeyspaceFilter.Size() > 0 {
		return rp.KeyspaceFilter.FirstMatch(keyspace) == -1
	}
	return false
}

// Render renders *RestoreProgress in a tabular format.
func (rp RestoreProgress) Render(w io.Writer) error {
	if rp.Progress != nil {
		fmt.Fprintf(w, "Restore progress\n")
	}
	if err := rp.addHeader(w); err != nil {
		return err
	}
	if rp.Progress == nil {
		return nil
	}

	if rp.Progress.Size > 0 {
		t := table.New()
		rp.addKeyspaceProgress(t)
		if _, err := io.WriteString(w, t.String()); err != nil {
			return err
		}
	}

	if rp.Detailed && rp.Progress.Size > 0 {
		if err := rp.addTableProgress(w); err != nil {
			return err
		}
	}

	// Check if there is repair progress to display
	if rp.Progress.RepairProgress != nil {
		fmt.Fprintf(w, "\nPost-restore repair progress\n")

		repairRunPr := &models.TaskRunRepairProgress{
			Progress: rp.Progress.RepairProgress,
			Run:      rp.Run,
		}
		repairPr := RepairProgress{
			TaskRunRepairProgress: repairRunPr,
			Task:                  rp.Task,
			Detailed:              rp.Detailed,
			keyspaceFilter:        rp.KeyspaceFilter,
		}

		if err := repairPr.Render(w); err != nil {
			return err
		}
	}

	rp.addViewProgress(w)
	return nil
}

func (rp RestoreProgress) addKeyspaceProgress(t *table.Table) {
	t.AddRow("Keyspace", "Progress", "Size", "Success", "Downloaded", "Failed")
	t.AddSeparator()
	for _, ks := range rp.Progress.Keyspaces {
		if rp.hideKeyspace(ks.Keyspace) {
			continue
		}
		p := "-"
		if len(ks.Tables) > 0 {
			p = FormatRestoreProgress(ks.Size, ks.Restored, ks.Downloaded, ks.Failed)
		}
		t.AddRow(ks.Keyspace, p,
			FormatSizeSuffix(ks.Size),
			FormatSizeSuffix(ks.Restored),
			FormatSizeSuffix(ks.Downloaded),
			FormatSizeSuffix(ks.Failed),
		)
	}
	t.SetColumnAlignment(termtables.AlignRight, 1, 2, 3, 4, 5)
}

func (rp RestoreProgress) addTableProgress(w io.Writer) error {
	for _, ks := range rp.Progress.Keyspaces {
		if rp.hideKeyspace(ks.Keyspace) {
			continue
		}
		fmt.Fprintf(w, "\nKeyspace: %s\n", ks.Keyspace)

		t := table.New("Table", "Progress", "Size", "Success", "Downloaded", "Failed", "Started at", "Completed at", "tombstone_gc mode")
		for i, tab := range ks.Tables {
			if i > 0 {
				t.AddSeparator()
			}

			startedAt := strfmt.DateTime{}
			completedAt := strfmt.DateTime{}
			if tab.StartedAt != nil {
				startedAt = *tab.StartedAt
			}
			if tab.CompletedAt != nil {
				completedAt = *tab.CompletedAt
			}
			t.AddRow(
				tab.Table,
				FormatRestoreProgress(tab.Size,
					tab.Restored,
					tab.Downloaded,
					tab.Failed),
				FormatSizeSuffix(tab.Size),
				FormatSizeSuffix(tab.Restored),
				FormatSizeSuffix(tab.Downloaded),
				FormatSizeSuffix(tab.Failed),
				FormatTime(startedAt),
				FormatTime(completedAt),
				tab.TombstoneGc,
			)
		}
		t.SetColumnAlignment(termtables.AlignRight, 1, 2, 3, 4, 5)
		if _, err := w.Write([]byte(t.String())); err != nil {
			return err
		}
	}
	return nil
}

func (rp RestoreProgress) addViewProgress(w io.Writer) {
	if len(rp.Progress.Views) == 0 {
		return
	}

	_, _ = fmt.Fprintf(w, "\nRestored views\n")
	for _, v := range rp.Progress.Views {
		if rp.Detailed {
			_, _ = fmt.Fprintf(w, "View: %s.%s, Status: %s, Schema definition:\n  %s\n", v.Keyspace, v.View, v.Status, v.CreateStmt)
		} else {
			_, _ = fmt.Fprintf(w, "View: %s.%s, Status: %s\n", v.Keyspace, v.View, v.Status)
		}
	}
}

// ValidateBackupProgress prints validate_backup task progress.
type ValidateBackupProgress struct {
	*models.TaskRunValidateBackupProgress
	Task     *Task
	Detailed bool

	hostFilter inexlist.InExList
}

// SetHostFilter adds filtering rules used for rendering for host details.
func (p *ValidateBackupProgress) SetHostFilter(filters []string) (err error) {
	p.hostFilter, err = inexlist.ParseInExList(filters)
	return
}

func (p ValidateBackupProgress) hideHost(host string) bool {
	if p.hostFilter.Size() > 0 {
		return p.hostFilter.FirstMatch(host) == -1
	}
	return false
}

// Render implements Renderer interface.
func (p ValidateBackupProgress) Render(w io.Writer) error {
	if err := p.addHeader(w); err != nil {
		return err
	}
	if p.Detailed {
		if err := p.addHostProgress(w); err != nil {
			return err
		}
	}

	return nil
}

var validateBackupProgressTemplate = `{{ with .Run -}}
Run:		{{ .ID }}
Status:		{{ .Status }}
{{- if .Cause }}
Cause:		{{ FormatError .Cause }}

{{- end }}
{{- if not (isZero .StartTime) }}
Start time:	{{ FormatTime .StartTime }}
{{- end -}}
{{- if not (isZero .EndTime) }}
End time:	{{ FormatTime .EndTime }}
{{- end }}
Duration:	{{ FormatDuration .StartTime .EndTime }}
{{ end }}
{{ with progress -}}
Scanned files:	{{ .ScannedFiles }}
Missing files:	{{ .MissingFiles }}
Orphaned files:	{{ .OrphanedFiles }} {{ if gt .OrphanedFiles 0 }}({{ FormatSizeSuffix .OrphanedBytes }}){{ end }}
{{- if gt .DeletedFiles 0 }}
Deleted files:	{{ .DeletedFiles }}
{{- end }}
{{- if .BrokenSnapshots }}

Broken snapshots:	{{ range .BrokenSnapshots }}
  - {{ . }}
{{- end }}
{{- end }}
{{- end }}
`

func (p ValidateBackupProgress) addHeader(w io.Writer) error {
	temp := template.Must(template.New("validate_backup_progress").Funcs(template.FuncMap{
		"isZero":               isZero,
		"FormatTime":           FormatTime,
		"FormatDuration":       FormatDuration,
		"FormatError":          FormatError,
		"FormatUploadProgress": FormatUploadProgress,
		"FormatSizeSuffix":     FormatSizeSuffix,
		"progress":             p.aggregatedProgress,
	}).Parse(validateBackupProgressTemplate))
	return temp.Execute(w, p)
}

func (p ValidateBackupProgress) aggregatedProgress() models.ValidateBackupProgress {
	var a models.ValidateBackupProgress

	bs := strset.New()
	for _, i := range p.Progress {
		a.Manifests += i.Manifests
		a.ScannedFiles += i.ScannedFiles
		bs.Add(a.BrokenSnapshots...)
		a.MissingFiles += i.MissingFiles
		a.OrphanedFiles += i.OrphanedFiles
		a.OrphanedBytes += i.OrphanedBytes
		a.DeletedFiles += i.DeletedFiles
	}
	a.BrokenSnapshots = bs.List()
	sort.Strings(a.BrokenSnapshots)

	return a
}

func (p ValidateBackupProgress) addHostProgress(w io.Writer) error {
	t := table.New(
		"Host",
		"Manifests",
		"Scanned files",
		"Missing files",
		"Orphaned files",
		"Orphaned bytes",
		"Deleted files",
	)
	t.SetColumnAlignment(termtables.AlignRight, 1, 2, 3, 4, 5, 6, 7)
	lastLocation := ""

	fmt.Fprintln(w)
	for _, hp := range p.Progress {
		if hp.Location != lastLocation {
			if t.Size() > 0 {
				fmt.Fprintf(w, "Location: %s\n%s\n", lastLocation, t)
			}
			t.Reset()
			lastLocation = hp.Location
		}
		if p.hideHost(hp.Host) {
			continue
		}
		t.AddRow(
			hp.Host,
			hp.Manifests,
			hp.ScannedFiles,
			hp.MissingFiles,
			hp.OrphanedFiles,
			FormatSizeSuffix(hp.OrphanedBytes),
			hp.DeletedFiles,
		)
	}
	if t.Size() > 0 {
		fmt.Fprintf(w, "Location: %s\n%s\n", lastLocation, t)
	}

	return nil
}

// BackupListItems is a []backup.ListItem representation.
type BackupListItems struct {
	items       []*models.BackupListItem
	AllClusters bool
	ShowTables  int
}

const backupListItemTemplate = `backup/{{ .TaskID }}
Snapshots:
{{- range .SnapshotInfo }}
  - {{ .SnapshotTag }} ({{ if eq .Size 0 }}n/a{{ else }}{{ FormatSizeSuffix .Size }}{{ end }}, {{ .Nodes }} nodes)
{{- end }}
Keyspaces:
{{- range .Units }}
  - {{ .Keyspace }} {{ FormatTables .Tables .AllTables }}
{{- end }}

`

// Render implements Renderer interface.
func (bl BackupListItems) Render(w io.Writer) error {
	temp := template.Must(template.New("backup_list_items").Funcs(template.FuncMap{
		"FormatTables": func(tables []string, all bool) string {
			return FormatTables(bl.ShowTables, tables, all)
		},
		"FormatSizeSuffix": FormatSizeSuffix,
	}).Parse(backupListItemTemplate))

	prev := ""
	for _, i := range bl.items {
		if bl.AllClusters {
			if prev != i.ClusterID {
				prev = i.ClusterID
				fmt.Fprintln(w, "Cluster:", i.ClusterID)
				fmt.Fprintln(w)
			}
		}
		if err := temp.Execute(w, i); err != nil {
			return err
		}
	}
	return nil
}
