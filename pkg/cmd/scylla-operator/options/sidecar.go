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
	CPU string
}

func GetSidecarOptions() *sidecarOptions {
	return sidecarOpts
}

func (o *sidecarOptions) AddFlags(cmd *cobra.Command) {
	o.CommonOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.CPU, "cpu", os.Getenv(naming.EnvVarCPU), "number of cpus to use")
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
