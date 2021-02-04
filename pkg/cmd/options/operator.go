package options

import (
	"os"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
)

// Singleton
var operatorOpts = &OperatorOptions{
	CommonOptions: GetCommonOptions(),
}

type OperatorOptions struct {
	*CommonOptions
	Image                  string
	EnableAdmissionWebhook bool
	RackFromNode           bool
}

func GetOperatorOptions() *OperatorOptions {
	return operatorOpts
}

func (o *OperatorOptions) AddFlags(cmd *cobra.Command) {
	o.CommonOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.Image, "image", "", "image of the operator used")
	cmd.Flags().BoolVar(&o.EnableAdmissionWebhook, "enable-admission-webhook", true, "enable the admission webhook")
	cmd.Flags().BoolVar(&o.RackFromNode, "rack-from-node", os.Getenv(naming.EnvVarRackFromNode) == "true", "Use node labels instead of pod labels for rack")
}

func (o *OperatorOptions) Validate() error {

	if o.Image == "" && o.CommonOptions.Validate() != nil {
		return errors.New("image not set - you must set either image or namespace and name")
	}
	return nil
}
