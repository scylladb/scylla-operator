package serveprobes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type ProbeServerOptions struct {
	Address string
	Port    uint16

	handler http.Handler
}

func NewProbeServerOptions(streams genericclioptions.IOStreams, port uint16, handler http.Handler) *ProbeServerOptions {
	return &ProbeServerOptions{
		Address: "",
		Port:    port,
		handler: handler,
	}
}
func (o *ProbeServerOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Address, "address", "", o.Address, "Listen address for the server.")
	cmd.Flags().Uint16VarP(&o.Port, "port", "", o.Port, "Port to use for the server.")
}

func (o *ProbeServerOptions) Validate(args []string) error {
	var errs []error

	return apierrors.NewAggregate(errs)
}

func (o *ProbeServerOptions) Complete(args []string) error {
	return nil
}

func (o *ProbeServerOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.RunCtx(ctx, originalStreams, cmd)
}

func (o *ProbeServerOptions) RunCtx(ctx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", o.Address, o.Port),
		Handler: o.handler,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("can't create tcp listener on addess %q: %w", server.Addr, err)
	}

	resolvedListenAddr := listener.Addr().String()
	klog.InfoS("Starting probe server", "Address", resolvedListenAddr)
	defer klog.InfoS("Probe server shut down")

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		klog.Infof("Shutting down probe server.")
		err := server.Shutdown(context.Background())
		if err != nil {
			klog.ErrorS(err, "can't shutdown the server")
		}
	}()

	err = server.Serve(listener)
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
