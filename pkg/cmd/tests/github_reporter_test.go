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
	report.ParallelProcess = 3
	report.RunningInParallel = true
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
				"::group::✅    6.2s  ScyllaCluster webhook should reject an invalid update",
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
				"::group::🔁    3m1s  ScyllaCluster should scale",
				"spec body",
				"::endgroup::",
				"",
			}, "\n"),
		},
		{
			name:   "failed spec is folded into a group and annotated",
			report: newTestFailedSpecReport(),
			expectedOutput: strings.Join([]string{
				"::group::❌ [#3]    4m59s  ScyllaCluster should connect, via Ingresses [FAILED]",
				"spec body",
				"::endgroup::",
				"::error title=[FAILED] [#3] ScyllaCluster should connect%2C via Ingresses,file=test/e2e/set/scyllacluster/scyllacluster_expose.go,line=51::Timed out after 300s.%0AExpected <bool>: false%0Ato be 100%25 true%0A%0ADuring step: Connecting to cluster via Ingress",
				"",
			}, "\n"),
		},
		{
			name: "panic is annotated at the innermost repository frame, with its value",
			report: func() types.SpecReport {
				report := newTestSpecReport([]string{"ScyllaCluster"}, "should not panic", types.SpecStatePanicked, 0)
				report.LeafNodeLocation = types.CodeLocation{
					FileName:   "github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster.go",
					LineNumber: 20,
				}
				report.Failure = types.Failure{
					Message:        "Test Panicked",
					ForwardedPanic: "assignment to entry in nil map",
					Location: types.CodeLocation{
						FileName:   "internal/runtime/maps/runtime_faststr.go",
						LineNumber: 263,
						FullStackTrace: strings.Join([]string{
							"github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster.init.func3.3()",
							"\tgithub.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster.go:28 +0x28",
						}, "\n"),
					},
				}
				return report
			}(),
			expectedOutput: strings.Join([]string{
				"::group::❌      0s  ScyllaCluster should not panic [PANICKED]",
				"spec body",
				"::endgroup::",
				"::error title=[PANICKED] ScyllaCluster should not panic,file=test/e2e/set/scyllacluster/scyllacluster.go,line=28::Test Panicked: assignment to entry in nil map",
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
				"::group::❌      1s  [SynchronizedBeforeSuite] [PANICKED]",
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
		"::group::✅      1s  ScyllaCluster should work",
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
		"<details open><summary><b>❌ Failed (1)</b></summary>",
		"",
		"<details><summary>❌ <b>ScyllaCluster should connect, via Ingresses</b> <code>[FAILED]</code> <sub>process #3 · after 4m59s</sub></summary>",
		"",
		"In <code>[It]</code> at <code>test/e2e/set/scyllacluster/scyllacluster_expose.go:51</code>, during step <i>Connecting to cluster via Ingress</i>",
		"",
		"<pre>Timed out after 300s.",
		"Expected &lt;bool&gt;: false",
		"to be 100% true</pre>",
		"",
		"</details>",
		"",
		"</details>",
		"",
		"<details><summary><b>🔁 Flaked (1)</b></summary>",
		"",
		"| Attempts | Duration | Process | Spec |",
		"|---:|---:|---:|:---|",
		"| 2 | 2m0s |  | ScyllaCluster should scale |",
		"",
		"</details>",
		"",
		"<details><summary><b>✅ Passed (2)</b></summary>",
		"",
		"| Duration | Process | Spec |",
		"|---:|---:|:---|",
		"| 2m0s |  | 🔁 ScyllaCluster should scale |",
		"| 10s |  | ScyllaCluster should &lt;deploy&gt; \\| fast |",
		"",
		"</details>",
		"",
		"<details><summary><b>⏭️ Skipped (2)</b></summary>",
		"",
		"- ScyllaCluster should be skipped",
		"- ScyllaCluster should be skipped too",
		"",
		"</details>",
		"",
	}, "\n")
	if diff := cmp.Diff(expectedSummary, string(summary)); diff != "" {
		t.Errorf("expected and got summary differ:\n%s", diff)
	}
}

func TestGithubReporterOutputDestination(t *testing.T) {
	t.Parallel()

	r, buf := newTestGithubReporter("")
	destination := r.outputDestination()

	// Starts are held back until the suite begins.
	_, err := destination.Write([]byte(`{"kind":"scylla-operator-tests/spec-started","process":2,"text":"ScyllaCluster should start early"}`))
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output before the suite begins, got %q", buf.String())
	}
	r.SuiteWillBegin(types.Report{})

	writes := []string{
		`{"kind":"scylla-operator-tests/spec-started","process":3,"text":"ScyllaCluster should\nscale","location":"github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster_scale.go:42","labels":["SuiteKindFast","SuiteParallel"]}`,
		"partial output",
		`{"kind":"something-else"}`,
		"\n",
		`{"kind":"scylla-operator-tests/spec-started","process":1,"text":"ScyllaCluster should deploy"}`,
	}
	for _, w := range writes {
		n, err := destination.Write([]byte(w))
		if err != nil {
			t.Fatal(err)
		}
		if n != len(w) {
			t.Errorf("expected to write %d bytes, got %d", len(w), n)
		}
	}

	expectedOutput := strings.Join([]string{
		"::group::🏃 [#2]  started  ScyllaCluster should start early",
		"::endgroup::",
		"::group::🏃 [#3]  started  ScyllaCluster should scale",
		"test/e2e/set/scyllacluster/scyllacluster_scale.go:42",
		"Labels: SuiteKindFast, SuiteParallel",
		"::endgroup::",
		`partial output{"kind":"something-else"}`,
		"::group::🏃 [#1]  started  ScyllaCluster should deploy",
		"::endgroup::",
		"",
	}, "\n")
	if diff := cmp.Diff(expectedOutput, buf.String()); diff != "" {
		t.Errorf("expected and got output differ:\n%s", diff)
	}
}

func TestProgressReportTitle(t *testing.T) {
	t.Parallel()

	specStartTime := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

	tt := []struct {
		name          string
		report        types.ProgressReport
		expectedTitle string
	}{
		{
			name: "parallel spec at a step",
			report: types.ProgressReport{
				ParallelProcess:         3,
				RunningInParallel:       true,
				ContainerHierarchyTexts: []string{"ScyllaCluster"},
				LeafNodeText:            "should scale",
				SpecStartTime:           specStartTime,
				CurrentStepText:         "Waiting for the ScyllaCluster to roll out",
				TimelineLocation: types.TimelineLocation{
					Time: specStartTime.Add(10*time.Minute + 200*time.Millisecond),
				},
			},
			expectedTitle: `⏳ [#3]    10m0s  ScyllaCluster should scale (still running), at step "Waiting for the ScyllaCluster to roll out"`,
		},
		{
			name: "serial spec without timing",
			report: types.ProgressReport{
				ContainerHierarchyTexts: []string{"ScyllaCluster"},
				LeafNodeText:            "should scale",
			},
			expectedTitle: "⏳          ScyllaCluster should scale (still running)",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tc.expectedTitle, progressReportTitle(tc.report)); diff != "" {
				t.Errorf("expected and got title differ:\n%s", diff)
			}
		})
	}
}
