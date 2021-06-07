package options

import (
	"os"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
)

// Singleton
var sidecarOpts = &sidecarOptions{
	CommonOptions: GetCommonOptions(),
}

type sidecarOptions struct {
	*CommonOptions
	CPU               string
	RunPerftune       bool
	DisableWriteCache bool
}

func GetSidecarOptions() *sidecarOptions {
	return sidecarOpts
}

func (o *sidecarOptions) AddFlags(cmd *cobra.Command) {
	o.CommonOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.CPU, "cpu", os.Getenv(naming.EnvVarCPU), "number of cpus to use")
	cmd.Flags().BoolVar(&o.RunPerftune, "run-perftune", false, "run perftune script to optimize node")
	cmd.Flags().BoolVar(&o.DisableWriteCache, "disable-write-cache", false, "disable disk write cache")
}

func (o *sidecarOptions) Validate() error {
	if err := o.CommonOptions.Validate(); err != nil {
		return errors.WithStack(err)
	}
	if o.CPU == "" {
		return errors.New("cpu not set")
	}
	return nil
}
