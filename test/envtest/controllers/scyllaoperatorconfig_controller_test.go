//go:build envtest

package controllers

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

const scyllaOperatorConfigControllerResyncPeriod = 12 * time.Hour

// runScyllaOperatorConfigController runs the ScyllaOperatorConfig controller against the envtest API server, with
// its informer filtered to the singleton like in the operator binary.
func runScyllaOperatorConfigController(ctx context.Context, e *envtest.Environment, getClusterDomain scyllaoperatorconfig.GetClusterDomainFunc, options ...scyllaoperatorconfig.ControllerOption) {
	g.GinkgoHelper()

	informerFactory := scyllainformers.NewSharedInformerFactoryWithOptions(
		e.ScyllaClient(),
		scyllaOperatorConfigControllerResyncPeriod,
		scyllainformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", naming.SingletonName).String()
		}),
	)

	socc, err := scyllaoperatorconfig.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient().ScyllaV1alpha1(),
		informerFactory.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		getClusterDomain,
		options...,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	g.DeferCleanup(func() {
		cancel()
		wg.Wait()
		informerFactory.Shutdown()
	})

	informerFactory.Start(ctx.Done())
	wg.Go(func() {
		socc.Run(ctx, 1)
	})
}
