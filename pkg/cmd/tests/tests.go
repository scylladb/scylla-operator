package tests

import (
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_TESTS_"
)

func NewTestsCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Long: templates.LongDesc(`
		Scylla operator tests

		This command verifies behavior of an scylla-operator by running remote tests on a Kubernetes cluster.
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
		},
	}

	cmd.AddCommand(NewRunCommand(streams))

	// TODO: wrap help func for the root command and every subcommand to add a line about automatic env vars and the prefix.

	cmdutil.InstallKlog(cmd)

	return cmd
}
