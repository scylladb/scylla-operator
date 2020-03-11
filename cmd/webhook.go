package main

import (
	"context"

	"github.com/scylladb/go-log"

	"github.com/scylladb/scylla-operator/pkg/apis"
	"github.com/scylladb/scylla-operator/pkg/webhook"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func newWebhookCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	var webhookCmd = &cobra.Command{
		Use:   "webhook",
		Short: "Start the scylla webhook.",
		Long:  `sidecar is for starting the scylla admission webhook.`,
		Run: func(cmd *cobra.Command, args []string) {
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
			if err := webhook.AddToManager(mgr); err != nil {
				logger.Fatal(ctx, "unable to add web hook to manager", "error", err)
			}

			logger.Info(ctx, "Starting the webhook...")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Fatal(ctx, "error launching manager", "mode", "webhook", "error", err)
			}
		},
	}
	return webhookCmd
}
