package main

import (
	"context"

	"github.com/scylladb/go-log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	// TODO: What is this package for?
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// Basic structure for a simple cobra cli application
// For more info on the structure of the code in package main,
// see: https://github.com/spf13/cobra
func main() {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	var rootCmd = &cobra.Command{}
	rootCmd.AddCommand(newOperatorCmd(ctx, logger, atom), newSidecarCmd(ctx, logger, atom))
	if err := rootCmd.Execute(); err != nil {
		logger.Error(context.Background(), "Root command: a fatal error occured: %+v", err)
	}
}
