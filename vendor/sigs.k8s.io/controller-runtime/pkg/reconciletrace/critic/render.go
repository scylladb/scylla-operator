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
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
)

// RenderOptions tune the timeline.
type RenderOptions struct {
	// MaxDiffLines caps the changed fields shown for one write. Defaults to 20.
	MaxDiffLines int
	// MaxValueLen caps a value shown in a diff. Defaults to 160.
	MaxValueLen int
	// CollapseIdleRuns folds runs of at least this many consecutive reconciles that wrote nothing, failed nothing
	// and errored not into one line. Defaults to 3; a negative value never folds.
	CollapseIdleRuns int
	// MaxBytes caps the timeline; a longer one loses lines from its middle. Defaults to 200000; a negative value
	// never caps.
	MaxBytes int
}

func (o RenderOptions) withDefaults() RenderOptions {
	if o.MaxDiffLines <= 0 {
		o.MaxDiffLines = 20
	}
	if o.MaxValueLen <= 0 {
		o.MaxValueLen = 160
	}
	if o.CollapseIdleRuns == 0 {
		o.CollapseIdleRuns = 3
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 200_000
	}

	return o
}

// idleRun is a run of consecutive reconciles that did nothing visible, rendered as one line.
type idleRun struct {
	reconcileIDs []string
	start, end   time.Time
	triggers     []uint64
	clientCalls  int
}

// findIdleRuns returns the runs of at least minLen consecutive reconciles that wrote nothing, had no failed read
// and returned no error, keyed by each member's reconcile ID.
func findIdleRuns(e *Episode, minLen int) map[string]*idleRun {
	if minLen < 1 {
		return nil
	}

	var order []string
	busy := map[string]bool{}
	starts := map[string]reconciletrace.Record{}
	ends := map[string]reconciletrace.Record{}
	for _, r := range e.Records {
		switch r.Type {
		case reconciletrace.RecordTypeReconcileStart:
			order = append(order, r.ReconcileID)
			starts[r.ReconcileID] = r
		case reconciletrace.RecordTypeReconcileEnd:
			ends[r.ReconcileID] = r
			if r.ReconcileEnd.Error != "" {
				busy[r.ReconcileID] = true
			}
		case reconciletrace.RecordTypeClientCall:
			c := r.ClientCall
			if isWrite(c) || (c.Error != "" && c.ErrorReason != "NotFound") {
				busy[r.ReconcileID] = true
			}
		}
	}

	runs := map[string]*idleRun{}
	var current []string
	closeRun := func() {
		if len(current) >= minLen {
			run := &idleRun{reconcileIDs: current}
			for _, id := range current {
				run.triggers = append(run.triggers, starts[id].ReconcileStart.Triggers...)
				if end, ok := ends[id]; ok {
					run.clientCalls += end.ReconcileEnd.ClientCalls
					run.end = end.Time
				}
				runs[id] = run
			}
			run.start = starts[current[0]].Time
		}
		current = nil
	}
	for _, id := range order {
		_, ended := ends[id]
		if busy[id] || !ended {
			closeRun()
			continue
		}
		current = append(current, id)
	}
	closeRun()

	return runs
}

// Timeline is the compact text rendering of an episode a model reviews.
type Timeline struct {
	Text string
	// Lines maps the ID of each record the timeline shows to its line, so that the evidence of a finding can be
	// checked and shown. Records folded into a summary, like most reads, have no line and can't be cited.
	Lines map[uint64]string
}

// ignoredDiffPaths are the fields every write changes or no reviewer needs.
var ignoredDiffPaths = []string{
	"metadata.resourceVersion",
	"metadata.managedFields",
	"metadata.creationTimestamp",
	"metadata.uid",
}

