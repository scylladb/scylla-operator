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

// Package critic reviews the reconcile traces recorded by package reconciletrace and reports where a controller did
// more, or later, than it had to.
//
// The review has a deterministic part and an optional model part. The deterministic part splits a trace into
// episodes, computes generic signals (reconcile errors, no-op writes, conflicts, reconciles triggered by the
// controller's own writes that wrote nothing, objects written over and over) and renders a compact timeline. The
// model part sends the signals, the timeline and the caller's domain context to a Model and keeps the findings whose
// evidence cites records shown in the timeline.
//
// The package defines no model client. The caller implements Model for the API or tool of its choice, and adds what
// only it knows about its controller through ContextProviders.
package critic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Prompt is what a Model is asked.
type Prompt struct {
	System string
	User   string
}

// Model answers a prompt with text. Implementations wrap an LLM API or tool; the critic depends on nothing else.
type Model interface {
	Complete(ctx context.Context, p Prompt) (string, error)
}

// ModelFunc adapts a function to a Model.
type ModelFunc func(ctx context.Context, p Prompt) (string, error)

// Complete implements Model.
func (f ModelFunc) Complete(ctx context.Context, p Prompt) (string, error) {
	return f(ctx, p)
}

// Section is a titled block of context added to the prompt.
type Section struct {
	Title string
	Body  string
}

// ContextProvider adds domain context about an episode to the prompt: the controller's conventions the reviewer
// must not flag, what counts as necessary disruption, where the code lives, the state of the objects involved.
type ContextProvider interface {
	Context(ctx context.Context, e *Episode) ([]Section, error)
}

// ContextProviderFunc adapts a function to a ContextProvider.
type ContextProviderFunc func(ctx context.Context, e *Episode) ([]Section, error)

// Context implements ContextProvider.
func (f ContextProviderFunc) Context(ctx context.Context, e *Episode) ([]Section, error) {
	return f(ctx, e)
}

// StaticContext returns a ContextProvider adding the same sections to every episode.
func StaticContext(sections ...Section) ContextProvider {
	return ContextProviderFunc(func(context.Context, *Episode) ([]Section, error) {
		return sections, nil
	})
}

// Options configure a Critic.
type Options struct {
	// Model reviews the episodes. Without one, Review reports the signals only.
	Model Model
	// ContextProviders add domain context to every prompt, in order.
	ContextProviders []ContextProvider
	// FilterPrompt, if set, is applied to every prompt before it is sent, e.g. to redact names.
	FilterPrompt func(Prompt) Prompt
	// Render tunes the timeline.
	Render RenderOptions
	// Timeout bounds one model call. Defaults to 10 minutes.
	Timeout time.Duration
}

// Critic reviews episodes.
type Critic struct {
	opts Options
}

// New creates a Critic.
func New(opts Options) *Critic {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}

	return &Critic{opts: opts}
}

// Finding is one thing the model found the controller could do better.
type Finding struct {
	Title string `json:"title"`
	// Category is e.g. unnecessary-disruption, write-churn, latency, ordering, error-on-happy-path or status-quality.
	Category string `json:"category"`
	// Severity is low, medium or high.
	Severity string `json:"severity"`
	// Kind is observation, for what the trace shows, or hypothesis, for an explanation the trace doesn't prove.
	Kind string `json:"kind"`
	// Evidence are the IDs of the timeline records the finding rests on.
	Evidence    []uint64 `json:"evidence"`
	Explanation string   `json:"explanation"`
	Suggestion  string   `json:"suggestion,omitempty"`
	// CodeReferences point at the code the finding is about, as path:line, when the model could tell.
	CodeReferences []string `json:"codeReferences,omitempty"`
}

// Report is the review of one episode.
type Report struct {
	Controller string
	Request    string
	Signals    Signals
	Timeline   Timeline
	// Findings are the model's findings whose evidence the timeline shows.
	Findings []Finding
	// Rejected are the model's findings citing no record of the timeline.
	Rejected []Finding
	// Prompt is what a model is, or would be, asked about the episode.
	Prompt *Prompt
	// Reviewed tells a report a model answered; Response is the answer.
	Reviewed bool
	Response string
}

// Prepare computes the signals and the timeline of e and builds the prompt, without asking the model. Callers that
// cache answers key them by the prompt and pass them to SetResponse.
func (c *Critic) Prepare(ctx context.Context, e *Episode) (*Report, error) {
	report := &Report{
		Controller: e.Controller,
		Request:    e.Request,
		Signals:    ComputeSignals(e),
		Timeline:   RenderTimeline(e, c.opts.Render),
	}

	var sections []Section
	for _, p := range c.opts.ContextProviders {
		s, err := p.Context(ctx, e)
		if err != nil {
			return report, fmt.Errorf("can't get context: %w", err)
		}
		sections = append(sections, s...)
	}

	prompt := BuildPrompt(report.Signals, report.Timeline, sections)
	if c.opts.FilterPrompt != nil {
		prompt = c.opts.FilterPrompt(prompt)
	}
	report.Prompt = &prompt

	return report, nil
}

// Ask asks the critic's model about a prepared report and records the answer.
func (c *Critic) Ask(ctx context.Context, report *Report) error {
	if c.opts.Model == nil {
		return errors.New("no model configured")
	}

	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	response, err := c.opts.Model.Complete(ctx, *report.Prompt)
	if err != nil {
		return fmt.Errorf("model failed: %w", err)
	}

	return report.SetResponse(response)
}

