// Copyright (c) 2026 ScyllaDB.

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/reconcilecritic"
	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
	"sigs.k8s.io/controller-runtime/pkg/reconciletrace/critic"
)

const (
	// reconcileTraceDirEnv names the directory every spec running the ScyllaDBDatacenter controller records its
	// reconcile trace to, as <spec>.jsonl with <spec>.meta.json, and a review of the trace's signals in
	// <spec>.review.md. Unset, nothing is recorded. go run ./hack/reconcilecritic --traces-dir renders the directory
	// as HTML.
	reconcileTraceDirEnv = "SCYLLA_OPERATOR_ENVTEST_RECONCILE_TRACE_DIR"
	// reconcileCriticEnv set to "claude" has the claude CLI review the trace too. It costs model usage; it is never
	// on by default.
	reconcileCriticEnv = "SCYLLA_OPERATOR_ENVTEST_RECONCILE_CRITIC"
	// reconcileCriticModelEnv picks the model the claude CLI uses.
	reconcileCriticModelEnv = "SCYLLA_OPERATOR_ENVTEST_RECONCILE_CRITIC_MODEL"
)

var nonFileNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// specTraceRecorderFromEnv returns a recorder of the current spec's trace, or nil if reconcileTraceDirEnv is unset.
// The trace is written and reviewed when the spec's cleanups run; register this before the controller's cleanup so
// that it comes after the controller stops.
func specTraceRecorderFromEnv() reconciletrace.Recorder {
	g.GinkgoHelper()

	dir := os.Getenv(reconcileTraceDirEnv)
	if dir == "" {
		return nil
	}

	err := os.MkdirAll(dir, 0o755)
	o.Expect(err).NotTo(o.HaveOccurred())

	name := nonFileNameChars.ReplaceAllString(g.CurrentSpecReport().FullText(), "_")
	name = strings.Trim(name, "_")
	if len(name) > 200 {
		name = name[:200]
	}
	base := filepath.Join(dir, name)

	f, err := os.Create(base + ".jsonl")
	o.Expect(err).NotTo(o.HaveOccurred())

	jsonl := reconciletrace.NewJSONLRecorder(f)
	memory := reconciletrace.NewMemoryRecorder()
	g.DeferCleanup(func(ctx context.Context) {
		o.Expect(jsonl.Flush()).To(o.Succeed())
		o.Expect(f.Close()).To(o.Succeed())

		sr := g.CurrentSpecReport()
		err := reconcilecritic.WriteTraceMeta(base+".meta.json", reconcilecritic.TraceMeta{
			TestName: sr.FullText(),
			Failed:   sr.Failed(),
			Source:   "envtest",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		reviewSpecTrace(ctx, base, memory.Records())
	})

	return reconciletrace.MultiRecorder{jsonl, memory}
}

// reviewSpecTrace writes the review of every episode of records next to the trace.
func reviewSpecTrace(ctx context.Context, base string, records []reconciletrace.Record) {
	g.GinkgoHelper()

	opts := critic.Options{
		ContextProviders: []critic.ContextProvider{reconcilecritic.ScyllaDBDatacenterContext(true)},
	}
	if os.Getenv(reconcileCriticEnv) == "claude" {
		repoRoot, err := filepath.Abs("../../..")
		o.Expect(err).NotTo(o.HaveOccurred())
		opts.Model = reconcilecritic.ClaudeCLIModel{
			Model: os.Getenv(reconcileCriticModelEnv),
			Dir:   repoRoot,
		}
	}
	c := critic.New(opts)

	var reviews []string
	for _, e := range critic.SplitEpisodes(records, 0) {
		report, err := c.Review(ctx, &e)
		if err != nil {
			// A failed review doesn't fail the spec; the trace is still there to review again offline.
			g.GinkgoWriter.Printf("Can't review the reconcile trace of %s: %v\n", e.Request, err)
		}
		if report == nil {
			continue
		}
		reviews = append(reviews, report.Markdown())
		if report.Reviewed {
			prompt := fmt.Sprintf("# System\n\n%s\n\n# User\n\n%s\n\n# Response\n\n%s\n", report.Prompt.System, report.Prompt.User, report.Response)
			o.Expect(os.WriteFile(base+".prompt.md", []byte(prompt), 0o644)).To(o.Succeed())
		}
	}

	o.Expect(os.WriteFile(base+".review.md", []byte(strings.Join(reviews, "\n---\n\n")), 0o644)).To(o.Succeed())
}
