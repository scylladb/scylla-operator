package main

import (
	"context"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/sidecar"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func newSidecarCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	var sidecarCmd = &cobra.Command{
		Use:   "sidecar",
		Short: "Start the scylla sidecar.",
		Long:  `sidecar is for starting the scylla sidecar. It is meant to run as a standalone binary.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Validate the cmd flags
			opts := options.GetSidecarOptions()
			if err := opts.Validate(); err != nil {
				logger.Fatal(ctx, "sidecar options", "error", err)
			}
			v := version.Get()
			logger.Info(ctx, "sidecar started", "version", v.GitVersion, "build_date", v.BuildDate,
				"commit", v.GitCommit, "go_version", v.GoVersion, "options", opts)

			// Set log level
			if err := level.UnmarshalText([]byte(opts.LogLevel)); err != nil {
				logger.Error(ctx, "unable to change log level",
					"level", opts.LogLevel, "error", err)
			}

			// Create a new Cmd to provide shared dependencies and start components
			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
				Scheme:             scheme,
				MetricsBindAddress: "0", // disable metrics
			})
			if err != nil {
				logger.Fatal(ctx, "unable to create manager", "error", err)
			}

			logger.Info(ctx, "Registering Components.")

			sc, err := sidecar.New(ctx, mgr, logger.Named("member_controller"))
			if err != nil {
				logger.Fatal(ctx, "unable to create sidecar controller", "error", err)
			}
			if err := sc.SetupWithManager(mgr); err != nil {
				logger.Fatal(ctx, "unable to setup sidecar manager", "error", err)
			}

			logger.Info(ctx, "Starting the sidecar...")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Fatal(ctx, "error launching manager", "mode", "sidecar", "error", err)
			}
		},
	}
	options.GetSidecarOptions().AddFlags(sidecarCmd)
	return sidecarCmd
}
