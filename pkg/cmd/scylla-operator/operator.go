package main

import (
	"context"
	"fmt"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
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
			v := version.Get()
			logger.Info(ctx, "Operator started", "version", v.GitVersion, "build_date", v.BuildDate,
				"commit", v.GitCommit, "go_version", v.GoVersion, "options", opts)

			// Set log level
			if err := level.UnmarshalText([]byte(opts.LogLevel)); err != nil {
				logger.Error(ctx, "unable to change log level",
					"level", opts.LogLevel, "error", err)
			}

			cfg := ctrl.GetConfigOrDie()
			// Create a new Cmd to provide shared dependencies and start components
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:                 scheme,
				HealthProbeBindAddress: fmt.Sprintf(":%d", naming.ProbePort),
				MetricsBindAddress:     fmt.Sprintf(":%d", naming.MetricsPort),
			})
			if err != nil {
				logger.Fatal(ctx, "unable to create manager", "error", err)
			}

			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				logger.Fatal(ctx, "unable to set up liveness probe", "error", err)
			}

			if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
				logger.Fatal(ctx, "unable to set up readiness probe", "error", err)
			}

			logger.Info(ctx, "Registering Components.")

			ctx := log.WithNewTraceID(context.Background())
			cc, err := cluster.New(ctx, mgr, logger.Named("cluster-controller"))
			if err != nil {
				logger.Fatal(ctx, "unable to create cluster controller", "error", err)
			}
			if err := cc.SetupWithManager(mgr); err != nil {
				logger.Fatal(ctx, "unable to setup cluster manager", "error", err)
			}

			// Enable webhook if requested
			if opts.EnableAdmissionWebhook {
				if err = (&v1.ScyllaCluster{}).SetupWebhookWithManager(mgr); err != nil {
					logger.Fatal(ctx, "unable to add web hook to manager", "error", err)
				}
			}

			logger.Info(ctx, "Starting the operator...")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logger.Fatal(ctx, "error launching manager", "mode", "operator", "error", err)
			}
		},
	}
	options.GetOperatorOptions().AddFlags(operatorCmd)
	return operatorCmd
}
