package helpers

import (
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "SCYLLA_OPERATOR_HELPERS_"
)

func NewHelpersCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Long: templates.LongDesc(`
		Scylla operator helpers

		This command implement various utilities to manage this project.
		`),

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(EnvVarPrefix, cmd)
		},
	}

	cmd.AddCommand(NewGenerateOLMBundleCommand(streams))

	cmdutil.InstallKlog(cmd)

	return cmd
}
