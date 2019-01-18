package options

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Singleton
var controllerOpts = &controllerOptions{
	commonOptions: GetCommonOptions(),
}

type controllerOptions struct {
	*commonOptions
	Image string
}

func GetControllerOptions() *controllerOptions {
	return controllerOpts
}

func (o *controllerOptions) AddFlags(cmd *cobra.Command) {
	o.commonOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.Image, "image", "", "image of the operator used")
}

func (o *controllerOptions) Validate() error {

	if o.Image == "" && o.commonOptions.Validate() != nil {
		return errors.New("image not set - you must set either image or namespace and name")
	}
	return nil
}
