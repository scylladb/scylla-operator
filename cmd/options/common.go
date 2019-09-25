package options

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// Singleton
var commonOpts = &commonOptions{}

type commonOptions struct {
	Name      string
	Namespace string
	LogLevel  string
}

func GetCommonOptions() *commonOptions {
	return commonOpts
}

func (o *commonOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Name, "pod-name", os.Getenv(naming.EnvVarEnvVarPodName), "name of the pod")
	cmd.Flags().StringVar(&o.Namespace, "pod-namespace", os.Getenv(naming.EnvVarPodNamespace), "namespace of the pod")
	cmd.Flags().StringVar(&o.LogLevel,
		"log-level",
		"info",
		fmt.Sprintf("verbosity of the logs. Possible values, in descending order of verbosity: %s, %s, %s, %s", "fatal", "error", "info", "debug"),
	)

}

func (o *commonOptions) Validate() error {
	if o.Name == "" {
		return errors.New("pod-name not set")
	}
	if o.Namespace == "" {
		return errors.New("pod-namespace not set")
	}
	atomic := zap.NewAtomicLevel()
	if err := atomic.UnmarshalText([]byte(o.LogLevel)); err != nil {
		return errors.Wrapf(err, "invalid log level: '%s'", o.LogLevel)
	}
	return nil
}
