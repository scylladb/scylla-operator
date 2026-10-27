// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"bytes"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
)

// fakeDelegateReporter stands in for Ginkgo's default reporter, writing a fixed body for every spec it reports.
type fakeDelegateReporter struct {
	reporters.NoopReporter
	out *lineTrackingWriter
}

func (r *fakeDelegateReporter) DidRun(report types.SpecReport) {
	if report.State.Is(types.SpecStateSkipped) {
		return
	}
	// Leaves the line unterminated on purpose, the reporter has to break it before the next command.
	_, _ = r.out.Write([]byte("spec body"))
}

func (r *fakeDelegateReporter) SuiteDidEnd(report types.Report) {
	_, _ = r.out.Write([]byte("suite summary\n"))
}

func newTestGithubReporter(summaryPath string) (*githubReporter, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	out := newLineTrackingWriter(buf)
	r := newGithubReporter(out, "kind-fast", summaryPath)
	r.modulePath = "github.com/scylladb/scylla-operator"
	r.Reporter = &fakeDelegateReporter{out: out}
	return r, buf
}

func newTestSpecReport(texts []string, leaf string, state types.SpecState, runTime time.Duration) types.SpecReport {
	return types.SpecReport{
		ContainerHierarchyTexts: texts,
		LeafNodeText:            leaf,
		LeafNodeType:            types.NodeTypeIt,
		State:                   state,
		RunTime:                 runTime,
		NumAttempts:             1,
		MaxFlakeAttempts:        1,
	}
}

func newTestFailedSpecReport() types.SpecReport {
	report := newTestSpecReport([]string{"ScyllaCluster"}, "should connect, via Ingresses", types.SpecStateFailed, 298690*time.Millisecond)
	report.Failure = types.Failure{
		Message:         "Timed out after 300s.\nExpected <bool>: false\nto be 100% true",
		FailureNodeType: types.NodeTypeIt,
		Location: types.CodeLocation{
			FileName:   "github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster_expose.go",
			LineNumber: 51,
		},
	}
	report.Failure.ProgressReport.CurrentStepText = "Connecting to cluster via Ingress"
	return report
}

func TestGithubReporterDidRun(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		report         types.SpecReport
		expectedOutput string
	}{
		{
			name:   "passed spec is folded into a group",
			report: newTestSpecReport([]string{"ScyllaCluster", "webhook"}, "should reject\nan invalid update", types.SpecStatePassed, 6210*time.Millisecond),
			expectedOutput: strings.Join([]string{
				"::group::✅ ScyllaCluster webhook should reject an invalid update (6.2s)",
				"spec body",
				"::endgroup::",
				"",
			}, "\n"),
		},
		{
			name: "flaky spec is marked as such",
			report: func() types.SpecReport {
				report := newTestSpecReport([]string{"ScyllaCluster"}, "should scale", types.SpecStatePassed, 3*time.Minute+500*time.Millisecond)
				report.NumAttempts = 2
				report.MaxFlakeAttempts = 3
				return report
			}(),
			expectedOutput: strings.Join([]string{
				"::group::🔁 ScyllaCluster should scale (3m1s)",
				"spec body",
				"::endgroup::",
				"",
			}, "\n"),
		},
		{
			name:   "failed spec is folded into a group and annotated",
			report: newTestFailedSpecReport(),
			expectedOutput: strings.Join([]string{
				"::group::❌ ScyllaCluster should connect, via Ingresses [FAILED] (4m59s)",
				"spec body",
				"::endgroup::",
				"::error title=[FAILED] ScyllaCluster should connect%2C via Ingresses,file=test/e2e/set/scyllacluster/scyllacluster_expose.go,line=51::Timed out after 300s.%0AExpected <bool>: false%0Ato be 100%25 true",
				"",
			}, "\n"),
		},
		{
			name: "suite node failure is titled by its node type",
			report: func() types.SpecReport {
				report := newTestSpecReport(nil, "", types.SpecStatePanicked, time.Second)
				report.LeafNodeType = types.NodeTypeSynchronizedBeforeSuite
				report.Failure = types.Failure{
					Message: "boom",
				}
				return report
			}(),
			expectedOutput: strings.Join([]string{
				"::group::❌ [SynchronizedBeforeSuite] [PANICKED] (1s)",
				"spec body",
				"::endgroup::",
				"::error title=[PANICKED] [SynchronizedBeforeSuite]::boom",
				"",
			}, "\n"),
		},
		{
			name:           "skipped spec isn't folded",
			report:         newTestSpecReport([]string{"ScyllaCluster"}, "should be skipped", types.SpecStateSkipped, 0),
			expectedOutput: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, buf := newTestGithubReporter("")
			r.DidRun(tc.report)

			if diff := cmp.Diff(tc.expectedOutput, buf.String()); diff != "" {
				t.Errorf("expected and got output differ:\n%s", diff)
			}
		})
	}
}

