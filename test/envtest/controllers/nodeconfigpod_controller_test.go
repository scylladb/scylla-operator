//go:build envtest

package controllers

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigpod"
	"github.com/scylladb/scylla-operator/test/envtest"
	kubeinformers "k8s.io/client-go/informers"
)

const nodeConfigPodControllerResyncPeriod = 12 * time.Hour

// runNodeConfigPodController runs the NodeConfigPod controller against the envtest API server.
func runNodeConfigPodController(ctx context.Context, e *envtest.Environment, options ...nodeconfigpod.ControllerOption) {
	g.GinkgoHelper()

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(e.TypedKubeClient(), nodeConfigPodControllerResyncPeriod)
	scyllaInformerFactory := scyllainformers.NewSharedInformerFactory(e.ScyllaClient(), nodeConfigPodControllerResyncPeriod)

	ncpc, err := nodeconfigpod.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient().ScyllaV1alpha1(),
		kubeInformerFactory.Core().V1().Pods(),
		kubeInformerFactory.Core().V1().ConfigMaps(),
		kubeInformerFactory.Core().V1().Nodes(),
		scyllaInformerFactory.Scylla().V1alpha1().NodeConfigs(),
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
		ncpc.Run(ctx, 1)
	})
}