// RenderTimeline renders e: one header line per reconcile with its triggers, queue wait, duration and result, one
// line per write with the fields it changed against the previous write of the object, and the reads folded into
// counts. Every shown line starts with the ID of its record.
func RenderTimeline(e *Episode, opts RenderOptions) Timeline {
	opts = opts.withDefaults()

	t := Timeline{Lines: map[uint64]string{}}
	var b strings.Builder

	start := e.Start()
	ownWrites := map[string]string{}
	enqueues := map[uint64]reconciletrace.Record{}
	lastBody := map[string]map[string]any{}
	reconcileOrdinal := 0
	idleRuns := findIdleRuns(e, opts.CollapseIdleRuns)
	// A folded line describes the triggers of reconciles further down the episode.
	allEnqueues := map[uint64]reconciletrace.Record{}
	for _, r := range e.Records {
		if r.Type == reconciletrace.RecordTypeEnqueue {
			allEnqueues[r.ID] = r
		}
	}

	type readCounts struct {
		counts map[string]int
		errors []string
	}
	var reads *readCounts

	line := func(id uint64, indent, text string) {
		l := fmt.Sprintf("#%d %s", id, text)
		t.Lines[id] = l
		b.WriteString(indent)
		b.WriteString(l)
		b.WriteString("\n")
	}
	flushReads := func() {
		if reads == nil {
			return
		}
		if len(reads.counts) > 0 {
			keys := slices.Sorted(maps.Keys(reads.counts))
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%d %s", reads.counts[k], k))
			}
			fmt.Fprintf(&b, "    reads: %s\n", strings.Join(parts, ", "))
		}
		reads = nil
	}

	fmt.Fprintf(&b, "Episode of controller %q for request %q: %d records over %s.\n", e.Controller, e.Request, len(e.Records), e.End().Sub(start).Round(time.Millisecond))
	b.WriteString("Times are relative to the first record. Reads are folded into counts per reconcile.\n\n")

	for i := range e.Records {
		r := e.Records[i]
		at := fmt.Sprintf("+%.3fs", r.Time.Sub(start).Seconds())

		switch r.Type {
		case reconciletrace.RecordTypeEnqueue:
			enqueues[r.ID] = r
			// Watch events are shown with the reconcile they trigger; requeues and unattributed adds where they
			// happen, unless a folded reconcile requeued.
			if _, folded := idleRuns[r.Enqueue.ByReconcileID]; folded {
				continue
			}
			if r.Enqueue.Cause != reconciletrace.CauseTypeEvent {
				line(r.ID, "", fmt.Sprintf("%s ENQUEUE %s", at, describeEnqueue(r.Enqueue, ownWrites)))
			}

		case reconciletrace.RecordTypeReconcileStart:
			reconcileOrdinal++
			if run, folded := idleRuns[r.ReconcileID]; folded {
				if run.reconcileIDs[0] == r.ReconcileID {
					last := reconcileOrdinal + len(run.reconcileIDs) - 1
					b.WriteString("\n")
					line(r.ID, "", fmt.Sprintf("%s RECONCILES %d-%d (%d reconciles over %s, %d client calls) wrote nothing and returned no error; triggered by: %s",
						at, reconcileOrdinal, last, len(run.reconcileIDs), run.end.Sub(run.start).Round(time.Millisecond), run.clientCalls, describeTriggers(run.triggers, allEnqueues, ownWrites)))
					for _, id := range run.triggers {
						if enq, ok := allEnqueues[id]; ok {
							t.Lines[id] = fmt.Sprintf("#%d trigger of reconciles %d-%d: %s", id, reconcileOrdinal, last, describeEnqueue(enq.Enqueue, ownWrites))
						}
					}
				}
				continue
			}
			rs := r.ReconcileStart
			triggers := describeTriggers(rs.Triggers, enqueues, ownWrites)
			b.WriteString("\n")
			line(r.ID, "", fmt.Sprintf("%s RECONCILE %d (reconcileID %s) queue wait %s, triggered by: %s", at, reconcileOrdinal, shortID(r.ReconcileID), rs.QueueWait.Round(time.Millisecond), triggers))
			for _, id := range rs.Triggers {
				if enq, ok := enqueues[id]; ok {
					t.Lines[id] = fmt.Sprintf("#%d trigger of reconcile %d: %s", id, reconcileOrdinal, describeEnqueue(enq.Enqueue, ownWrites))
				}
			}
			reads = &readCounts{counts: map[string]int{}}

		case reconciletrace.RecordTypeClientCall:
			if _, folded := idleRuns[r.ReconcileID]; folded {
				continue
			}
			c := r.ClientCall
			if !isWrite(c) {
				if reads == nil {
					reads = &readCounts{counts: map[string]int{}}
				}
				if c.Error != "" && c.ErrorReason != "NotFound" {
					line(r.ID, "    ", fmt.Sprintf("%s %s %s %s failed: %s", at, c.Client, c.Verb, describeObject(c.Object, ""), truncate(c.Error, opts.MaxValueLen)))
					continue
				}
				k := fmt.Sprintf("%s %s %s", c.Client, c.Verb, c.Object.Kind)
				if c.ErrorReason == "NotFound" {
					k += " (not found)"
				}
				reads.counts[k]++
				continue
			}

			key := objectKey(c.Object, c.SubResource)
			text := fmt.Sprintf("%s %s %s", at, strings.ToUpper(c.Verb), describeObject(c.Object, c.SubResource))
			switch {
			case c.Error != "":
				text += fmt.Sprintf(" FAILED (%s): %s", c.ErrorReason, truncate(c.Error, opts.MaxValueLen))
			case c.NoOp:
				text += fmt.Sprintf(" NO-OP (resourceVersion stays %s)", c.ResourceVersionAfter)
			default:
				if c.ResourceVersionBefore != "" {
					text += fmt.Sprintf(" rv %s->%s", c.ResourceVersionBefore, c.ResourceVersionAfter)
				} else if c.ResourceVersionAfter != "" {
					text += fmt.Sprintf(" rv ->%s", c.ResourceVersionAfter)
				}
				if c.ResourceVersionAfter != "" {
					ownWrites[c.ResourceVersionAfter] = objectKey(c.Object, "")
				}
			}
			if c.PatchType != "" {
				text += " patch type " + c.PatchType
			}
			line(r.ID, "    ", text)

			if len(c.Body) > 0 && c.Error == "" {
				var body map[string]any
				if err := json.Unmarshal(c.Body, &body); err == nil {
					if prev, ok := lastBody[key]; ok {
						for _, d := range diffLines(prev, body, opts) {
							b.WriteString("        ")
							b.WriteString(d)
							b.WriteString("\n")
						}
					} else if c.Verb != "create" {
						b.WriteString("        (first write of the object in this episode, no baseline to diff against)\n")
					}
					lastBody[key] = body
				}
			}

		case reconciletrace.RecordTypeReconcileEnd:
			if _, folded := idleRuns[r.ReconcileID]; folded {
				continue
			}
			flushReads()
			end := r.ReconcileEnd
			result := "done"
			switch {
			case end.Error != "":
				result = fmt.Sprintf("ERROR (terminal=%v): %s", end.Terminal, truncate(end.Error, 2*opts.MaxValueLen))
			case end.RequeueAfter > 0:
				result = fmt.Sprintf("requeue after %s", end.RequeueAfter)
			case end.Requeue:
				result = "requeue"
			}
			line(r.ID, "", fmt.Sprintf("%s END of reconcile %d after %s, %d client calls: %s", at, reconcileOrdinal, end.Duration.Round(time.Millisecond), end.ClientCalls, result))
		}
	}

	t.Text = b.String()
	if opts.MaxBytes > 0 && len(t.Text) > opts.MaxBytes {
		t = capTimeline(t, opts.MaxBytes)
	}

	return t
}

