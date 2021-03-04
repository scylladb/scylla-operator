package options

import (
	"github.com/spf13/cobra"
)

// Singleton
var managerOpts = &managerOptions{
	CommonOptions: GetCommonOptions(),
}

type managerOptions struct {
	*CommonOptions
}

func GetManagerOptions() *managerOptions {
	return managerOpts
}

func (o *managerOptions) AddFlags(cmd *cobra.Command) {
	o.CommonOptions.AddFlags(cmd)
}

func (o *managerOptions) Validate() error {
	return nil
}
