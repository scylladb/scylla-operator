package options

import (
	"github.com/spf13/cobra"
)

// Singleton
var managerOpts = &managerOptions{
	commonOptions: GetCommonOptions(),
}

type managerOptions struct {
	*commonOptions
}

func GetManagerOptions() *managerOptions {
	return managerOpts
}

func (o *managerOptions) AddFlags(cmd *cobra.Command) {
	o.commonOptions.AddFlags(cmd)
}

func (o *managerOptions) Validate() error {
	return nil
}
