//go:build envtest

package controllers

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagertask"
	"github.com/scylladb/scylla-operator/test/envtest"
)

const scyllaDBManagerTaskControllerResyncPeriod = 12 * time.Hour

// runScyllaDBManagerTaskController runs the ScyllaDBManagerTask controller against the envtest API server.
func runScyllaDBManagerTaskController(ctx context.Context, e *envtest.Environment, options ...scylladbmanagertask.ControllerOption) {
	g.GinkgoHelper()

	scyllaInformerFactory := scyllainformers.NewSharedInformerFactory(e.ScyllaClient(), scyllaDBManagerTaskControllerResyncPeriod)

	smtc, err := scylladbmanagertask.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient().ScyllaV1alpha1(),
		scyllaInformerFactory.Scylla().V1alpha1().ScyllaDBManagerTasks(),
		scyllaInformerFactory.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		options...,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	g.DeferCleanup(func() {
		cancel()
		wg.Wait()
		scyllaInformerFactory.Shutdown()
	})

	scyllaInformerFactory.Start(ctx.Done())
	wg.Go(func() {
		smtc.Run(ctx, 1)
	})
}
