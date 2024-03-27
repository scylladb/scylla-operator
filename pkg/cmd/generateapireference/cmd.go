package generateapireference

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	"k8s.io/klog/v2"
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
			_, err := maxprocs.Set(maxprocs.Logger(func(format string, v ...interface{}) {
				klog.V(2).Infof(format, v)
			}))
			if err != nil {
				return fmt.Errorf("can't set maxproc: %w", err)
			}

			err = cmdutil.ReadFlagsFromEnv(naming.OperatorEnvVarPrefix, cmd)
			if err != nil {
				return fmt.Errorf("can't read flags from env: %w", err)
			}

			return nil
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
