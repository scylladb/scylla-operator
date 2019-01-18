package main

import (
	"github.com/scylladb/scylla-operator/pkg/apis"
	"github.com/scylladb/scylla-operator/pkg/webhook"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func newWebhookCmd() *cobra.Command {
	var webhookCmd = &cobra.Command{
		Use:   "webhook",
		Short: "Start the scylla webhook.",
		Long:  `sidecar is for starting the scylla admission webhook.`,
		Run:   startWebhook,
	}
	return webhookCmd
}

func startWebhook(cmd *cobra.Command, args []string) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("%+v", err)
	}

	// Create a new manager to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatalf("%+v", err)
	}

	log.Printf("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatalf("%+v", err)
	}

	// Setup all Controllers
	if err := webhook.AddToManager(mgr); err != nil {
		log.Fatalf("%+v", err)
	}

	log.Printf("Starting the webhook...")

	// Start the operator
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
