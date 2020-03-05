package main

import (
	"context"
	"fmt"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/cmd/options"
	"github.com/scylladb/scylla-operator/pkg/apis"
	"github.com/scylladb/scylla-operator/pkg/controller"
	"github.com/scylladb/scylla-operator/pkg/webhook"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func newOperatorCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	var operatorCmd = &cobra.Command{
		Use:   "operator",
		Short: "Start the scylla operator.",
		Long:  `operator is for starting the scylla operator. It is meant to run as a standalone binary.`,
		Run: func(cmd *cobra.Command, args []string) {
			opts := options.GetOperatorOptions()
			if err := opts.Validate(); err != nil {
				logger.Fatal(ctx, "invalid options", "error", err)
			}
			logger.Info(ctx, "Operator started", "options", opts)

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

			// Create a new Cmd to provide shared dependencies and start components
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
			if err := controller.UseOperatorControllers(mgr, logger); err != nil {
				logger.Fatal(ctx, "unable to add sidecar controllers to manager", "error", err)
			}

			// Enable webhook if requested
			fmt.Printf("WEBHOOK ENABLE=%v\n", opts.EnableAdmissionWebhook)
			if opts.EnableAdmissionWebhook {
				if err := webhook.AddToManager(mgr); err != nil {
					logger.Fatal(ctx, "unable to add web hook to manager", "error", err)
				}
			}

			logger.Info(ctx, "Starting the operator...")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Fatal(ctx, "error launching manager", "error", err)
			}
		},
	}
	options.GetOperatorOptions().AddFlags(operatorCmd)
	return operatorCmd
}
