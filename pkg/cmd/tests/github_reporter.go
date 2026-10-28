// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
)

// lineTrackingWriter remembers whether the last byte written through it ended a line, so that GitHub Actions
// workflow commands, which are only recognized at the start of a line, can be emitted after arbitrary output.
// It's safe for concurrent use, as the logs are written through it too.
type lineTrackingWriter struct {
	lock        sync.Mutex
	w           io.Writer
	atLineStart bool
}

var _ io.Writer = &lineTrackingWriter{}

func newLineTrackingWriter(w io.Writer) *lineTrackingWriter {
	return &lineTrackingWriter{
		w:           w,
		atLineStart: true,
	}
}

func (w *lineTrackingWriter) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	n, err := w.w.Write(p)
	if n > 0 {
		w.atLineStart = p[n-1] == '\n'
	}
	return n, err
}

// githubReporter decorates a Ginkgo reporter for GitHub Actions logs. Each finished spec is folded into its own log
// group, titled with its state, duration, name and parallel process, so a run reads as a list of specs rather than a
// wall of text. Specs are also announced as they start, so that the ones in flight can be told apart.
// The decorated reporter is set once the reporter config is known. It must write through the same lineTrackingWriter
// and must not emit log groups itself, as GitHub Actions doesn't support nesting them.
type githubReporter struct {
	reporters.Reporter

	// lock serializes the output, as the start of specs is reported independently of Ginkgo's reporting.
	lock sync.Mutex

	out       *lineTrackingWriter
	groupOpen bool
	// suiteBegan tells whether the suite banner was emitted. Ginkgo holds back the reports of finished specs until
	// all the parallel processes begin the suite, but the start of specs comes through regardless, so it's held back
	// in pendingSpecStarts until then.
	suiteBegan        bool
	pendingSpecStarts []specStartedEvent
}

var _ reporters.Reporter = &githubReporter{}

func newGithubReporter(out *lineTrackingWriter) *githubReporter {
	return &githubReporter{
		out: out,
	}
}

func (r *githubReporter) command(format string, args ...any) {
	r.out.lock.Lock()
	atLineStart := r.out.atLineStart
	r.out.lock.Unlock()
	if !atLineStart {
		_, _ = fmt.Fprintln(r.out)
	}
	_, _ = fmt.Fprintf(r.out, format+"\n", args...)
}

func (r *githubReporter) startGroup(title string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.beginGroup(title)
}

func (r *githubReporter) endGroup() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.finishGroup()
}

func (r *githubReporter) beginGroup(title string) {
	// Groups don't nest, so a new one has to close the previous one first.
	r.finishGroup()
	// The title is kept as it is, as its columns are padded with spaces.
	r.command("::group::%s", strings.NewReplacer("\r", " ", "\n", " ").Replace(title))
	r.groupOpen = true
}

func (r *githubReporter) finishGroup() {
	if !r.groupOpen {
		return
	}
	r.command("::endgroup::")
	r.groupOpen = false
}

// outputDestination returns the writer for the parallel server's output channel, which carries the events about specs
// starting in the parallel processes. Anything else that comes through it is passed on as it is.
func (r *githubReporter) outputDestination() io.Writer {
	return newSpecStartedReceiver(r.specStarted, r.writeOutput)
}

func (r *githubReporter) specStarted(event specStartedEvent) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.suiteBegan {
		r.pendingSpecStarts = append(r.pendingSpecStarts, event)
		return
	}

	r.writeSpecStarted(event)
}

func (r *githubReporter) writeOutput(p []byte) (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.out.Write(p)
}

// writeSpecStarted announces a spec that started. It's a group, like the spec's result, so that the two line up in
// the log, and it holds the details of the spec that its title has no room for.
func (r *githubReporter) writeSpecStarted(event specStartedEvent) {
	r.beginGroup(logEntry("🏃", event.Process, "started", oneLine(event.Text)))
	if len(event.Location) != 0 {
		r.command("%s", event.Location)
	}
	if len(event.Labels) != 0 {
		r.command("Labels: %s", strings.Join(event.Labels, ", "))
	}
	r.finishGroup()
}

func (r *githubReporter) SuiteWillBegin(report types.Report) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Closes the group with the setup output, if any.
	r.finishGroup()
	r.Reporter.SuiteWillBegin(report)

	r.suiteBegan = true
	for _, event := range r.pendingSpecStarts {
		r.writeSpecStarted(event)
	}
	r.pendingSpecStarts = nil
}

func (r *githubReporter) DidRun(report types.SpecReport) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if report.State.Is(types.SpecStateSkipped) && report.Failure.IsZero() {
		r.Reporter.DidRun(report)
		return
	}

	r.beginGroup(specTitle(report))
	r.Reporter.DidRun(report)
	r.finishGroup()
}

func (r *githubReporter) EmitProgressReport(report types.ProgressReport) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.beginGroup(progressReportTitle(report))
	r.Reporter.EmitProgressReport(report)
	r.finishGroup()
}

func (r *githubReporter) SuiteDidEnd(report types.Report) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.finishGroup()
	r.Reporter.SuiteDidEnd(report)

	// Folds the teardown output, mostly the logs of the parallel processes, until the run ends.
	r.beginGroup("Test suite teardown")
}

func specText(report types.SpecReport) string {
	if report.LeafNodeType.Is(types.NodeTypesForSuiteLevelNodes) {
		return fmt.Sprintf("[%s]", report.LeafNodeType)
	}
	return report.FullText()
}

func specStateIcon(report types.SpecReport) string {
	switch {
	case report.State.Is(types.SpecStatePassed) && report.NumAttempts > 1 && report.MaxFlakeAttempts > 1:
		return "🔁"
	case report.State.Is(types.SpecStatePassed):
		return "✅"
	case report.State.Is(types.SpecStateTimedout | types.SpecStateInterrupted):
		return "⏱️"
	case report.State.Is(types.SpecStateFailureStates):
		return "❌"
	case report.State.Is(types.SpecStatePending):
		return "⏸️"
	default:
		return "⏭️"
	}
}

// logEntry formats a log line about a spec in columns: an icon, the parallel process, a duration or other short
// status, and the text, so that the starts, the progress and the results of specs line up with each other.
func logEntry(icon string, process int, status string, text string) string {
	processColumn := ""
	if process > 0 {
		processColumn = fmt.Sprintf("%-6s", fmt.Sprintf("[#%d]", process))
	}
	return fmt.Sprintf("%s %s%7s  %s", icon, processColumn, status, text)
}

func specTitle(report types.SpecReport) string {
	text := oneLine(specText(report))
	if !report.State.Is(types.SpecStatePassed) {
		text = fmt.Sprintf("%s [%s]", text, strings.ToUpper(report.State.String()))
	}
	return logEntry(specStateIcon(report), report.ParallelProcess, formatDuration(report.RunTime), text)
}

func progressReportTitle(report types.ProgressReport) string {
	subject := strings.Join(append(slices.Clone(report.ContainerHierarchyTexts), report.LeafNodeText), " ")
	if len(strings.TrimSpace(subject)) == 0 {
		subject = report.Message
	}

	process := 0
	if report.RunningInParallel {
		process = report.ParallelProcess
	}

	elapsed := ""
	if !report.SpecStartTime.IsZero() && !report.Time().IsZero() {
		elapsed = formatDuration(report.Time().Sub(report.SpecStartTime))
	}

	text := fmt.Sprintf("%s (still running)", oneLine(subject))
	if len(report.CurrentStepText) != 0 {
		text = fmt.Sprintf("%s, at step %q", text, report.CurrentStepText)
	}

	return logEntry("⏳", process, elapsed, text)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
