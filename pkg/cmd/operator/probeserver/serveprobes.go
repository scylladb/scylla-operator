package probeserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type ServeProbesOptions struct {
	Address string
	Port    uint16

	handler http.Handler
}

func NewServeProbesOptions(streams genericclioptions.IOStreams, port uint16, handler http.Handler) *ServeProbesOptions {
	return &ServeProbesOptions{
		Address: "",
		Port:    port,
		handler: handler,
	}
}
func (o *ServeProbesOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Address, "address", "", o.Address, "Listen address for the server.")
	cmd.Flags().Uint16VarP(&o.Port, "port", "", o.Port, "Port to use for the server.")
}

func (o *ServeProbesOptions) Validate(args []string) error {
	var errs []error

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *ServeProbesOptions) Complete(args []string) error {
	return nil
}

func (o *ServeProbesOptions) Run(originalStreams genericclioptions.IOStreams, cmd *cobra.Command) (returnErr error) {
	cmdutil.LogCommandStarting(cmd)
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	return o.Execute(ctx, originalStreams, cmd)
}

func (o *ServeProbesOptions) Execute(ctx context.Context, originalStreams genericclioptions.IOStreams, cmd *cobra.Command) error {
	server := &http.Server{
		Addr:    net.JoinHostPort(o.Address, strconv.Itoa(int(o.Port))),
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
		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCtxCancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			klog.ErrorS(err, "can't shut down the server")
		}
	}()

	err = server.Serve(listener)
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
