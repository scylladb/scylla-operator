package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/formatter"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	gomegaformat "github.com/onsi/gomega/format"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	ginkgotest "github.com/scylladb/scylla-operator/pkg/test/ginkgo"
	"github.com/scylladb/scylla-operator/pkg/thirdparty/github.com/onsi/ginkgo/v2/exposedinternal/parallel_support"
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

const (
	parallelShardFlagKey            = "parallel-shard"
	parallelServerAddressFlagKey    = "parallel-server-address"
	ginkgoOutputInterceptorModeNone = "none"
)

var suites = ginkgotest.TestSuites{
	{
		Name: "all",
		Description: templates.LongDesc(`
		Runs all tests.
		`),
		DefaultParallelism: 30,
	},
	{
		Name: "scylla-operator/conformance/parallel",
		Description: templates.LongDesc(`
		Tests that ensure an Scylla Operator is working properly.
		`),
		LabelFilter:        fmt.Sprintf("!%s", framework.SerialLabelName),
		DefaultParallelism: 30,
	},
	{
		Name: "scylla-operator/conformance/serial",
		Description: templates.LongDesc(`
		Tests that ensure an Scylla Operator is working properly.
		`),
		LabelFilter:        fmt.Sprintf("%s", framework.SerialLabelName),
		DefaultParallelism: 1,
	},
}

type RunOptions struct {
	genericclioptions.IOStreams
	genericclioptions.ClientConfig
	TestFrameworkOptions

	Timeout                time.Duration
	Quiet                  bool
	ShowProgress           bool
	FlakeAttempts          int
	FailFast               bool
	LabelFilter            string
	FocusStrings           []string
	SkipStrings            []string
	RandomSeed             int64
	DryRun                 bool
	Color                  bool
	OverrideIngressAddress string
	Parallelism            int
	ParallelShard          int
	ParallelServerAddress  string
	ParallelLogLevel       int32

	SelectedSuite *ginkgotest.TestSuite
}

func NewRunOptions(streams genericclioptions.IOStreams) *RunOptions {
	return &RunOptions{
		ClientConfig:         genericclioptions.NewClientConfig("scylla-operator-e2e"),
		TestFrameworkOptions: NewTestFrameworkOptions(),

		Timeout:               24 * time.Hour,
		Quiet:                 false,
		ShowProgress:          true,
		FlakeAttempts:         0,
		FailFast:              false,
		LabelFilter:           "",
		FocusStrings:          []string{},
		SkipStrings:           []string{},
		RandomSeed:            time.Now().Unix(),
		DryRun:                false,
		Color:                 true,
		Parallelism:           0,
		ParallelShard:         0,
		ParallelServerAddress: "",
		ParallelLogLevel:      0,
	}
}

func NewRunCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRunOptions(streams)

	cmd := &cobra.Command{
		Use: "run SUITE_NAME",
		Long: templates.LongDesc(`
		Runs a test suite
		`),
		ValidArgs: suites.Names(),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate(args)
			if err != nil {
				return err
			}

			err = o.Complete(args)
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
	cmd.Flags().BoolVarP(&o.ShowProgress, "progress", "", o.ShowProgress, "Shows progress during test run. Only applies to serial execution.")
	cmd.Flags().IntVarP(&o.FlakeAttempts, "flake-attempts", "", o.FlakeAttempts, "Retries a failed test up to N times. If it succeeds at least once the test will be considered a success.")
	cmd.Flags().BoolVarP(&o.FailFast, "fail-fast", "", o.FailFast, "Stops execution after first failed test.")
	cmd.Flags().StringVarP(&o.LabelFilter, "label-filter", "", o.LabelFilter, "Ginkgo label filter.")
	cmd.Flags().StringSliceVarP(&o.FocusStrings, "focus", "", o.FocusStrings, "Regex to select a subset of tests to run.")
	cmd.Flags().StringSliceVarP(&o.SkipStrings, "skip", "", o.SkipStrings, "Regex to select a subset of tests to skip.")
	cmd.Flags().Int64VarP(&o.RandomSeed, "random-seed", "", o.RandomSeed, "Seed for the test suite.")
	cmd.Flags().BoolVarP(&o.DryRun, "dry-run", "", o.DryRun, "Doesn't execute the tests, only prints. Limited to serial execution.")
	cmd.Flags().BoolVarP(&o.Color, "color", "", o.Color, "Colors the output.")
	cmd.Flags().StringVarP(&o.OverrideIngressAddress, "override-ingress-address", "", o.OverrideIngressAddress, "This flag will override destination address when sending testing data to applications behind ingresses.")
	cmd.Flags().IntVarP(&o.Parallelism, "parallelism", "", o.Parallelism, "Determines how many workers are going to run in parallel. If not specified or if zero, the default parallelism for the suite will be chosen.")
	cmd.Flags().Int32Var(&o.ParallelLogLevel, "parallel-loglevel", o.ParallelLogLevel, "Set the level of log output for parallel processes (0-10).")
	cmd.Flags().IntVarP(&o.ParallelShard, parallelShardFlagKey, "", o.ParallelShard, "")
	cmd.Flags().MarkHidden(parallelShardFlagKey)
	cmd.Flags().StringVarP(&o.ParallelServerAddress, parallelServerAddressFlagKey, "", o.ParallelServerAddress, "")
	cmd.Flags().MarkHidden(parallelServerAddressFlagKey)

	return cmd
}