var citedIDRegexp = regexp.MustCompile(`#(\d+)`)

// capTimeline keeps the head and the tail of a timeline within maxBytes and drops the lines of the records no
// longer shown, so they can't be cited.
func capTimeline(t Timeline, maxBytes int) Timeline {
	lines := strings.SplitAfter(t.Text, "\n")

	headBudget, tailBudget := maxBytes*2/5, maxBytes*3/5
	head, size := 0, 0
	for head < len(lines) && size+len(lines[head]) <= headBudget {
		size += len(lines[head])
		head++
	}
	tail, size := len(lines), 0
	for tail > head && size+len(lines[tail-1]) <= tailBudget {
		size += len(lines[tail-1])
		tail--
	}

	var b strings.Builder
	for _, l := range lines[:head] {
		b.WriteString(l)
	}
	fmt.Fprintf(&b, "\n... %d lines omitted to keep the timeline within %d bytes ...\n\n", tail-head, maxBytes)
	for _, l := range lines[tail:] {
		b.WriteString(l)
	}

	shown := map[uint64]bool{}
	for _, m := range citedIDRegexp.FindAllStringSubmatch(b.String(), -1) {
		id, err := strconv.ParseUint(m[1], 10, 64)
		if err == nil {
			shown[id] = true
		}
	}
	kept := map[uint64]string{}
	for id, l := range t.Lines {
		if shown[id] {
			kept[id] = l
		}
	}

	return Timeline{Text: b.String(), Lines: kept}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}

	return id
}

func describeObject(ref reconciletrace.ObjectRef, subResource string) string {
	s := ref.Kind
	if ref.Name != "" {
		s += " " + ref.Name
	}
	if subResource != "" {
		s += " /" + subResource
	}

	return s
}

