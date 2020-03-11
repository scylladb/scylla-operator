package main

import (
	"context"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/cmd/options"
	"github.com/scylladb/scylla-operator/pkg/apis"
	"github.com/scylladb/scylla-operator/pkg/controller"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
			logger.Info(ctx, "sidecar started", "options", opts)

			// Set log level
			if err := level.UnmarshalText([]byte(opts.LogLevel)); err != nil {
				logger.Error(ctx, "unable to change log level",
					"level", opts.LogLevel, "error", err)
			}

			// Get a config to talk to the apiserver
			cfg, err := config.GetConfig()
			if err != nil {
				logger.Fatal(ctx, "unable to load config", "error", err)
			}

			// Create a new manager to provide shared dependencies and start components
			mgr, err := manager.New(cfg, manager.Options{})
			if err != nil {
				logger.Fatal(ctx, "unable to create manager", "error", err)
			}

			logger.Info(ctx, "Registering Components.")

			// Setup Scheme for all resources
			if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
				logger.Fatal(ctx, "unable to add schemes to api", "error", err)
			}

			// Setup all Controllers
			if err := controller.UseSidecarControllers(mgr, logger); err != nil {
				logger.Fatal(ctx, "unable to add sidecar controllers to manager", "error", err)
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
