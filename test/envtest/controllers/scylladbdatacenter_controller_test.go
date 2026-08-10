//go:build envtest

package controllers

import (
	"context"
	stdcrypto "crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/util/retry"
)

const (
	// scyllaDBDatacenterControllerDisabledStatefulSetCachePropagationDelay disables the production cache-propagation
	// wait in envtests. Envtest runs the controller and API server in-process, so the default delay only slows tests down.
	// Tests that need to exercise cache lag should override this.
	scyllaDBDatacenterControllerDisabledStatefulSetCachePropagationDelay = 0 * time.Second

	scyllaDBDatacenterControllerResyncPeriod = 12 * time.Hour

	// scyllaDBDatacenterControllerDefaultEventuallyTimeout is the default timeout for async envtest assertions.
	// Pad accordingly when a test uses a non-zero cache-propagation delay, otherwise Eventually may time out before
	// the controller resumes reconciliation.
	scyllaDBDatacenterControllerDefaultEventuallyTimeout = 15 * time.Second

	// scyllaDBDatacenterControllerDefaultConsistentlyTimeout is the default window for stability assertions.
	// Pad accordingly when a test uses a non-zero cache-propagation delay, otherwise Consistently may pass while the
	// controller is delayed instead of observing real steady state.
	scyllaDBDatacenterControllerDefaultConsistentlyTimeout = 5 * time.Second
)

var _ = g.Describe("ScyllaDBDatacenter controller", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should create rack StatefulSets sequentially with Sequential bootstrap policy", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with two racks")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a", "rack-b"}, withBootstrapPolicy(scyllav1alpha1.BootstrapPolicySequential))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the first rack StatefulSet to be created")
		firstRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, firstRackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

		g.By("Verifying the second rack StatefulSet is not created before the first rack rolls out")
		secondRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, secondRackStatefulSetName, metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Marking the first rack StatefulSet as rolled out")
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), firstRackStatefulSetName)

		g.By("Waiting for the second rack StatefulSet to be created")
		waitForStatefulSet(ctx, env, secondRackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
	})

	g.It("should create rack StatefulSets in parallel with Parallel bootstrap policy", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with three racks and the Parallel bootstrap policy")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a", "rack-b", "rack-c"}, withBootstrapPolicy(scyllav1alpha1.BootstrapPolicyParallel))
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for all rack StatefulSets to be created without any of them rolling out")
		for _, rack := range sdc.Spec.Racks {
			statefulSet := waitForStatefulSet(ctx, env, naming.StatefulSetNameForRack(rack, sdc), scyllaDBDatacenterControllerDefaultEventuallyTimeout)
			o.Expect(statefulSet.Spec.PodManagementPolicy).To(o.Equal(appsv1.ParallelPodManagement))
		}
	})

	g.DescribeTableSubtree("with bootstrap policy",
		func(bootstrapPolicy scyllav1alpha1.BootstrapPolicy) {
			g.DescribeTable("should not create a StatefulSet for a new rack while an existing rack is not rolled out",
				func(ctx g.SpecContext, initialRacks, updatedRacks []string, existingRack, newRack string) {
					g.By("Running ScyllaDBDatacenter controller")
					runScyllaDBDatacenterController(ctx, env)

					g.By("Creating ScyllaOperatorConfig singleton")
					createScyllaOperatorConfig(ctx, env)

					g.By("Creating a ScyllaDBDatacenter")
					sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), initialRacks, withBootstrapPolicy(bootstrapPolicy))
					sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Waiting for the first StatefulSet to be created")
					existingRackStatefulSetName := naming.StatefulSetNameForRack(makeRackSpec(existingRack), sdc)
					waitForStatefulSet(ctx, env, existingRackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

					g.By("Marking the first rack StatefulSet as not rolled out")
					markStatefulSetAsNotRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), existingRackStatefulSetName)

					g.By("Adding a new rack")
					err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
						if err != nil {
							return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(env.Namespace(), sdc.Name), err)
						}

						sdc.Spec.Racks = makeRackSpecs(updatedRacks...)
						_, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})
						if err != nil {
							return fmt.Errorf("can't update ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
						}

						return nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Verifying the new rack StatefulSet is not created")
					newRackStatefulSetName := naming.StatefulSetNameForRack(makeRackSpec(newRack), sdc)
					o.Consistently(func(co o.Gomega, ctx context.Context) {
						_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, newRackStatefulSetName, metav1.GetOptions{})
						co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
					}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
				},
				g.Entry("when new rack is prepended", []string{"rack-b"}, []string{"rack-a", "rack-b"}, "rack-b", "rack-a"),
				g.Entry("when new rack is appended", []string{"rack-a"}, []string{"rack-a", "rack-b"}, "rack-a", "rack-b"),
			)
		},
		g.Entry("Sequential", scyllav1alpha1.BootstrapPolicySequential),
		// The Parallel bootstrap policy only relaxes the initial creation of missing StatefulSets. Adding a rack to an
		// existing datacenter still waits for the existing racks to roll out, so that racks aren't added while another one
		// is scaling or updating, regardless of the bootstrap policy.
		g.Entry("Parallel", scyllav1alpha1.BootstrapPolicyParallel),
	)
})

