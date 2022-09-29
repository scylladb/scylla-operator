package operator

import (
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

func NewOperatorCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(naming.OperatorEnvVarPrefix, cmd)
		},
	}

	cmd.AddCommand(NewOperatorCmd(streams))
	cmd.AddCommand(NewWebhookCmd(streams))
	cmd.AddCommand(NewSidecarCmd(streams))
	cmd.AddCommand(NewManagerControllerCmd(streams))
	cmd.AddCommand(NewNodeConfigCmd(streams))

	// TODO: wrap help func for the root command and every subcommand to add a line about automatic env vars and the prefix.

	cmdutil.InstallKlog(cmd)

	utilfeature.DefaultMutableFeatureGate.AddFlag(cmd.PersistentFlags())

	return cmd
}
