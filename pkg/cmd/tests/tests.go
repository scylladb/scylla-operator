package tests

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	ginkgotest "github.com/scylladb/scylla-operator/pkg/test/ginkgo"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubectl/pkg/util/templates"

	// Include suites
	_ "github.com/scylladb/scylla-operator/test/e2e"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_TESTS_"
)

var Suites = ginkgotest.TestSuites{
	{
		Name: "scylla-operator/conformance/parallel",
		Description: templates.LongDesc(`
		Tests that ensure an Scylla Operator is working properly.
		`),
		LabelFilter:        fmt.Sprintf("!%s", framework.SerialLabelName),
		DefaultParallelism: 42,
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

func NewTestsCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use: "scylla-operator-tests",
		Long: templates.LongDesc(`
		Scylla operator tests

		This command verifies behavior of an scylla-operator by running remote tests on a Kubernetes cluster.
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
		},

		SilenceUsage:  true,
		SilenceErrors: true,
	}

	userAgent := "scylla-operator-e2e"
	cmd.AddCommand(NewRunCommand(streams, Suites, NewTestFrameworkOptions(userAgent)))

	// TODO: wrap help func for the root command and every subcommand to add a line about automatic env vars and the prefix.

	cmdutil.InstallKlog(cmd)

	utilfeature.DefaultMutableFeatureGate.AddFlag(cmd.PersistentFlags())

	return cmd
}
