// Copyright (c) 2026 ScyllaDB.

package tests

import (
	"bytes"
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
	_, _ = r.out.Write([]byte("suite results\n"))
}

func newTestGithubReporter() (*githubReporter, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	out := newLineTrackingWriter(buf)
	r := newGithubReporter(out)
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
		Message:         "Timed out after 300s.",
		FailureNodeType: types.NodeTypeIt,
	}
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
			name:   "failed spec is folded into a group",
			report: newTestFailedSpecReport(),
			expectedOutput: strings.Join([]string{
				"::group::❌ [#3]    4m59s  ScyllaCluster should connect, via Ingresses [FAILED]",
				"spec body",
				"::endgroup::",
				"",
			}, "\n"),
		},
		{
			name: "timed out spec is marked as such",
			report: func() types.SpecReport {
				report := newTestSpecReport([]string{"ScyllaCluster"}, "should roll out", types.SpecStateTimedout, 10*time.Minute)
				report.Failure = types.Failure{
					Message: "A spec timeout occurred",
				}
				return report
			}(),
			expectedOutput: strings.Join([]string{
				"::group::⏱️   10m0s  ScyllaCluster should roll out [TIMEDOUT]",
				"spec body",
				"::endgroup::",
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

			r, buf := newTestGithubReporter()
			r.DidRun(tc.report)

			if diff := cmp.Diff(tc.expectedOutput, buf.String()); diff != "" {
				t.Errorf("expected and got output differ:\n%s", diff)
			}
		})
	}
}

func TestGithubReporterSetupGroup(t *testing.T) {
	t.Parallel()

	r, buf := newTestGithubReporter()
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

func TestGithubReporterSuiteDidEnd(t *testing.T) {
	t.Parallel()

	r, buf := newTestGithubReporter()
	r.startGroup("✅ ScyllaCluster should work")
	r.SuiteDidEnd(types.Report{})

	expectedOutput := strings.Join([]string{
		"::group::✅ ScyllaCluster should work",
		"::endgroup::",
		"suite results",
		"::group::Test suite teardown",
		"",
	}, "\n")
	if diff := cmp.Diff(expectedOutput, buf.String()); diff != "" {
		t.Errorf("expected and got output differ:\n%s", diff)
	}
}

func TestGithubReporterOutputDestination(t *testing.T) {
	t.Parallel()

	r, buf := newTestGithubReporter()
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
		"github.com/scylladb/scylla-operator/test/e2e/set/scyllacluster/scyllacluster_scale.go:42",
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
