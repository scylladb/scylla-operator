// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"cmp"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/scylladb/scylla-operator/pkg/thirdparty/github.com/onsi/ginkgo/v2/exposedinternal/parallel_support"
	"k8s.io/klog/v2"
)

const (
	// githubAnnotationMessageMaxLength caps the message of an error annotation. GitHub truncates long annotations
	// anyway, and the full failure stays in the spec's log group and in the summary.
	githubAnnotationMessageMaxLength = 4 * 1024
	// githubSummaryFailureMessageMaxLength caps a single failure message in the job summary, which GitHub limits to
	// 1MiB per step.
	githubSummaryFailureMessageMaxLength = 16 * 1024
	// githubSummaryMaxFailures caps the number of failures detailed in the job summary.
	githubSummaryMaxFailures = 50
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

// specStartedEventKind marks the events that parallel processes send to the parent process through the parallel
// server's output channel when they start a spec.
const specStartedEventKind = "scylla-operator-tests/spec-started"

type specStartedEvent struct {
	Kind     string   `json:"kind"`
	Process  int      `json:"process"`
	Text     string   `json:"text"`
	Location string   `json:"location,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

// newSpecStartedNotifier connects to the parallel server and returns a function that tells the parent process about
// every spec that is about to run, as Ginkgo's own parallel protocol only reports specs once they finish.
// It's meant to be called from a ReportBeforeEach node in a parallel process.
func newSpecStartedNotifier(serverAddress string) (func(types.SpecReport), error) {
	client := parallel_support.NewClient(serverAddress)
	if !client.Connect() {
		return nil, fmt.Errorf("can't connect to the parallel server at %q", serverAddress)
	}

	return func(report types.SpecReport) {
		// Specs that are filtered out or otherwise won't run go through ReportBeforeEach too.
		if report.State.Is(types.SpecStateSkipped | types.SpecStatePending | types.SpecStateFailureStates) {
			return
		}

		data, err := json.Marshal(specStartedEvent{
			Kind:     specStartedEventKind,
			Process:  report.ParallelProcess,
			Text:     specText(report),
			Location: report.LeafNodeLocation.String(),
			Labels:   report.Labels(),
		})
		if err != nil {
			klog.ErrorS(err, "Can't encode the spec started event")
			return
		}

		_, err = client.Write(data)
		if err != nil {
			klog.ErrorS(err, "Can't send the spec started event")
		}
	}, nil
}

// githubReporter decorates a Ginkgo reporter for GitHub Actions logs. Each finished spec is folded into its own log
// group, titled with its state, duration, name and parallel process, and every failure is annotated, so a run reads
// as a list of specs rather than a wall of text. Specs are also announced as they start, so that the ones in flight
// can be told apart. When the suite ends, it writes a Markdown summary meant for the job summary page.
// The decorated reporter is set once the reporter config is known. It must write through the same lineTrackingWriter
// and must not emit log groups itself, as GitHub Actions doesn't support nesting them.
type githubReporter struct {
	reporters.Reporter

	// lock serializes the output, as the start of specs is reported independently of Ginkgo's reporting.
	lock sync.Mutex

	out         *lineTrackingWriter
	suiteName   string
	summaryPath string
	modulePath  string
	groupOpen   bool
	// suiteBegan tells whether the suite banner was emitted. Ginkgo holds back the reports of finished specs until
	// all the parallel processes begin the suite, but the start of specs comes through regardless, so it's held back
	// in pendingSpecStarts until then.
	suiteBegan        bool
	pendingSpecStarts []specStartedEvent
}

var _ reporters.Reporter = &githubReporter{}

func newGithubReporter(out *lineTrackingWriter, suiteName, summaryPath string) *githubReporter {
	modulePath := ""
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		modulePath = buildInfo.Main.Path
	}

	return &githubReporter{
		out:         out,
		suiteName:   suiteName,
		summaryPath: summaryPath,
		modulePath:  modulePath,
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
	return githubReporterOutputDestination{r: r}
}

type githubReporterOutputDestination struct {
	r *githubReporter
}

func (d githubReporterOutputDestination) Write(p []byte) (int, error) {
	d.r.lock.Lock()
	defer d.r.lock.Unlock()

	// Every write carries one whole message of a parallel process.
	event := specStartedEvent{}
	err := json.Unmarshal(p, &event)
	if err != nil || event.Kind != specStartedEventKind {
		return d.r.out.Write(p)
	}

	if !d.r.suiteBegan {
		d.r.pendingSpecStarts = append(d.r.pendingSpecStarts, event)
		return len(p), nil
	}

	d.r.writeSpecStarted(event)
	return len(p), nil
}

// writeSpecStarted announces a spec that started. It's a group, like the spec's result, so that the two line up in
// the log, and it holds the details of the spec that its title has no room for.
func (r *githubReporter) writeSpecStarted(event specStartedEvent) {
	r.beginGroup(logEntry("🏃", event.Process, "started", oneLine(event.Text)))
	if len(event.Location) != 0 {
		r.command("%s", r.repositoryPath(event.Location))
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

	if report.Failed() {
		message := failureMessage(report)
		if step := report.Failure.ProgressReport.CurrentStepText; len(step) != 0 {
			message = fmt.Sprintf("%s\n\nDuring step: %s", message, step)
		}
		r.command("::error %s::%s", r.annotationProperties(report), escapeGithubCommandData(truncate(message, githubAnnotationMessageMaxLength)))
	}
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

	if len(r.summaryPath) != 0 {
		err := os.WriteFile(r.summaryPath, []byte(r.summary(report)), 0666)
		if err != nil {
			r.command("::warning title=E2E summary::%s", escapeGithubCommandData(fmt.Sprintf("Can't write the summary to %q: %v", r.summaryPath, err)))
		}
	}

	// Folds the teardown output, mostly the logs of the parallel processes, until the run ends.
	r.beginGroup("Test suite teardown")
}

func (r *githubReporter) annotationProperties(report types.SpecReport) string {
	properties := []string{
		fmt.Sprintf("title=%s", escapeGithubCommandProperty(fmt.Sprintf("[%s] %s%s", strings.ToUpper(report.State.String()), processLabel(report.ParallelProcess), specText(report)))),
	}

	location := r.failureLocation(report)
	if len(location.FileName) != 0 {
		properties = append(properties, fmt.Sprintf("file=%s", escapeGithubCommandProperty(location.FileName)))
		if location.LineNumber > 0 {
			properties = append(properties, fmt.Sprintf("line=%d", location.LineNumber))
		}
	}

	return strings.Join(properties, ",")
}

// failureLocation returns the repository location closest to the failure. A failure may be located outside the
// repository, e.g. when a spec panics in the runtime, in which case it's the innermost repository frame of the stack
// trace, or the spec itself.
func (r *githubReporter) failureLocation(report types.SpecReport) types.CodeLocation {
	location := report.Failure.Location
	if r.inRepository(location.FileName) {
		return types.CodeLocation{
			FileName:   r.repositoryPath(location.FileName),
			LineNumber: location.LineNumber,
		}
	}

	for _, line := range strings.Split(location.FullStackTrace, "\n") {
		// Frame locations look like "<file>:<line> +0x28".
		fileName, lineNumberWithOffset, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found || !r.inRepository(fileName) {
			continue
		}
		lineNumber, err := strconv.Atoi(strings.Fields(lineNumberWithOffset + " ")[0])
		if err != nil {
			continue
		}
		return types.CodeLocation{
			FileName:   r.repositoryPath(fileName),
			LineNumber: lineNumber,
		}
	}

	location = report.LeafNodeLocation
	if r.inRepository(location.FileName) {
		return types.CodeLocation{
			FileName:   r.repositoryPath(location.FileName),
			LineNumber: location.LineNumber,
		}
	}

	return types.CodeLocation{}
}

// inRepository tells whether a file name belongs to this repository. Without the module path known, it assumes so.
func (r *githubReporter) inRepository(fileName string) bool {
	if len(fileName) == 0 {
		return false
	}
	if len(r.modulePath) == 0 {
		return true
	}
	return strings.HasPrefix(fileName, r.modulePath+"/") && strings.HasSuffix(fileName, ".go")
}

// repositoryPath turns a file name, which is module-qualified in binaries built with -trimpath, into a path relative
// to the repository root, as GitHub expects in annotations.
func (r *githubReporter) repositoryPath(fileName string) string {
	if len(r.modulePath) == 0 {
		return fileName
	}
	return strings.TrimPrefix(fileName, r.modulePath+"/")
}

func (r *githubReporter) summary(report types.Report) string {
	specs := report.SpecReports.WithLeafNodeType(types.NodeTypeIt)
	passed := specs.CountWithState(types.SpecStatePassed)
	failed := specs.CountWithState(types.SpecStateFailureStates)
	flaked := specs.CountOfFlakedSpecs()
	skipped := specs.CountWithState(types.SpecStateSkipped)
	pending := specs.CountWithState(types.SpecStatePending)
	failures := report.SpecReports.WithState(types.SpecStateFailureStates)

	sb := &strings.Builder{}

	icon, verdict := "✅", "passed"
	if !report.SuiteSucceeded {
		icon, verdict = "❌", "failed"
	}
	_, _ = fmt.Fprintf(sb, "### %s E2E suite `%s` %s\n\n", icon, r.suiteName, verdict)
	for _, reason := range report.SpecialSuiteFailureReasons {
		_, _ = fmt.Fprintf(sb, "> [!CAUTION]\n> %s\n\n", reason)
	}

	sb.WriteString("| ✅ Passed | ❌ Failed | 🔁 Flaked | ⏭️ Skipped |")
	if pending > 0 {
		sb.WriteString(" ⏸️ Pending |")
	}
	sb.WriteString(" ⏱️ Duration |\n")
	sb.WriteString("|---:|---:|---:|---:|")
	if pending > 0 {
		sb.WriteString("---:|")
	}
	sb.WriteString("---:|\n")
	_, _ = fmt.Fprintf(sb, "| %d | %d | %d | %d |", passed, failed, flaked, skipped)
	if pending > 0 {
		_, _ = fmt.Fprintf(sb, " %d |", pending)
	}
	_, _ = fmt.Fprintf(sb, " %s |\n\n", formatDuration(report.RunTime))

	_, _ = fmt.Fprintf(sb, "<sub>Ran %d of %d specs across %d parallel processes · random seed %d</sub>\n\n",
		passed+failed, report.PreRunStats.TotalSpecs, report.SuiteConfig.ParallelTotal, report.SuiteConfig.RandomSeed)

	// The failures are what the summary is mostly read for, so they're expanded.
	if len(failures) > 0 {
		_, _ = fmt.Fprintf(sb, "<details open><summary><b>❌ Failed (%d)</b></summary>\n\n", len(failures))
		for i, failure := range sortedByRunTime(failures) {
			if i == githubSummaryMaxFailures {
				_, _ = fmt.Fprintf(sb, "…and %d more, see the job log.\n\n", len(failures)-githubSummaryMaxFailures)
				break
			}
			r.writeSummaryFailure(sb, failure)
		}
		sb.WriteString("</details>\n\n")
	}

	var flakes types.SpecReports
	for _, spec := range specs {
		if spec.State.Is(types.SpecStatePassed) && spec.NumAttempts > 1 && spec.MaxFlakeAttempts > 1 {
			flakes = append(flakes, spec)
		}
	}
	if len(flakes) > 0 {
		_, _ = fmt.Fprintf(sb, "<details><summary><b>🔁 Flaked (%d)</b></summary>\n\n", len(flakes))
		sb.WriteString("| Attempts | Duration | Process | Spec |\n")
		sb.WriteString("|---:|---:|---:|:---|\n")
		for _, spec := range sortedByRunTime(flakes) {
			_, _ = fmt.Fprintf(sb, "| %d | %s | %s | %s |\n", spec.NumAttempts, formatDuration(spec.RunTime), summaryProcess(spec), summaryTableText(spec))
		}
		sb.WriteString("\n</details>\n\n")
	}

	passedSpecs := specs.WithState(types.SpecStatePassed)
	if len(passedSpecs) > 0 {
		_, _ = fmt.Fprintf(sb, "<details><summary><b>✅ Passed (%d)</b></summary>\n\n", len(passedSpecs))
		sb.WriteString("| Duration | Process | Spec |\n")
		sb.WriteString("|---:|---:|:---|\n")
		for _, spec := range sortedByRunTime(passedSpecs) {
			text := summaryTableText(spec)
			// Flaky specs passed too, but they're worth telling apart.
			if spec.NumAttempts > 1 && spec.MaxFlakeAttempts > 1 {
				text = fmt.Sprintf("🔁 %s", text)
			}
			_, _ = fmt.Fprintf(sb, "| %s | %s | %s |\n", formatDuration(spec.RunTime), summaryProcess(spec), text)
		}
		sb.WriteString("\n</details>\n\n")
	}

	skippedSpecs := specs.WithState(types.SpecStateSkipped)
	if len(skippedSpecs) > 0 {
		_, _ = fmt.Fprintf(sb, "<details><summary><b>⏭️ Skipped (%d)</b></summary>\n\n", len(skippedSpecs))
		for _, spec := range skippedSpecs {
			_, _ = fmt.Fprintf(sb, "- %s\n", html.EscapeString(oneLine(specText(spec))))
		}
		sb.WriteString("\n</details>\n\n")
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func (r *githubReporter) writeSummaryFailure(sb *strings.Builder, report types.SpecReport) {
	details := fmt.Sprintf("after %s", formatDuration(report.RunTime))
	if report.ParallelProcess > 0 {
		details = fmt.Sprintf("process #%d · %s", report.ParallelProcess, details)
	}
	_, _ = fmt.Fprintf(sb, "<details><summary>%s <b>%s</b> <code>[%s]</code> <sub>%s</sub></summary>\n\n",
		specStateIcon(report), html.EscapeString(specText(report)), strings.ToUpper(report.State.String()), details)

	location := r.failureLocation(report)
	if len(location.FileName) != 0 {
		_, _ = fmt.Fprintf(sb, "In <code>[%s]</code> at <code>%s:%d</code>", report.Failure.FailureNodeType, html.EscapeString(location.FileName), location.LineNumber)
		if len(report.Failure.ProgressReport.CurrentStepText) != 0 {
			_, _ = fmt.Fprintf(sb, ", during step <i>%s</i>", html.EscapeString(report.Failure.ProgressReport.CurrentStepText))
		}
		sb.WriteString("\n\n")
	}

	_, _ = fmt.Fprintf(sb, "<pre>%s</pre>\n\n</details>\n\n", html.EscapeString(truncate(failureMessage(report), githubSummaryFailureMessageMaxLength)))
}

// failureMessage returns the message of a spec's failure, including the value of a panic, which Ginkgo reports apart.
func failureMessage(report types.SpecReport) string {
	if len(report.Failure.ForwardedPanic) != 0 {
		return fmt.Sprintf("%s: %s", report.Failure.Message, report.Failure.ForwardedPanic)
	}
	return report.Failure.Message
}

// sortedByRunTime returns a copy of the specs, the longest running first.
func sortedByRunTime(specs types.SpecReports) types.SpecReports {
	sorted := slices.Clone(specs)
	slices.SortStableFunc(sorted, func(a, b types.SpecReport) int {
		return cmp.Compare(b.RunTime, a.RunTime)
	})
	return sorted
}

func summaryProcess(report types.SpecReport) string {
	if report.ParallelProcess <= 0 {
		return ""
	}
	return fmt.Sprintf("#%d", report.ParallelProcess)
}

func summaryTableText(report types.SpecReport) string {
	return strings.ReplaceAll(html.EscapeString(oneLine(specText(report))), "|", `\|`)
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

// processLabel identifies the parallel process that runs a spec, if any.
func processLabel(process int) string {
	if process <= 0 {
		return ""
	}
	return fmt.Sprintf("[#%d] ", process)
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

func truncate(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return strings.ToValidUTF8(s[:maxLength], "") + "\n…(truncated)"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// escapeGithubCommandData escapes the data of a GitHub Actions workflow command.
// https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts
func escapeGithubCommandData(s string) string {
	return strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	).Replace(s)
}

// escapeGithubCommandProperty escapes a property value of a GitHub Actions workflow command.
// https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts
func escapeGithubCommandProperty(s string) string {
	return strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
		":", "%3A",
		",", "%2C",
	).Replace(s)
}
