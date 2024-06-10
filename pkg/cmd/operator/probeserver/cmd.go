package probeserver

import (
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/spf13/cobra"
)

func NewServeProbesCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "serve-probes",
		Short:  "Internal command used by Scylla Operator to interface Kubernetes probes and ScyllaDB internal status.",
		Hidden: true,
	}

	cmd.AddCommand(NewScyllaDBAPIStatusCmd(streams))

	return cmd
}