func describeEnqueue(enq *reconciletrace.Enqueue, ownWrites map[string]string) string {
	switch enq.Cause {
	case reconciletrace.CauseTypeEvent:
		ev := enq.Event
		s := fmt.Sprintf("%s %s rv %s", ev.Type, describeObject(ev.Object, ""), ev.Object.ResourceVersion)
		var notes []string
		if isOwnEvent(ev, ownWrites) {
			notes = append(notes, "own write")
		}
		if ev.GenerationChanged {
			notes = append(notes, "spec changed")
		}
		if ev.Resync {
			notes = append(notes, "resync")
		}
		if ev.InInitialList {
			notes = append(notes, "initial list")
		}
		if len(notes) > 0 {
			s += " (" + strings.Join(notes, ", ") + ")"
		}
		return s
	case reconciletrace.CauseTypeRequeueAfter:
		return fmt.Sprintf("requeue after %s by reconcile %s", enq.After, shortID(enq.ByReconcileID))
	default:
		if enq.ByReconcileID != "" {
			return fmt.Sprintf("%s by reconcile %s", enq.Cause, shortID(enq.ByReconcileID))
		}
		return string(enq.Cause)
	}
}

// describeTriggers summarizes the triggers of a reconcile, grouping the events by type and kind.
func describeTriggers(ids []uint64, enqueues map[uint64]reconciletrace.Record, ownWrites map[string]string) string {
	if len(ids) == 0 {
		return "nothing recorded"
	}

	type group struct {
		ids   []uint64
		own   int
		names map[string]bool
	}
	var order []string
	groups := map[string]*group{}
	for _, id := range ids {
		enq, ok := enqueues[id]
		if !ok {
			continue
		}
		var k, name string
		own := false
		if ev := enq.Enqueue.Event; ev != nil {
			k = ev.Type + " " + ev.Object.Kind
			name = ev.Object.Name
			own = isOwnEvent(ev, ownWrites)
		} else {
			k = string(enq.Enqueue.Cause)
		}
		g, ok := groups[k]
		if !ok {
			g = &group{names: map[string]bool{}}
			groups[k] = g
			order = append(order, k)
		}
		g.ids = append(g.ids, id)
		if own {
			g.own++
		}
		if name != "" {
			g.names[name] = true
		}
	}

	parts := make([]string, 0, len(order))
	for _, k := range order {
		g := groups[k]
		s := fmt.Sprintf("%d x %s", len(g.ids), k)
		if len(g.names) > 0 && len(g.names) <= 3 {
			s += " " + strings.Join(slices.Sorted(maps.Keys(g.names)), ", ")
		}
		if g.own > 0 {
			s += fmt.Sprintf(" (%d own writes)", g.own)
		}
		s += " " + formatIDs(g.ids)
		parts = append(parts, s)
	}

	return strings.Join(parts, "; ")
}

// diffLines lists the leaf fields that differ between two versions of an object.
func diffLines(prev, cur map[string]any, opts RenderOptions) []string {
	a, b := map[string]string{}, map[string]string{}
	flatten("", prev, a)
	flatten("", cur, b)

	paths := map[string]bool{}
	for p := range a {
		paths[p] = true
	}
	for p := range b {
		paths[p] = true
	}

	var lines []string
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		if ignored(p) || a[p] == b[p] {
			continue
		}
		old, okOld := a[p]
		cur, okCur := b[p]
		switch {
		case !okOld:
			lines = append(lines, fmt.Sprintf("+ %s: %s", p, truncate(cur, opts.MaxValueLen)))
		case !okCur:
			lines = append(lines, fmt.Sprintf("- %s: %s", p, truncate(old, opts.MaxValueLen)))
		default:
			lines = append(lines, fmt.Sprintf("~ %s: %s -> %s", p, truncate(old, opts.MaxValueLen), truncate(cur, opts.MaxValueLen)))
		}
	}

	if len(lines) == 0 {
		return []string{"(no field changed besides the resource version)"}
	}
	if len(lines) > opts.MaxDiffLines {
		more := len(lines) - opts.MaxDiffLines
		lines = append(lines[:opts.MaxDiffLines], fmt.Sprintf("... %d more changed fields", more))
	}

	return lines
}

func ignored(path string) bool {
	for _, p := range ignoredDiffPaths {
		if path == p || strings.HasPrefix(path, p+".") || strings.HasPrefix(path, p+"[") {
			return true
		}
	}

	return false
}

// flatten maps every leaf of v to its path. Lists of objects with a name or type are keyed by it, so that a reordered
// list of conditions doesn't show as a change of every element.
func flatten(prefix string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			flatten(join(prefix, k), child, out)
		}
	case []any:
		for i, child := range t {
			key := fmt.Sprintf("[%d]", i)
			if m, ok := child.(map[string]any); ok {
				for _, field := range []string{"name", "type"} {
					if s, ok := m[field].(string); ok {
						key = fmt.Sprintf("[%s=%s]", field, s)
						break
					}
				}
			}
			flatten(prefix+key, child, out)
		}
	default:
		b, _ := json.Marshal(t)
		out[prefix] = string(b)
	}
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}

	return prefix + "." + key
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}
