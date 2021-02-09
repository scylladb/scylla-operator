package main

import (
	"context"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	// TODO: What is this package for?
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = scyllav1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// Basic structure for a simple cobra cli application
// For more info on the structure of the code in package main,
// see: https://github.com/spf13/cobra
func main() {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	var rootCmd = &cobra.Command{}
	rootCmd.AddCommand(
		newOperatorCmd(ctx, logger, atom),
		newSidecarCmd(ctx, logger, atom),
		newManagerControllerCmd(ctx, logger, atom),
	)
	if err := rootCmd.Execute(); err != nil {
		logger.Error(context.Background(), "Root command: a fatal error occured", "error", err)
	}
}
