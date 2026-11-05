// Copyright (c) 2026 ScyllaDB.

// Command reconcilecritic reviews the reconcile traces recorded by controller-runtime's reconciletrace and renders the
// reviews as HTML or Markdown. It reads the traces of envtest (SCYLLA_OPERATOR_ENVTEST_RECONCILE_TRACE_DIR) and of
// e2e (the reconcile-traces directories of the artifacts), grouped by test.
//
// Without a model it reports the signals only, which is quick. With --model=claude every episode costs a model call;
// narrow the run with --focus and keep the answers with --reviews-dir.
//
//	go run ./hack/reconcilecritic --traces-dir <artifacts> --html review.html
//	go run ./hack/reconcilecritic --traces-dir <artifacts> --focus 'scale' --model claude --reviews-dir .reviews --html review.html
package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/reconcilecritic"
	"sigs.k8s.io/controller-runtime/pkg/reconciletrace"
	"sigs.k8s.io/controller-runtime/pkg/reconciletrace/critic"
)

type options struct {
	traces           []string
	tracesDir        string
	focus            *regexp.Regexp
	model            string
	claudeModel      string
	repoDir          string
	parallel         int
	reviewsDir       string
	htmlPath         string
	title            string
	maxTimelineBytes int
	minReconciles    int
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags() (*options, error) {
	o := &options{}
	var traces, focus string
	flag.StringVar(&traces, "trace", "", "Comma-separated JSON lines traces to review.")
	flag.StringVar(&o.tracesDir, "traces-dir", "", "Directory searched recursively for *.jsonl traces.")
	flag.StringVar(&focus, "focus", "", "Review only the tests whose name matches this regular expression.")
	flag.StringVar(&o.model, "model", "none", `Model to review with: "none" for the signals only, or "claude" for the claude CLI.`)
	flag.StringVar(&o.claudeModel, "claude-model", "", "Model the claude CLI uses.")
	flag.StringVar(&o.repoDir, "repo-dir", ".", "Repository root the model may read.")
	flag.IntVar(&o.parallel, "parallel", 4, "Model calls running at once.")
	flag.StringVar(&o.reviewsDir, "reviews-dir", "", "Directory keeping the model's answers by prompt, so that a rerun only asks about what changed.")
	flag.StringVar(&o.htmlPath, "html", "", "Write the review as HTML to this file instead of Markdown to standard output.")
	flag.StringVar(&o.title, "title", "Reconcile review", "Title of the HTML review.")
	flag.IntVar(&o.maxTimelineBytes, "max-timeline-bytes", 0, "Cap of an episode's timeline; 0 keeps the default.")
	flag.IntVar(&o.minReconciles, "min-reconciles", 1, "Skip episodes with fewer reconciles.")
	flag.Parse()

	if traces != "" {
		o.traces = strings.Split(traces, ",")
	}
	if len(o.traces) == 0 && o.tracesDir == "" {
		return nil, errors.New("--trace or --traces-dir is required")
	}
	if focus != "" {
		re, err := regexp.Compile(focus)
		if err != nil {
			return nil, fmt.Errorf("invalid --focus: %w", err)
		}
		o.focus = re
	}
	if o.model != "none" && o.model != "claude" {
		return nil, fmt.Errorf("unknown model %q", o.model)
	}
	o.parallel = max(o.parallel, 1)

	return o, nil
}

// group collects the reviews of one test.
type group struct {
	critic.ReportGroup
	failed bool
	mu     sync.Mutex
}

// job is an episode waiting for the model.
type job struct {
	group  *group
	report *critic.Report
	critic *critic.Critic
	key    string
}

func run() error {
	o, err := parseFlags()
	if err != nil {
		return err
	}

	files := o.traces
	if o.tracesDir != "" {
		found, err := findTraces(o.tracesDir)
		if err != nil {
			return err
		}
		files = append(files, found...)
	}

	var model critic.Model
	if o.model == "claude" {
		dir, err := filepath.Abs(o.repoDir)
		if err != nil {
			return err
		}
		model = reconcilecritic.ClaudeCLIModel{Model: o.claudeModel, Dir: dir}
	}
	newCritic := func(inEnvtest bool) *critic.Critic {
		return critic.New(critic.Options{
			Model:            model,
			ContextProviders: []critic.ContextProvider{reconcilecritic.ScyllaDBDatacenterContext(inEnvtest)},
			Render:           critic.RenderOptions{MaxBytes: o.maxTimelineBytes},
			Timeout:          20 * time.Minute,
		})
	}
	critics := map[bool]*critic.Critic{true: newCritic(true), false: newCritic(false)}

	ctx := context.Background()
	groups := map[string]*group{}
	var jobs []job
	for _, path := range files {
		meta, err := reconcilecritic.ReadTraceMeta(path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		inEnvtest := false
		g := &group{}
		if meta != nil {
			name = meta.TestName
			inEnvtest = meta.Source == "envtest"
			g.failed = meta.Failed
		}
		if o.focus != nil && !o.focus.MatchString(name) {
			continue
		}

		if existing, ok := groups[name]; ok {
			g = existing
			g.failed = g.failed || (meta != nil && meta.Failed)
		} else {
			g.Name = name
			groups[name] = g
		}
		g.Notes = append(g.Notes, "Trace: "+path)

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		records, err := reconciletrace.ReadJSONL(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("can't read %q: %w", path, err)
		}

		c := critics[inEnvtest]
		for _, e := range critic.SplitEpisodes(records, 0) {
			report, err := c.Prepare(ctx, &e)
			if err != nil {
				g.Errors = append(g.Errors, err.Error())
				continue
			}
			if report.Signals.Reconciles < o.minReconciles {
				continue
			}
			g.Reports = append(g.Reports, report)

			if model == nil {
				continue
			}
			key := promptKey(o.model+"/"+o.claudeModel, report.Prompt)
			if response, ok := readCachedResponse(o.reviewsDir, key); ok {
				err := report.SetResponse(response)
				if err != nil {
					g.Errors = append(g.Errors, err.Error())
				}
				continue
			}
			jobs = append(jobs, job{group: g, report: report, critic: c, key: key})
		}
	}

	askModel(ctx, o, jobs)

	ordered := make([]*group, 0, len(groups))
	for _, g := range groups {
		if g.failed {
			g.Status = "failed"
		} else {
			g.Status = "passed"
		}
		ordered = append(ordered, g)
	}
	slices.SortFunc(ordered, func(a, b *group) int {
		if a.failed != b.failed {
			if a.failed {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})

	reportGroups := make([]critic.ReportGroup, 0, len(ordered))
	for _, g := range ordered {
		reportGroups = append(reportGroups, g.ReportGroup)
	}

	if o.htmlPath == "" {
		for _, g := range reportGroups {
			fmt.Printf("# %s (%s)\n\n", g.Name, g.Status)
			for _, r := range g.Reports {
				fmt.Println(r.Markdown())
			}
		}
		return nil
	}

	f, err := os.Create(o.htmlPath)
	if err != nil {
		return err
	}
	defer f.Close()

	subtitle := fmt.Sprintf("%d traces", len(files))
	if model != nil {
		subtitle += ", reviewed by " + o.model
		if o.claudeModel != "" {
			subtitle += " (" + o.claudeModel + ")"
		}
	} else {
		subtitle += ", signals only"
	}
	err = critic.WriteHTML(f, critic.HTMLOptions{Title: o.title, Subtitle: subtitle}, reportGroups)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote %s\n", o.htmlPath)

	return nil
}

// askModel runs the model calls, o.parallel at a time.
func askModel(ctx context.Context, o *options, jobs []job) {
	if len(jobs) == 0 {
		return
	}

	work := make(chan int)
	var wg sync.WaitGroup
	var done sync.Mutex
	finished := 0
	for range o.parallel {
		wg.Go(func() {
			for i := range work {
				j := jobs[i]
				start := time.Now()
				err := j.critic.Ask(ctx, j.report)
				if err == nil {
					writeCachedResponse(o.reviewsDir, j.key, j.report.Response)
				}

				j.group.mu.Lock()
				if err != nil {
					j.group.Errors = append(j.group.Errors, fmt.Sprintf("%s: %v", j.report.Request, err))
				}
				j.group.mu.Unlock()

				done.Lock()
				finished++
				fmt.Fprintf(os.Stderr, "[%d/%d] %s %s: %d findings in %s%s\n", finished, len(jobs), j.group.Name, j.report.Request, len(j.report.Findings), time.Since(start).Round(time.Second), errSuffix(err))
				done.Unlock()
			}
		})
	}
	for i := range jobs {
		work <- i
	}
	close(work)
	wg.Wait()
}

func errSuffix(err error) string {
	if err == nil {
		return ""
	}

	return " (failed: " + err.Error() + ")"
}

func findTraces(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	slices.Sort(files)

	return files, err
}

func promptKey(model string, p *critic.Prompt) string {
	h := sha256.New()
	for _, part := range []string{model, p.System, p.User} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil))
}

func readCachedResponse(dir, key string) (string, bool) {
	if dir == "" {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(dir, key+".txt"))
	if err != nil {
		return "", false
	}

	return string(b), true
}

func writeCachedResponse(dir, key, response string) {
	if dir == "" {
		return
	}
	err := os.MkdirAll(dir, 0o755)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, key+".txt"), []byte(response), 0o644)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't keep the answer: %v\n", err)
	}
}
