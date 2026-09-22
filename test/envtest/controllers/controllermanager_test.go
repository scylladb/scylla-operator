//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllermanager"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	"github.com/scylladb/scylla-operator/test/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Controller manager", func() {
	g.It("runs every controller of the operator binary on the shared controller-runtime cache", func(ctx g.SpecContext) {
		env := envtest.Setup(ctx)

		g.By("Starting the controller manager with the same wiring as the operator binary")
		runControllerManager(ctx, env)

		g.By("Expecting the ScyllaOperatorConfig controller to create the singleton")
		waitForScyllaOperatorConfigSingleton(ctx, env)

		g.By("Expecting the ScyllaCluster controller to create the ScyllaDBDatacenter")
		sc := newBasicScyllaCluster("shared-cache", env.Namespace())
		_, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)

		g.By("Expecting the ScyllaDBDatacenter controller to reconcile it")
		o.Eventually(func(eo o.Gomega) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(sdc.Namespace).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.Conditions).NotTo(o.BeEmpty())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Expecting the ScyllaDBMonitoring controller to run, as the Prometheus Operator CRDs are installed")
		sm := createScyllaDBMonitoring(ctx, env, newBasicScyllaDBMonitoring("shared-cache", env.Namespace()))
		o.Eventually(func(eo o.Gomega) {
			_, err := env.TypedKubeClient().AppsV1().Deployments(env.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana", sm.Name), metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("runs without the ScyllaDBMonitoring controller when the Prometheus Operator CRDs are not installed", func(ctx g.SpecContext) {
		env := envtest.Setup(ctx, envtest.WithoutMonitoringCRDs())

		g.By("Starting the controller manager with the same wiring as the operator binary")
		runControllerManager(ctx, env)

		g.By("Expecting the ScyllaOperatorConfig controller to create the singleton")
		waitForScyllaOperatorConfigSingleton(ctx, env)

		g.By("Expecting a ScyllaDBMonitoring to stay unreconciled")
		sm := createScyllaDBMonitoring(ctx, env, newBasicScyllaDBMonitoring("no-monitoring-crds", env.Namespace()))
		o.Consistently(func(eo o.Gomega) {
			sm, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sm.Namespace).Get(ctx, sm.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sm.Status.Conditions).To(o.BeEmpty())
		}).WithTimeout(5 * time.Second).WithContext(ctx).Should(o.Succeed())
	})
})

func waitForScyllaOperatorConfigSingleton(ctx context.Context, e *envtest.Environment) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega) {
		_, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Get(ctx, naming.SingletonName, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
}

// runControllerManager starts the controller manager the operator binary runs, against the envtest API server,
// with every controller registered. adjust functions edit the options before the manager is built.
func runControllerManager(ctx context.Context, e *envtest.Environment, adjust ...func(*controllermanager.Options)) *controllermanager.Manager {
	g.GinkgoHelper()

	options := controllermanager.Options{
		RestConfig:       e.Config(),
		Logger:           g.GinkgoLogr,
		KubeClient:       e.TypedKubeClient(),
		ScyllaClient:     e.ScyllaClient(),
		MonitoringClient: e.MonitoringClient(),
		ClusterDomainGetter: func(ctx context.Context) (string, error) {
			return "cluster.local", nil
		},
		KeyGenerator:    newStaticKeyGenerator(),
		OperatorImage:   envtestOperatorImage,
		CQLSIngressPort: scylla.DefaultNativeTransportPort,
		ConcurrentSyncs: 1,
		ResyncPeriod:    12 * time.Hour,
	}
	for _, f := range adjust {
		f(&options)
	}

	cm, err := controllermanager.New(options)
	o.Expect(err).NotTo(o.HaveOccurred())

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	g.DeferCleanup(func() {
		cancel()
		wg.Wait()
	})

	wg.Go(func() {
		defer g.GinkgoRecover()
		err := cm.Run(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	return cm
}
