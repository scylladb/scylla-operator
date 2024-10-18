package operator

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/cmd/operator/probeserver"
	versioncmd "github.com/scylladb/scylla-operator/pkg/cmd/version"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
)

func NewOperatorCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
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
	}

	cmd.AddCommand(versioncmd.NewCmd(streams))
	cmd.AddCommand(NewOperatorCmd(streams))
	cmd.AddCommand(NewWebhookCmd(streams, DefaultValidators))
	cmd.AddCommand(NewSidecarCmd(streams))
	cmd.AddCommand(NewManagerControllerCmd(streams))
	cmd.AddCommand(NewNodeSetupCmd(streams))
	cmd.AddCommand(NewCleanupJobCmd(streams))
	cmd.AddCommand(NewGatherCmd(streams))
	cmd.AddCommand(NewMustGatherCmd(streams))
	cmd.AddCommand(probeserver.NewServeProbesCmd(streams))
	cmd.AddCommand(NewIgnitionCmd(streams))
	cmd.AddCommand(NewRlimitsJobCmd(streams))

	// TODO: wrap help func for the root command and every subcommand to add a line about automatic env vars and the prefix.

	cmdutil.InstallKlog(cmd)

	utilfeature.DefaultMutableFeatureGate.AddFlag(cmd.PersistentFlags())

	return cmd
}
