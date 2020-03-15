package options

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Singleton
var operatorOpts = &operatorOptions{
	commonOptions: GetCommonOptions(),
}

type operatorOptions struct {
	*commonOptions
	Image                    string
	EnableAdmissionWebhook   bool
	EnableScyllaAgentSidecar bool
}

func GetOperatorOptions() *operatorOptions {
	return operatorOpts
}

func (o *operatorOptions) AddFlags(cmd *cobra.Command) {
	o.commonOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.Image, "image", "", "image of the operator used")
	cmd.Flags().BoolVar(&o.EnableAdmissionWebhook, "enable-admission-webhook", true, "enable the admission webhook")
	cmd.Flags().BoolVar(&o.EnableScyllaAgentSidecar, "enable-scylla-agent-sidecar", false, "inject the scylla-agent sidecar")
}

func (o *operatorOptions) Validate() error {

	if o.Image == "" && o.commonOptions.Validate() != nil {
		return errors.New("image not set - you must set either image or namespace and name")
	}
	return nil
}
