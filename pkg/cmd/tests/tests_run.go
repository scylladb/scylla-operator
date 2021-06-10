package tests

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/onsi/ginkgo"
	gconfig "github.com/onsi/ginkgo/config"
	greporters "github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	// Include suites
	_ "github.com/scylladb/scylla-operator/test/e2e"
)

type RunOptions struct {
	genericclioptions.IOStreams
	genericclioptions.ClientConfig
	TestFrameworkOptions

	Quiet            bool
	ShowProgress     bool
	FlakeAttempts    int
	FailFast         bool
	FocusStrings     []string
	SkipStrings      []string
	SkipMeasurements bool
	RandomSeed       int64
	DryRun           bool
	Color            bool
}

func NewRunOptions(streams genericclioptions.IOStreams) *RunOptions {
	return &RunOptions{
		ClientConfig:         genericclioptions.NewClientConfig("scylla-operator-e2e"),
		TestFrameworkOptions: NewTestFrameworkOptions(),

		Quiet:            false,
		ShowProgress:     true,
		FlakeAttempts:    0,
		FailFast:         false,
		FocusStrings:     []string{},
		SkipStrings:      []string{},
		SkipMeasurements: false,
		RandomSeed:       time.Now().Unix(),
		DryRun:           false,
		Color:            true,
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

			err = o.Run(streams, cmd.Name())
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

	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "", o.Quiet, "Reduces the tests output.")
	cmd.Flags().BoolVarP(&o.ShowProgress, "progress", "", o.ShowProgress, "Shows progress during test run.")
	cmd.Flags().IntVarP(&o.FlakeAttempts, "flake-attempts", "", o.FlakeAttempts, "Retries a failed test up to N times. If it succeeds at least once the test will be considered a success.")
	cmd.Flags().BoolVarP(&o.FailFast, "fail-fast", "", o.FailFast, "Stops execution after first failed test.")
	cmd.Flags().StringSliceVarP(&o.FocusStrings, "focus", "", o.FocusStrings, "Regex to select a subset of tests to run.")
	cmd.Flags().StringSliceVarP(&o.SkipStrings, "skip", "", o.SkipStrings, "Regex to select a subset of tests to skip.")
	cmd.Flags().BoolVarP(&o.SkipMeasurements, "skip-measurements", "", o.SkipMeasurements, "Skips measurements.")
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

func (o *RunOptions) Run(streams genericclioptions.IOStreams, commandName string) error {
	klog.Infof("%s version %q", commandName, version.Get())
	klog.Infof("loglevel is set to %q", cmdutil.GetLoglevel())

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

	gconfig.GinkgoConfig.EmitSpecProgress = o.ShowProgress
	gconfig.GinkgoConfig.FlakeAttempts = o.FlakeAttempts + 1
	if gconfig.GinkgoConfig.FlakeAttempts > 1 {
		klog.Infof("Flakes will be retried up to %d times.", gconfig.GinkgoConfig.FlakeAttempts-1)
	}
	gconfig.GinkgoConfig.FailFast = o.FailFast
	gconfig.GinkgoConfig.FocusStrings = o.FocusStrings
	gconfig.GinkgoConfig.SkipStrings = o.SkipStrings
	gconfig.GinkgoConfig.SkipMeasurements = o.SkipMeasurements
	gconfig.GinkgoConfig.RandomSeed = o.RandomSeed
	gconfig.GinkgoConfig.DryRun = o.DryRun
	gconfig.DefaultReporterConfig.Verbose = !o.Quiet
	gconfig.DefaultReporterConfig.NoColor = !o.Color

	// Not configurable. We are opinionated about these.

	// Prevents growing a dependency.
	gconfig.GinkgoConfig.RandomizeAllSpecs = true
	// Better context and it's required for nested assertions. Offset doesn't really solve it as it omits the nested function line.
	gconfig.DefaultReporterConfig.FullTrace = true
	gconfig.DefaultReporterConfig.SlowSpecThreshold = 60 // seconds

	gomega.RegisterFailHandler(ginkgo.Fail)

	var r []ginkgo.Reporter
	if len(o.ArtifactsDir) > 0 {
		r = append(r, greporters.NewJUnitReporter(path.Join(o.ArtifactsDir, fmt.Sprintf("e2e_%02d.xml", gconfig.GinkgoConfig.ParallelNode))))
	}

	passed := ginkgo.RunSpecsWithDefaultAndCustomReporters(&fakeT{}, suite, r)
	if !passed {
		return fmt.Errorf("test suite %q failed", suite)
	}

	return nil
}
