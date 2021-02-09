// Copyright (C) 2017 ScyllaDB

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster"
	"github.com/scylladb/scylla-operator/pkg/test/integration"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

var (
	testEnv *integration.TestEnvironment
	ctx     = context.Background()
)

const (
	retryInterval = 200 * time.Millisecond
	timeout       = 60 * time.Second
	shortWait     = 30 * time.Second
)

func TestMain(m *testing.M) {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})
	zlogger, _ := zap.NewDevelopment()
	ctrl.SetLogger(zapr.NewLogger(zlogger))
	klog.InitFlags(nil)
	klog.SetOutput(os.Stdout)

	logger.Info(ctx, "Creating test environment")
	var err error
	testEnv, err = integration.NewTestEnvironment(logger.Named("env"),
		integration.WithPollRetryInterval(retryInterval),
		integration.WithPollTimeout(timeout),
	)
	if err != nil {
		panic(err)
	}

	logger.Info(ctx, "Starting test manager")
	go func() {
		if err := testEnv.StartManager(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the envtest manager: %v", err))
		}
	}()

	options.GetOperatorOptions().Image = "scylladb/scylla-operator"
	defer func() {
		options.GetOperatorOptions().Image = ""
	}()

	reconciler, err := cluster.New(ctx, testEnv.Manager, logger)
	if err != nil {
		panic(errors.Wrap(err, "create cluster reconciler"))
	}
	logger.Info(ctx, "Reconciler setup")
	if err := reconciler.SetupWithManager(testEnv.Manager); err != nil {
		panic(errors.Wrap(err, "setup cluster reconciler"))
	}

	logger.Info(ctx, "Starting tests")
	// Run tests
	code := m.Run()
	logger.Info(ctx, "Tests done")
	// Tearing down the test environment
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the envtest: %v", err))
	}

	os.Exit(code)
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}
