// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"cmp"
	"fmt"
	"html"
	"io"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
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
	// githubSummarySlowestSpecs is the number of the slowest specs listed in the job summary.
	githubSummarySlowestSpecs = 10
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
// group, titled with its state, duration and name, and every failure is annotated, so a run reads as a list of specs
// rather than a wall of text. When the suite ends, it writes a Markdown summary meant for the job summary page.
// The decorated reporter is set once the reporter config is known. It must write through the same lineTrackingWriter
// and must not emit log groups itself, as GitHub Actions doesn't support nesting them.
type githubReporter struct {
	reporters.Reporter

	out         *lineTrackingWriter
	suiteName   string
	summaryPath string
	modulePath  string
	groupOpen   bool
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
	// Groups don't nest, so a new one has to close the previous one first.
	r.endGroup()
	r.command("::group::%s", oneLine(title))
	r.groupOpen = true
}

func (r *githubReporter) endGroup() {
	if !r.groupOpen {
		return
	}
	r.command("::endgroup::")
	r.groupOpen = false
}

func (r *githubReporter) SuiteWillBegin(report types.Report) {
	// Closes the group with the setup output, if any.
	r.endGroup()
	r.Reporter.SuiteWillBegin(report)
}

func (r *githubReporter) DidRun(report types.SpecReport) {
	if report.State.Is(types.SpecStateSkipped) && report.Failure.IsZero() {
		r.Reporter.DidRun(report)
		return
	}

	r.startGroup(specTitle(report))
	r.Reporter.DidRun(report)
	r.endGroup()

	if report.Failed() {
		r.command("::error %s::%s", r.annotationProperties(report), escapeGithubCommandData(truncate(report.Failure.Message, githubAnnotationMessageMaxLength)))
	}
}

func (r *githubReporter) EmitProgressReport(report types.ProgressReport) {
	r.startGroup(fmt.Sprintf("⏳ Progress report: %s", progressReportSubject(report)))
	r.Reporter.EmitProgressReport(report)
	r.endGroup()
}

func (r *githubReporter) SuiteDidEnd(report types.Report) {
	r.endGroup()
	r.Reporter.SuiteDidEnd(report)

	if len(r.summaryPath) != 0 {
		err := os.WriteFile(r.summaryPath, []byte(r.summary(report)), 0666)
		if err != nil {
			r.command("::warning title=E2E summary::%s", escapeGithubCommandData(fmt.Sprintf("Can't write the summary to %q: %v", r.summaryPath, err)))
		}
	}

	// Folds the teardown output, mostly the logs of the parallel processes, until the run ends.
	r.startGroup("Test suite teardown")
}

func (r *githubReporter) annotationProperties(report types.SpecReport) string {
	properties := []string{
		fmt.Sprintf("title=%s", escapeGithubCommandProperty(fmt.Sprintf("[%s] %s", strings.ToUpper(report.State.String()), specText(report)))),
	}

	location := report.Failure.Location
	if len(location.FileName) != 0 {
		properties = append(properties, fmt.Sprintf("file=%s", escapeGithubCommandProperty(r.repositoryPath(location.FileName))))
		if location.LineNumber > 0 {
			properties = append(properties, fmt.Sprintf("line=%d", location.LineNumber))
		}
	}

	return strings.Join(properties, ",")
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

	if len(failures) > 0 {
		_, _ = fmt.Fprintf(sb, "#### Failures\n\n")
		for i, failure := range failures {
			if i == githubSummaryMaxFailures {
				_, _ = fmt.Fprintf(sb, "…and %d more, see the job log.\n\n", len(failures)-githubSummaryMaxFailures)
				break
			}
			r.writeSummaryFailure(sb, failure)
		}
	}

	var flakes types.SpecReports
	for _, spec := range specs {
		if spec.State.Is(types.SpecStatePassed) && spec.NumAttempts > 1 && spec.MaxFlakeAttempts > 1 {
			flakes = append(flakes, spec)
		}
	}
	if len(flakes) > 0 {
		_, _ = fmt.Fprintf(sb, "#### Flakes\n\n")
		for _, flake := range flakes {
			_, _ = fmt.Fprintf(sb, "- 🔁 %s <sub>(%d attempts)</sub>\n", html.EscapeString(specText(flake)), flake.NumAttempts)
		}
		sb.WriteString("\n")
	}

	var ran types.SpecReports
	for _, spec := range specs {
		if spec.State.Is(types.SpecStatePassed | types.SpecStateFailureStates) {
			ran = append(ran, spec)
		}
	}
	slices.SortStableFunc(ran, func(a, b types.SpecReport) int {
		return cmp.Compare(b.RunTime, a.RunTime)
	})
	if len(ran) > 0 {
		_, _ = fmt.Fprintf(sb, "<details><summary>⏱️ Slowest specs</summary>\n\n")
		sb.WriteString("| Duration | Spec |\n")
		sb.WriteString("|---:|:---|\n")
		for _, spec := range ran[:min(len(ran), githubSummarySlowestSpecs)] {
			_, _ = fmt.Fprintf(sb, "| %s | %s %s |\n", formatDuration(spec.RunTime), specStateIcon(spec), strings.ReplaceAll(html.EscapeString(specText(spec)), "|", `\|`))
		}
		sb.WriteString("\n</details>\n")
	}

	return sb.String()
}

func (r *githubReporter) writeSummaryFailure(sb *strings.Builder, report types.SpecReport) {
	_, _ = fmt.Fprintf(sb, "<details><summary>%s <b>%s</b> <code>[%s]</code> <sub>after %s</sub></summary>\n\n",
		specStateIcon(report), html.EscapeString(specText(report)), strings.ToUpper(report.State.String()), formatDuration(report.RunTime))

	location := report.Failure.Location
	if len(location.FileName) != 0 {
		_, _ = fmt.Fprintf(sb, "In <code>[%s]</code> at <code>%s:%d</code>", report.Failure.FailureNodeType, html.EscapeString(r.repositoryPath(location.FileName)), location.LineNumber)
		if len(report.Failure.ProgressReport.CurrentStepText) != 0 {
			_, _ = fmt.Fprintf(sb, ", during step <i>%s</i>", html.EscapeString(report.Failure.ProgressReport.CurrentStepText))
		}
		sb.WriteString("\n\n")
	}

	_, _ = fmt.Fprintf(sb, "<pre>%s</pre>\n\n</details>\n\n", html.EscapeString(truncate(report.Failure.Message, githubSummaryFailureMessageMaxLength)))
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

func specTitle(report types.SpecReport) string {
	title := fmt.Sprintf("%s %s", specStateIcon(report), specText(report))
	if !report.State.Is(types.SpecStatePassed) {
		title = fmt.Sprintf("%s [%s]", title, strings.ToUpper(report.State.String()))
	}
	if report.RunTime > 0 {
		title = fmt.Sprintf("%s (%s)", title, formatDuration(report.RunTime))
	}
	return title
}

func progressReportSubject(report types.ProgressReport) string {
	subject := strings.Join(append(slices.Clone(report.ContainerHierarchyTexts), report.LeafNodeText), " ")
	if len(strings.TrimSpace(subject)) == 0 {
		subject = report.Message
	}
	if report.RunningInParallel {
		subject = fmt.Sprintf("%s (process #%d)", subject, report.ParallelProcess)
	}
	return subject
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
