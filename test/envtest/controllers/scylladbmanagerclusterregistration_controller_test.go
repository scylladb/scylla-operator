//go:build envtest

package controllers

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbmanagerclusterregistration"
	"github.com/scylladb/scylla-operator/test/envtest"
	kubeinformers "k8s.io/client-go/informers"
)

const scyllaDBManagerClusterRegistrationControllerResyncPeriod = 12 * time.Hour

// runScyllaDBManagerClusterRegistrationController runs the ScyllaDBManagerClusterRegistration controller against the
// envtest API server.
func runScyllaDBManagerClusterRegistrationController(ctx context.Context, e *envtest.Environment, options ...scylladbmanagerclusterregistration.ControllerOption) {
	g.GinkgoHelper()

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(e.TypedKubeClient(), scyllaDBManagerClusterRegistrationControllerResyncPeriod)
	scyllaInformerFactory := scyllainformers.NewSharedInformerFactory(e.ScyllaClient(), scyllaDBManagerClusterRegistrationControllerResyncPeriod)

	smcrc, err := scylladbmanagerclusterregistration.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient(),
		scyllaInformerFactory.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		scyllaInformerFactory.Scylla().V1alpha1().ScyllaDBDatacenters(),
		kubeInformerFactory.Core().V1().Secrets(),
		kubeInformerFactory.Core().V1().Namespaces(),
		options...,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	g.DeferCleanup(func() {
		cancel()
		wg.Wait()
		kubeInformerFactory.Shutdown()
		scyllaInformerFactory.Shutdown()
	})

	kubeInformerFactory.Start(ctx.Done())
	scyllaInformerFactory.Start(ctx.Done())
	wg.Go(func() {
		smcrc.Run(ctx, 1)
	})
}