// SetResponse records a model's answer to the report's prompt, keeping the findings whose evidence the timeline
// shows.
func (r *Report) SetResponse(response string) error {
	r.Reviewed = true
	r.Response = response

	findings, err := ParseFindings(response)
	if err != nil {
		return err
	}
	r.Findings, r.Rejected = checkEvidence(findings, r.Timeline)

	return nil
}

// Review reviews e. Without a model it returns the signals and the timeline.
func (c *Critic) Review(ctx context.Context, e *Episode) (*Report, error) {
	report, err := c.Prepare(ctx, e)
	if err != nil || c.opts.Model == nil {
		return report, err
	}

	return report, c.Ask(ctx, report)
}

// checkEvidence keeps the findings citing at least one record the timeline shows, with only such records.
func checkEvidence(findings []Finding, t Timeline) (kept, rejected []Finding) {
	for _, f := range findings {
		var evidence []uint64
		for _, id := range f.Evidence {
			if _, ok := t.Lines[id]; ok {
				evidence = append(evidence, id)
			}
		}
		if len(evidence) == 0 {
			rejected = append(rejected, f)
			continue
		}
		f.Evidence = evidence
		kept = append(kept, f)
	}

	return kept, rejected
}

const systemPrompt = `You review the behaviour of a Kubernetes controller from a trace of one episode of its work on one object: what enqueued it, what each reconcile read and wrote, and how each reconcile ended.

Look for places where the controller did more, or later, than it had to:
- unnecessary-disruption: restarts, deletions, rollouts or regenerated secrets the change didn't call for.
- write-churn: writes that change nothing, the same object rewritten reconcile after reconcile, conflicts, status flapping.
- latency: waiting after a precondition was already met, fixed requeue intervals where an event would do, long queue waits.
- ordering: steps that fail or wait because an earlier step of the same reconcile isn't visible yet, or work that could run in parallel.
- error-on-happy-path: reconcile errors or retries in a scenario where nothing went wrong.
- status-quality: conditions or status that lag, flap or mislead.

Rules:
- Ground every finding in the trace. Cite the records it rests on by their numeric IDs (the #N at the start of the timeline lines and in the trigger lists). Findings without valid IDs are discarded.
- Mark each finding as an "observation" when the trace shows it, or a "hypothesis" when it explains or extrapolates beyond the trace.
- The trace comes from a test or a live cluster where other actors also act, e.g. a test driver marking objects ready. Writes not made by the controller are not the controller's fault.
- Respect the controller's conventions given in the domain context; don't report them as problems.
- If you can read the controller's source code, use it to name the code responsible, as path:line.
- Fewer well-evidenced findings beat many weak ones. An empty list is a valid answer.

Answer with a single JSON object and nothing else:
{"findings": [{"title": "...", "category": "...", "severity": "low|medium|high", "kind": "observation|hypothesis", "evidence": [123, 456], "explanation": "...", "suggestion": "...", "codeReferences": ["path/to/file.go:42"]}]}`

// BuildPrompt builds the prompt for an episode from its signals, its timeline and the domain context.
func BuildPrompt(s Signals, t Timeline, sections []Section) Prompt {
	var b strings.Builder

	for _, sec := range sections {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", sec.Title, strings.TrimSpace(sec.Body))
	}

	b.WriteString("## Signals computed from the trace\n\n")
	b.WriteString(s.Summary())
	b.WriteString("\n## Timeline\n\n")
	b.WriteString(t.Text)

	return Prompt{
		System: systemPrompt,
		User:   b.String(),
	}
}

// ParseFindings parses the model's answer. It tolerates text or a code fence around the JSON object.
func ParseFindings(response string) ([]Finding, error) {
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start < 0 || end < start {
		return nil, errors.New("no JSON object in the model's answer")
	}

	var answer struct {
		Findings []Finding `json:"findings"`
	}
	err := json.Unmarshal([]byte(response[start:end+1]), &answer)
	if err != nil {
		return nil, fmt.Errorf("can't parse the model's answer: %w", err)
	}

	return answer.Findings, nil
}

// Markdown renders the report, quoting the timeline lines of each finding's evidence.
func (r *Report) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Reconcile review: %s %s\n\n", r.Controller, r.Request)
	b.WriteString("## Signals\n\n")
	b.WriteString(r.Signals.Summary())

	if !r.Reviewed {
		b.WriteString("\nNo model configured; signals only.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "\n## Findings (%d)\n\n", len(r.Findings))
	if len(r.Findings) == 0 {
		b.WriteString("None.\n")
	}
	for i, f := range r.Findings {
		fmt.Fprintf(&b, "### %d. %s\n\n", i+1, f.Title)
		fmt.Fprintf(&b, "%s, %s, %s\n\n", f.Category, f.Severity, f.Kind)
		fmt.Fprintf(&b, "%s\n\n", f.Explanation)
		if f.Suggestion != "" {
			fmt.Fprintf(&b, "Suggestion: %s\n\n", f.Suggestion)
		}
		if len(f.CodeReferences) > 0 {
			fmt.Fprintf(&b, "Code: %s\n\n", strings.Join(f.CodeReferences, ", "))
		}
		b.WriteString("Evidence:\n\n```\n")
		ids := slices.Clone(f.Evidence)
		slices.Sort(ids)
		for _, id := range ids {
			b.WriteString(r.Timeline.Lines[id])
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	if len(r.Rejected) > 0 {
		fmt.Fprintf(&b, "## Rejected findings (%d, evidence not in the timeline)\n\n", len(r.Rejected))
		for _, f := range r.Rejected {
			fmt.Fprintf(&b, "- %s (cited %v)\n", f.Title, f.Evidence)
		}
	}

	return b.String()
}
