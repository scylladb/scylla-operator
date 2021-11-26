package tests

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	// Include suites
	_ "github.com/scylladb/scylla-operator/test/e2e"
)

type RunOptions struct {
	genericclioptions.IOStreams
	genericclioptions.ClientConfig
	TestFrameworkOptions

	Timeout       time.Duration
	Quiet         bool
	ShowProgress  bool
	FlakeAttempts int
	FailFast      bool
	FocusStrings  []string
	SkipStrings   []string
	RandomSeed    int64
	DryRun        bool
	Color         bool
}

func NewRunOptions(streams genericclioptions.IOStreams) *RunOptions {
	return &RunOptions{
		ClientConfig:         genericclioptions.NewClientConfig("scylla-operator-e2e"),
		TestFrameworkOptions: NewTestFrameworkOptions(),

		Timeout:       24 * time.Hour,
		Quiet:         false,
		ShowProgress:  true,
		FlakeAttempts: 0,
		FailFast:      false,
		FocusStrings:  []string{},
		SkipStrings:   []string{},
		RandomSeed:    time.Now().Unix(),
		DryRun:        false,
		Color:         true,
	}
}

func NewRunCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRunOptions(streams)

	cmd := &cobra.Command{
		Use: "run",
		Long: templates.LongDesc(`
		Runs a test suite
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.ClientConfig.AddFlags(cmd)
	o.TestFrameworkOptions.AddFlags(cmd)

	cmd.Flags().DurationVarP(&o.Timeout, "timeout", "", o.Timeout, "If the overall suite(s) duration exceed this value, tests will be terminated.")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "", o.Quiet, "Reduces the tests output.")
	cmd.Flags().BoolVarP(&o.ShowProgress, "progress", "", o.ShowProgress, "Shows progress during test run.")
	cmd.Flags().IntVarP(&o.FlakeAttempts, "flake-attempts", "", o.FlakeAttempts, "Retries a failed test up to N times. If it succeeds at least once the test will be considered a success.")
	cmd.Flags().BoolVarP(&o.FailFast, "fail-fast", "", o.FailFast, "Stops execution after first failed test.")
	cmd.Flags().StringSliceVarP(&o.FocusStrings, "focus", "", o.FocusStrings, "Regex to select a subset of tests to run.")
	cmd.Flags().StringSliceVarP(&o.SkipStrings, "skip", "", o.SkipStrings, "Regex to select a subset of tests to skip.")
	cmd.Flags().Int64VarP(&o.RandomSeed, "random-seed", "", o.RandomSeed, "Seed for the test suite.")
	cmd.Flags().BoolVarP(&o.DryRun, "dry-run", "", o.DryRun, "Doesn't execute the tests, only prints.")
	cmd.Flags().BoolVarP(&o.Color, "color", "", o.Color, "Colors the output.")

	return cmd
}

func (o *RunOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.TestFrameworkOptions.Validate())

	if o.FlakeAttempts < 0 {
		errs = append(errs, fmt.Errorf("flake attempts can't be negative"))
	}

	if o.Timeout == 0 {
		errs = append(errs, fmt.Errorf("timeout can't be zero"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *RunOptions) Complete() error {
	err := o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.TestFrameworkOptions.Complete()
	if err != nil {
		return err
	}

	return nil
}

func (o *RunOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %q", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.run(ctx, streams)
}

type fakeT struct{}

func (*fakeT) Fail() {}

var _ ginkgo.GinkgoTestingT = &fakeT{}

func (o *RunOptions) run(ctx context.Context, streams genericclioptions.IOStreams) error {
	const suite = "Scylla operator E2E tests"

	framework.TestContext = &framework.TestContextType{
		RestConfig:            o.RestConfig,
		ArtifactsDir:          o.ArtifactsDir,
		DeleteTestingNSPolicy: o.DeleteTestingNSPolicy,
	}

	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()

	suiteConfig.Timeout = o.Timeout
	suiteConfig.EmitSpecProgress = o.ShowProgress
	suiteConfig.FlakeAttempts = o.FlakeAttempts + 1
	if suiteConfig.FlakeAttempts > 1 {
		klog.Infof("Flakes will be retried up to %d times.", suiteConfig.FlakeAttempts-1)
	}
	suiteConfig.FailFast = o.FailFast
	suiteConfig.FocusStrings = o.FocusStrings
	suiteConfig.SkipStrings = o.SkipStrings
	suiteConfig.RandomSeed = o.RandomSeed
	suiteConfig.DryRun = o.DryRun
	reporterConfig.Verbose = !o.Quiet
	reporterConfig.NoColor = !o.Color

	// Not configurable. We are opinionated about these.

	// Prevents growing a dependency.
	suiteConfig.RandomizeAllSpecs = true
	// Better context and it's required for nested assertions. Offset doesn't really solve it as it omits the nested function line.
	reporterConfig.FullTrace = true
	reporterConfig.SlowSpecThreshold = 60 // seconds

	gomega.RegisterFailHandler(ginkgo.Fail)

	if len(o.ArtifactsDir) > 0 {
		reporterConfig.JUnitReport = path.Join(o.ArtifactsDir, "e2e.junit.xml")
		reporterConfig.JSONReport = path.Join(o.ArtifactsDir, "e2e.json")
	}

	passed := ginkgo.RunSpecs(&fakeT{}, suite, suiteConfig, reporterConfig)
	if !passed {
		return fmt.Errorf("test suite %q failed", suite)
	}

	return nil
}
