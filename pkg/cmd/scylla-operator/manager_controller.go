package main

import (
	"context"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/manager"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func newManagerControllerCmd(ctx context.Context, logger log.Logger, level zap.AtomicLevel) *cobra.Command {
	var managerControllerCmd = &cobra.Command{
		Use:   "manager-controller",
		Short: "Start the scylla manager controller.",
		Long:  `manager-controller is for starting the scylla manager controller.`,
		Run: func(cmd *cobra.Command, args []string) {
			opts := options.GetManagerOptions()
			if err := opts.Validate(); err != nil {
				logger.Fatal(ctx, "invalid options", "error", err)
			}
			v := version.Get()
			logger.Info(ctx, "Scylla Manager Controller started", "version", v.GitVersion, "build_date", v.BuildDate,
				"commit", v.GitCommit, "go_version", v.GoVersion, "options", opts)

			// Set log level
			if err := level.UnmarshalText([]byte(opts.LogLevel)); err != nil {
				logger.Error(ctx, "unable to change log level",
					"level", opts.LogLevel, "error", err)
			}

			cfg := ctrl.GetConfigOrDie()
			// Create a new Cmd to provide shared dependencies and start components
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: scheme,
			})
			if err != nil {
				logger.Fatal(ctx, "unable to create manager", "error", err)
			}

			logger.Info(ctx, "Registering Components.")

			ctx := log.WithNewTraceID(context.Background())
			mc, err := manager.New(ctx, mgr, opts.Namespace, logger.Named("scylla-manager-controller"))
			if err != nil {
				logger.Fatal(ctx, "unable to create manager controller", "error", err)
			}
			if err := mc.SetupWithManager(mgr); err != nil {
				logger.Fatal(ctx, "unable to setup manager controller", "error", err)
			}

			logger.Info(ctx, "Starting the controller...")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Fatal(ctx, "error launching manager", "mode", "operator", "error", err)
			}
		},
	}
	options.GetManagerOptions().AddFlags(managerControllerCmd)
	return managerControllerCmd
}
