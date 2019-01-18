package options

import (
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
	"os"
)

// Singleton
var sidecarOpts = &sidecarOptions{
	commonOptions: GetCommonOptions(),
}

type sidecarOptions struct {
	*commonOptions
	CPU    string
	Memory string
}

func GetSidecarOptions() *sidecarOptions {
	return sidecarOpts
}

func (o *sidecarOptions) AddFlags(cmd *cobra.Command) {
	o.commonOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.CPU, "cpu", os.Getenv(naming.EnvVarCPU), "number of cpus to use")
	cmd.Flags().StringVar(&o.Memory, "memory", os.Getenv(naming.EnvVarMemory), "amount of memory to use")
}

func (o *sidecarOptions) Validate() error {
	if err := o.commonOptions.Validate(); err != nil {
		return errors.WithStack(err)
	}
	if o.CPU == "" {
		return errors.New("cpu not set")
	}
	if o.Memory == "" {
		return errors.New("memory not set")
	}
	return nil
}