func (o *RunOptions) Validate(args []string) error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.TestFrameworkOptions.Validate())

	if o.FlakeAttempts < 0 {
		errs = append(errs, fmt.Errorf("flake attempts can't be negative"))
	}

	if o.Timeout == 0 {
		errs = append(errs, fmt.Errorf("timeout can't be zero"))
	}

	if o.Parallelism < 0 {
		errs = append(errs, fmt.Errorf("parallelism can't be negative"))
	}

	if o.Parallelism > 1 && o.DryRun {
		errs = append(errs, fmt.Errorf("dry-run isn't supported in parallel runs"))
	}

	if o.ParallelShard > 0 && len(o.ParallelServerAddress) == 0 {
		errs = append(errs, fmt.Errorf("there has to be --%q given when specifying parallel shard", parallelServerAddressFlagKey))
	}

	switch len(args) {
	case 0:
		errs = append(errs, fmt.Errorf(
			"you have to specify at least one suite from [%s]",
			strings.Join(suites.Names(), ", ")),
		)

	case 1:
		suiteName := args[0]
		o.SelectedSuite = suites.Find(suiteName)
		if o.SelectedSuite == nil {
			errs = append(errs, fmt.Errorf("suite %q doesn't exist", suiteName))
		}

	default:
		errs = append(errs, fmt.Errorf("can't select more then 1 suite"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *RunOptions) Complete(args []string) error {
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
	klog.V(1).Infof("%q version %q", cmd.CommandPath(), version.Get())
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
		RestConfig:             o.RestConfig,
		ArtifactsDir:           o.ArtifactsDir,
		DeleteTestingNSPolicy:  o.DeleteTestingNSPolicy,
		OverrideIngressAddress: o.OverrideIngressAddress,
	}

	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()

	suiteConfig.Timeout = o.Timeout
	suiteConfig.EmitSpecProgress = o.ShowProgress
	suiteConfig.FlakeAttempts = o.FlakeAttempts + 1
	if suiteConfig.FlakeAttempts > 1 {
		klog.Infof("Flakes will be retried up to %d times.", suiteConfig.FlakeAttempts-1)
	}
	suiteConfig.FailFast = o.FailFast
	suiteConfig.RandomSeed = o.RandomSeed
	suiteConfig.DryRun = o.DryRun
	suiteConfig.OutputInterceptorMode = ginkgoOutputInterceptorModeNone
	reporterConfig.Verbose = !o.Quiet
	reporterConfig.NoColor = !o.Color

	suiteConfig.LabelFilter = o.SelectedSuite.LabelFilter
	if len(o.LabelFilter) != 0 {
		klog.InfoS("Overriding LabelFilter", "From", suiteConfig.LabelFilter, "To", o.LabelFilter)
		suiteConfig.LabelFilter = o.LabelFilter
	}

	suiteConfig.FocusStrings = o.SelectedSuite.FocusStrings
	if len(o.FocusStrings) != 0 {
		klog.InfoS("Overriding FocusStrings", "From", suiteConfig.FocusStrings, "To", o.FocusStrings)
		suiteConfig.FocusStrings = o.FocusStrings
	}

	suiteConfig.SkipStrings = o.SelectedSuite.SkipStrings
	if len(o.SkipStrings) != 0 {
		klog.InfoS("Overriding SkipStrings", "From", suiteConfig.SkipStrings, "To", o.SkipStrings)
		suiteConfig.SkipStrings = o.SkipStrings
	}

	// Not configurable. We are opinionated about these.

	// Prevents growing a dependency.
	suiteConfig.RandomizeAllSpecs = true
	// Better context and it's required for nested assertions. Offset doesn't really solve it as it omits the nested function line.
	reporterConfig.FullTrace = true
	reporterConfig.SlowSpecThreshold = 10 * time.Minute
	gomegaformat.MaxLength = 0
	gomegaformat.MaxDepth = 20
	gomegaformat.TruncatedDiff = false

	gomega.RegisterFailHandler(ginkgo.Fail)

	if len(o.ArtifactsDir) > 0 {
		reporterConfig.JUnitReport = path.Join(o.ArtifactsDir, "e2e.junit.xml")
		reporterConfig.JSONReport = path.Join(o.ArtifactsDir, "e2e.json")
	}

	suiteConfig.ParallelTotal = o.Parallelism
	if suiteConfig.ParallelTotal == 0 {
		if o.DryRun {
			suiteConfig.ParallelTotal = 1
		} else {
			suiteConfig.ParallelTotal = o.SelectedSuite.DefaultParallelism
		}
	}

	if suiteConfig.ParallelTotal <= 1 || o.ParallelShard != 0 {
		if suiteConfig.ParallelTotal > 1 {
			suiteConfig.ParallelHost = o.ParallelServerAddress
			suiteConfig.ParallelProcess = o.ParallelShard

			ginkgo.GinkgoWriter.TeeTo(os.Stdout)
			defer ginkgo.GinkgoWriter.ClearTeeWriters()
		}

		klog.InfoS("Running specs")
		passed := ginkgo.RunSpecs(&fakeT{}, suite, suiteConfig, reporterConfig)
		if !passed {
			return fmt.Errorf("test suite %q failed", suite)
		}

		return nil
	}

	server, err := parallel_support.NewServer(suiteConfig.ParallelTotal, reporters.NewDefaultReporter(reporterConfig, formatter.ColorableStdOut))
	if err != nil {
		return fmt.Errorf("can't create parallel spec server: %w", err)
	}
	server.Start()
	defer server.Close()

	commonArgs := make([]string, 0, 2+len(os.Args))
	if len(os.Args) > 1 {
		commonArgs = append(commonArgs, os.Args[1:]...)
	}
	commonArgs = append(commonArgs, fmt.Sprintf("--%s=%v", parallelServerAddressFlagKey, server.Address()))
	commonArgs = append(commonArgs, fmt.Sprintf("--%s=%v", cmdutil.FlagLogLevelKey, o.ParallelLogLevel))

	type cmdEntry struct {
		id  int
		cmd *exec.Cmd
		out *bytes.Buffer
	}
	cmdEntries := make([]*cmdEntry, 0, suiteConfig.ParallelTotal)
	for i := 1; i <= suiteConfig.ParallelTotal; i++ {
		suiteConfig := suiteConfig
		suiteConfig.ParallelProcess = i

		args := make([]string, 0, len(commonArgs)+1)
		args = append(args, commonArgs...)
		args = append(args, fmt.Sprintf("--%s=%v", parallelShardFlagKey, i))

		buf := &bytes.Buffer{}
		cmd := exec.CommandContext(ctx, os.Args[0], args...)
		cmd.Stdout = buf
		cmd.Stderr = buf
		cmdEntries = append(cmdEntries, &cmdEntry{
			id:  i,
			out: buf,
			cmd: cmd,
		})
	}

	var errs []error
	for entryIndex := range cmdEntries {
		e := cmdEntries[entryIndex]
		err := e.cmd.Start()
		if err != nil {
			errs = append(errs, fmt.Errorf("can't start command %q with args %v: %w", e.cmd.Path, e.cmd.Args, err))
		}
		klog.V(2).InfoS("Started process", "ID", e.id, "Command", e.cmd.String())
		// RegisterAlive needs to be set so ginkgo worker #1 can detect all the other workers are finished
		// and start serial tests. It needs cmd.Wait to be called first to pick up the state.
		server.RegisterAlive(e.id, func() bool { return e.cmd.ProcessState == nil || !e.cmd.ProcessState.Exited() })
	}
	if len(errs) != 0 {
		return apierrors.NewAggregate(errs)
	}

	// We need to wait for all the processes in parallel so the Alive function can read the status,
	// which is available only after calling Wait().
	var wg sync.WaitGroup
	defer wg.Wait()
	errs = make([]error, len(cmdEntries))
	for entryIndex := range cmdEntries {
		entryIndex := entryIndex
		e := cmdEntries[entryIndex]
		wg.Add(1)
		go func() {
			defer wg.Done()

			klog.V(2).InfoS("Waiting for process", "ID", e.id, "Command", e.cmd.String())
			err := e.cmd.Wait()
			// We'll handle exit codes separately.
			var exitError *exec.ExitError
			if err != nil && !errors.As(err, &exitError) {
				errs[entryIndex] = fmt.Errorf("can't wait for command %q with args %q: %w", e.cmd.Path, e.cmd.Args, err)
			}
			klog.V(2).InfoS("Process finished", "ID", e.id, "Command", e.cmd.String(), "ProcessState", e.cmd.ProcessState.String())
		}()
	}

	wg.Wait()

	err = apierrors.NewAggregate(errs)
	if err != nil {
		return fmt.Errorf("can't wait for processes: %w", err)
	}

	// Aggregate exit code.
	hasProgrammaticFocus := false
	passed := true
	for _, e := range cmdEntries {
		exitCode := e.cmd.ProcessState.ExitCode()
		switch exitCode {
		case 0:
			break
		case types.GINKGO_FOCUS_EXIT_CODE:
			hasProgrammaticFocus = true
		default:
			passed = false
			klog.ErrorS(errors.New("process failed"), "Process failed", "ID", e.id, "Command", e.cmd.String(), "ProcessState", e.cmd.ProcessState.String(), "Logs", e.out.String())
		}
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled")

	case <-server.GetSuiteDone():
		break

	default:
		return fmt.Errorf("all processes have finished but the suite still isn't done")
	}

	if !passed {
		return fmt.Errorf("test suite failed")
	}

	if hasProgrammaticFocus {
		return fmt.Errorf("test suite has programmatic focus")
	}

	return nil
}
