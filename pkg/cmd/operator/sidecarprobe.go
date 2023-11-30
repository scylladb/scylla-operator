package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type SidecarProbeOptions struct {
}

func NewSidecarProbeOptions(streams genericclioptions.IOStreams) *SidecarProbeOptions {
	return &SidecarProbeOptions{}
}

func NewSidecarProbeCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSidecarProbeOptions(streams)

	cmd := &cobra.Command{
		Use:   "sidecar-probe",
		Short: "Run the scylla sidecar probe.",
		Long:  `Run the scylla sidecar probe.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	return cmd
}

func (o *SidecarProbeOptions) Validate() error {
	return nil
}

func (o *SidecarProbeOptions) Complete() error {
	return nil
}

func (o *SidecarProbeOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	// Run probes.
	server := &http.Server{
		Addr:    fmt.Sprintf(":8081"),
		Handler: nil,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()

		klog.InfoS("Starting Prober server")
		defer klog.InfoS("Prober server shut down")

		http.HandleFunc(naming.LivenessProbePath, healthz)
		http.HandleFunc(naming.ReadinessProbePath, readyz)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatal("ListenAndServe failed: %v", err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		klog.InfoS("Shutting down Prober server")
		defer klog.InfoS("Shut down Prober server")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCtxCancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			klog.ErrorS(err, "Shutting down Prober server")
		}
	}()

	<-ctx.Done()

	return nil
}

const (
	localhost    = "localhost"
	probeTimeout = 60 * time.Second
)

func healthz(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyz(w http.ResponseWriter, req *http.Request) {
	ctx, ctxCancel := context.WithTimeout(req.Context(), probeTimeout)
	defer ctxCancel()

	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost()
	if err != nil {
		klog.ErrorS(err, "healthz probe: can't get scylla client")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer scyllaClient.Close()

	gossiperRunning, err := scyllaClient.GossiperIsRunning(ctx, localhost)
	if err != nil {
		klog.ErrorS(err, "readyz probe: can't check whether gossiper is running")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !gossiperRunning {
		klog.V(2).InfoS("readyz probe: gossiper not yet running")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