func waitForStatefulSet(ctx context.Context, e *envtest.Environment, name string, timeout time.Duration) *appsv1.StatefulSet {
	g.GinkgoHelper()

	var statefulSet *appsv1.StatefulSet
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		var err error
		statefulSet, err = e.TypedKubeClient().AppsV1().StatefulSets(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(timeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

	return statefulSet
}

func makeEnvtestScyllaDBDatacenter(namespace string, racks []string, mutators ...func(*scyllav1alpha1.ScyllaDBDatacenter)) *scyllav1alpha1.ScyllaDBDatacenter {
	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envtest-sdc",
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ClusterName: "envtest-cluster",
			DNSDomains:  []string{"envtest.local"},
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image:               unit.ScyllaDBImageRepository + ":" + configassets.Project.Operator.ScyllaDBVersion,
				EnableDeveloperMode: new(true),
			},
			RackTemplate: &scyllav1alpha1.RackTemplate{
				Nodes: new(int32(1)),
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
					},
				},
			},
		},
	}

	sdc.Spec.Racks = makeRackSpecs(racks...)

	for _, mutator := range mutators {
		mutator(sdc)
	}

	return sdc
}

func withBootstrapPolicy(bootstrapPolicy scyllav1alpha1.BootstrapPolicy) func(*scyllav1alpha1.ScyllaDBDatacenter) {
	return func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
		sdc.Spec.BootstrapPolicy = new(bootstrapPolicy)
	}
}

func makeRackSpecs(names ...string) []scyllav1alpha1.RackSpec {
	racks := make([]scyllav1alpha1.RackSpec, 0, len(names))
	for _, name := range names {
		racks = append(racks, makeRackSpec(name))
	}
	return racks
}

func makeRackSpec(name string) scyllav1alpha1.RackSpec {
	return scyllav1alpha1.RackSpec{Name: name}
}

func markStatefulSetAsNotRolledOut(ctx context.Context, statefulSets appsv1client.StatefulSetInterface, name string) {
	g.GinkgoHelper()

	statefulSet, err := statefulSets.Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	statefulSet.Status.ObservedGeneration = statefulSet.Generation - 1
	statefulSet.Status.Replicas = 1
	statefulSet.Status.ReadyReplicas = 0
	statefulSet.Status.UpdatedReplicas = 0
	_, err = statefulSets.UpdateStatus(ctx, statefulSet, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func markStatefulSetAsRolledOut(ctx context.Context, statefulSets appsv1client.StatefulSetInterface, name string) {
	g.GinkgoHelper()

	statefulSet, err := statefulSets.Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	replicas := *statefulSet.Spec.Replicas
	statefulSet.Status.ObservedGeneration = statefulSet.Generation
	statefulSet.Status.Replicas = replicas
	statefulSet.Status.ReadyReplicas = replicas
	statefulSet.Status.AvailableReplicas = replicas
	statefulSet.Status.UpdatedReplicas = replicas
	statefulSet.Status.CurrentRevision = "envtest-revision"
	statefulSet.Status.UpdateRevision = statefulSet.Status.CurrentRevision
	_, err = statefulSets.UpdateStatus(ctx, statefulSet, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

type staticKeyGenerator struct {
	key stdcrypto.Signer
}

func newStaticKeyGenerator() *staticKeyGenerator {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	o.Expect(err).NotTo(o.HaveOccurred())

	return &staticKeyGenerator{key: key}
}

func (g *staticKeyGenerator) GetNewKey(ctx context.Context) (stdcrypto.Signer, error) {
	return g.key, nil
}

func (g *staticKeyGenerator) GetKeyType() crypto.KeyType {
	return crypto.ECDSAKeyType
}

func runScyllaDBDatacenterController(ctx context.Context, e *envtest.Environment) {
	g.GinkgoHelper()

	kubeClient := e.TypedKubeClient()
	scyllaClient := e.ScyllaClient()
	kubeInformers := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		scyllaDBDatacenterControllerResyncPeriod,
		informers.WithNamespace(e.Namespace()),
	)
	scyllaInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		scyllaClient,
		scyllaDBDatacenterControllerResyncPeriod,
		scyllainformers.WithNamespace(e.Namespace()),
	)
	scyllaGlobalInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		scyllaClient,
		scyllaDBDatacenterControllerResyncPeriod,
		scyllainformers.WithNamespace(corev1.NamespaceAll),
	)
	keyGenerator := newStaticKeyGenerator()

	options := []scylladbdatacenter.ControllerOption{
		// The default delay only slows tests down; tests that need to exercise cache lag should override this.
		scylladbdatacenter.WithStatefulSetCachePropagationDelay(scyllaDBDatacenterControllerDisabledStatefulSetCachePropagationDelay),
	}

	sdcc, err := scylladbdatacenter.NewController(
		kubeClient,
		scyllaClient.ScyllaV1alpha1(),
		kubeInformers.Core().V1().Pods(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Apps().V1().StatefulSets(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Networking().V1().Ingresses(),
		kubeInformers.Batch().V1().Jobs(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenterNodesStatusReports(),
		scyllaGlobalInformers.Scylla().V1alpha1().ScyllaOperatorConfigs(),
		"scylla/operator:envtest",
		scylla.DefaultNativeTransportPort,
		keyGenerator,
		options...,
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	kubeInformers.Start(ctx.Done())
	scyllaInformers.Start(ctx.Done())
	scyllaGlobalInformers.Start(ctx.Done())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sdcc.Run(ctx, 1)
	}()

	g.DeferCleanup(func() {
		kubeInformers.Shutdown()
		scyllaInformers.Shutdown()
		scyllaGlobalInformers.Shutdown()
		wg.Wait()
	})
}
