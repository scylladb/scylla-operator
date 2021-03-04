package genericclioptions

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// IOStreams is a structure containing all standard streams.
type IOStreams struct {
	// In think, os.Stdin
	In io.Reader
	// Out think, os.Stdout
	Out io.Writer
	// ErrOut think, os.Stderr
	ErrOut io.Writer
}

type ClientConfig struct {
	Kubeconfig string
	RestConfig *restclient.Config
}

func NewClientConfig() ClientConfig {
	return ClientConfig{
		Kubeconfig: "",
		RestConfig: nil,
	}
}

func (cc *ClientConfig) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&cc.Kubeconfig, "kubeconfig", "", cc.Kubeconfig, "Path to the kubeconfig file")
}

func (cc *ClientConfig) Validate() error {
	return nil
}

func (cc *ClientConfig) Complete() error {
	var err error

	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	// Use explicit kubeconfig if set.
	loader.ExplicitPath = cc.Kubeconfig
	cc.RestConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("can't create client config: %w", err)
	}

	return nil
}
