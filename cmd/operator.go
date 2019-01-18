package main

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/scylladb/scylla-operator/cmd/options"
	"github.com/scylladb/scylla-operator/pkg/apis"
	"github.com/scylladb/scylla-operator/pkg/controller"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func newOperatorCmd() *cobra.Command {
	var operatorCmd = &cobra.Command{
		Use:   "operator",
		Short: "Start the scylla operator.",
		Long:  `operator is for starting the scylla operator. It is meant to run as a standalone binary.`,
		Run:   startOperator,
	}
	options.
		GetControllerOptions().
		AddFlags(operatorCmd)

	return operatorCmd
}

func startOperator(cmd *cobra.Command, args []string) {

	// Validate the cmd flags
	opts := options.GetControllerOptions()
	if err := opts.Validate(); err != nil {
		log.Fatalf("%+v", err)
	}
	log.Infof("Operator started with options: %+v", spew.Sdump(opts))

	// Set log format
	lvl, _ := log.ParseLevel(opts.LogLevel)
	log.SetLevel(lvl)

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("%+v", err)
	}

	// Create a new Cmd to provide shared dependencies and start components
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
	if err := controller.UseOperatorControllers(mgr); err != nil {
		log.Fatalf("%+v", err)
	}

	log.Printf("Starting the operator...")

	// Start the operator
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
