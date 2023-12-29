package generateapireference

import (
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvVarPrefix = "GENERATE_API_REFERENCE_"
)

func NewGenerateAPIReferenceCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGenerateAPIRefsOptions()

	cmd := &cobra.Command{
		Use: "generate-api-reference crd [crd...]",
		Long: templates.LongDesc(`
		Generate API reference

		This command generates API reference from CRDs based on user templates.
		`),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv(naming.OperatorEnvVarPrefix, cmd)
		},
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

	cmdutil.InstallKlog(cmd)

	o.AddFlags(cmd)

	return cmd
}