func TestGithubReporterSetupGroup(t *testing.T) {
	t.Parallel()

	r, buf := newTestGithubReporter("")
	r.startGroup("Test suite setup")
	_, _ = r.out.Write([]byte("FLAG: --v=2"))
	r.SuiteWillBegin(types.Report{})
	r.DidRun(newTestSpecReport([]string{"ScyllaCluster"}, "should work", types.SpecStatePassed, time.Second))

	expectedOutput := strings.Join([]string{
		"::group::Test suite setup",
		"FLAG: --v=2",
		"::endgroup::",
		"::group::✅ ScyllaCluster should work (1s)",
		"spec body",
		"::endgroup::",
		"",
	}, "\n")
	if diff := cmp.Diff(expectedOutput, buf.String()); diff != "" {
		t.Errorf("expected and got output differ:\n%s", diff)
	}
}

func TestGithubReporterSummary(t *testing.T) {
	t.Parallel()

	summaryPath := path.Join(t.TempDir(), "e2e-summary.md")
	r, buf := newTestGithubReporter(summaryPath)

	flake := newTestSpecReport([]string{"ScyllaCluster"}, "should scale", types.SpecStatePassed, 2*time.Minute)
	flake.NumAttempts = 2
	flake.MaxFlakeAttempts = 2

	report := types.Report{
		SuiteSucceeded: false,
		RunTime:        38*time.Minute + 11*time.Second,
		SuiteConfig: types.SuiteConfig{
			ParallelTotal: 8,
			RandomSeed:    1793057848,
		},
		PreRunStats: types.PreRunStats{
			TotalSpecs: 5,
		},
		SpecReports: types.SpecReports{
			newTestSpecReport([]string{"ScyllaCluster"}, "should <deploy> | fast", types.SpecStatePassed, 10*time.Second),
			flake,
			newTestFailedSpecReport(),
			newTestSpecReport([]string{"ScyllaCluster"}, "should be skipped", types.SpecStateSkipped, 0),
			newTestSpecReport([]string{"ScyllaCluster"}, "should be skipped too", types.SpecStateSkipped, 0),
		},
	}
	r.SuiteDidEnd(report)

	if diff := cmp.Diff("suite summary\n::group::Test suite teardown\n", buf.String()); diff != "" {
		t.Errorf("expected and got output differ:\n%s", diff)
	}

	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}

	expectedSummary := strings.Join([]string{
		"### ❌ E2E suite `kind-fast` failed",
		"",
		"| ✅ Passed | ❌ Failed | 🔁 Flaked | ⏭️ Skipped | ⏱️ Duration |",
		"|---:|---:|---:|---:|---:|",
		"| 2 | 1 | 1 | 2 | 38m11s |",
		"",
		"<sub>Ran 3 of 5 specs across 8 parallel processes · random seed 1793057848</sub>",
		"",
		"#### Failures",
		"",
		"<details><summary>❌ <b>ScyllaCluster should connect, via Ingresses</b> <code>[FAILED]</code> <sub>after 4m59s</sub></summary>",
		"",
		"In <code>[It]</code> at <code>test/e2e/set/scyllacluster/scyllacluster_expose.go:51</code>, during step <i>Connecting to cluster via Ingress</i>",
		"",
		"<pre>Timed out after 300s.",
		"Expected &lt;bool&gt;: false",
		"to be 100% true</pre>",
		"",
		"</details>",
		"",
		"#### Flakes",
		"",
		"- 🔁 ScyllaCluster should scale <sub>(2 attempts)</sub>",
		"",
		"<details><summary>⏱️ Slowest specs</summary>",
		"",
		"| Duration | Spec |",
		"|---:|:---|",
		"| 4m59s | ❌ ScyllaCluster should connect, via Ingresses |",
		"| 2m0s | 🔁 ScyllaCluster should scale |",
		"| 10s | ✅ ScyllaCluster should &lt;deploy&gt; \\| fast |",
		"",
		"</details>",
		"",
	}, "\n")
	if diff := cmp.Diff(expectedSummary, string(summary)); diff != "" {
		t.Errorf("expected and got summary differ:\n%s", diff)
	}
}
