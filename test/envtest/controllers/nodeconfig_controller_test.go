//go:build envtest

package controllers

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/envtest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
)

const nodeConfigControllerResyncPeriod = 12 * time.Hour

// runNodeConfigController runs the NodeConfig controller against the envtest API server, with its
// ScyllaOperatorConfig informer filtered to the singleton like in the operator binary.
func runNodeConfigController(ctx context.Context, e *envtest.Environment, options ...nodeconfig.ControllerOption) {
	g.GinkgoHelper()

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(e.TypedKubeClient(), nodeConfigControllerResyncPeriod)
	scyllaInformerFactory := scyllainformers.NewSharedInformerFactory(e.ScyllaClient(), nodeConfigControllerResyncPeriod)
	scyllaOperatorConfigInformerFactory := scyllainformers.NewSharedInformerFactoryWithOptions(
		e.ScyllaClient(),
		nodeConfigControllerResyncPeriod,
		scyllainformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", naming.SingletonName).String()
		}),
	)

	ncc, err := nodeconfig.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient().ScyllaV1alpha1(),
		scyllaInformerFactory.Scylla().V1alpha1().NodeConfigs(),
		scyllaOperatorConfigInformerFactory.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		kubeInformerFactory.Rbac().V1().ClusterRoles(),
		kubeInformerFactory.Rbac().V1().ClusterRoleBindings(),
		kubeInformerFactory.Rbac().V1().Roles(),
		kubeInformerFactory.Rbac().V1().RoleBindings(),
		kubeInformerFactory.Apps().V1().DaemonSets(),
		kubeInformerFactory.Core().V1().Namespaces(),
		kubeInformerFactory.Core().V1().Nodes(),
		kubeInformerFactory.Core().V1().ServiceAccounts(),
		kubeInformerFactory.Core().V1().ConfigMaps(),
		envtestOperatorImage,
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
		scyllaOperatorConfigInformerFactory.Shutdown()
	})

	kubeInformerFactory.Start(ctx.Done())
	scyllaInformerFactory.Start(ctx.Done())
	scyllaOperatorConfigInformerFactory.Start(ctx.Done())
	wg.Go(func() {
		ncc.Run(ctx, 1)
	})
}
