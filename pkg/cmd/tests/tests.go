package tests

import (
	"fmt"

	versioncmd "github.com/scylladb/scylla-operator/pkg/cmd/version"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	ginkgotest "github.com/scylladb/scylla-operator/pkg/test/ginkgo"
	_ "github.com/scylladb/scylla-operator/test/e2e" // Include suites
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_TESTS_"
)

var Suites = ginkgotest.TestSuites{
	{
		Name: "all",
		Description: templates.LongDesc(`
		Runs all tests.`,
		),
		DefaultParallelism: 42,
	},
	{
		Name: "scylla-operator/conformance/parallel",
		Description: templates.LongDesc(`
		Tests that can be run in parallel.
		`),
		LabelFilter: fmt.Sprintf(
			"!%s && !%s && !%s && !%s",
			framework.SerialLabelName,
			framework.MultiDatacenterLabelName,
			framework.SupportedOnlyOnOpenShiftLabelName,
			framework.IPv6LabelName,
		),
		DefaultParallelism: 42,
	},
	{
		Name: "scylla-operator/conformance/parallel/openshift",
		Description: templates.LongDesc(`
		Tests that can be run in parallel on an OpenShift cluster.
		`),
		LabelFilter: fmt.Sprintf(
			"!%s && !%s && !%s && !%s",
			framework.SerialLabelName,
			framework.MultiDatacenterLabelName,
			framework.NotSupportedOnOpenShiftLabelName,
			framework.IPv6LabelName,
		),
		DefaultParallelism: 42,
	},
	{
		Name: "scylla-operator/conformance/serial",
		Description: templates.LongDesc(`
		Tests that must be run serially.
		`),
		LabelFilter: fmt.Sprintf(
			"%s && !%s",
			framework.SerialLabelName,
			framework.SupportedOnlyOnOpenShiftLabelName,
		),
		DefaultParallelism: 1,
	},
	{
		Name: "scylla-operator/conformance/multi-datacenter-parallel",
		Description: templates.LongDesc(`
		Tests for multi-datacenter setups that can be run in parallel.
		`),
		LabelFilter: fmt.Sprintf(
			"%s && !%s",
			framework.MultiDatacenterLabelName,
			framework.SupportedOnlyOnOpenShiftLabelName,
		),
		DefaultParallelism: 42,
	},
	{
		Name: "scylla-operator/conformance/parallel-ipv6",
		Description: templates.LongDesc(`
		Tests that ensure Scylla Operator is working properly with IPv6 and dual-stack networking.
		`),
		LabelFilter:        fmt.Sprintf("%s", framework.IPv6LabelName),
		DefaultParallelism: 42,
	},
}

func NewTestsCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use: "scylla-operator-tests",
		Long: templates.LongDesc(`
		Scylla operator tests

		This command verifies behavior of an scylla-operator by running remote tests on a Kubernetes cluster.
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := maxprocs.Set(maxprocs.Logger(func(format string, v ...interface{}) {
				klog.V(2).Infof(format, v)
			}))
			if err != nil {
				return fmt.Errorf("can't set maxproc: %w", err)
			}

			err = cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
			if err != nil {
				return fmt.Errorf("can't read flags from env: %w", err)
			}

			return nil
		},

		SilenceUsage:  true,
		SilenceErrors: true,
	}

	userAgent := "scylla-operator-e2e"
	cmd.AddCommand(versioncmd.NewCmd(streams))
	cmd.AddCommand(NewRunCommand(streams, Suites, userAgent))

	// TODO: wrap help func for the root command and every subcommand to add a line about automatic env vars and the prefix.

	cmdutil.InstallKlog(cmd)

	utilfeature.DefaultMutableFeatureGate.AddFlag(cmd.PersistentFlags())

	return cmd
}
