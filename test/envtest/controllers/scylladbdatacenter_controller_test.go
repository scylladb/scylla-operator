//go:build envtest

package controllers

import (
	"context"
	stdcrypto "crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
)

const scyllaDBDatacenterControllerResyncPeriod = 12 * time.Hour

// scyllaDBDatacenterControllerEventuallyTimeout has to account for a hardcoded 10s sleep in StatefulSet reconciliation loop.
const scyllaDBDatacenterControllerEventuallyTimeout = 30 * time.Second

// scyllaDBDatacenterControllerConsistentlyTimeout has to cover at least one hardcoded 10s sleep in StatefulSet reconciliation loop.
const scyllaDBDatacenterControllerConsistentlyTimeout = 15 * time.Second

var _ = g.Describe("ScyllaDBDatacenter controller", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should create rack StatefulSets sequentially", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with two racks")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a", "rack-b"})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the first rack StatefulSet to be created")
		firstRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		waitForStatefulSet(ctx, env, firstRackStatefulSetName)

		g.By("Verifying the second rack StatefulSet is not created before the first rack rolls out")
		secondRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, secondRackStatefulSetName, metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Marking the first rack StatefulSet as rolled out")
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), firstRackStatefulSetName)

		g.By("Waiting for the second rack StatefulSet to be created")
		waitForStatefulSet(ctx, env, secondRackStatefulSetName)
	})

	g.DescribeTable("should not create a StatefulSet for a new rack while an existing rack is not rolled out",
		func(ctx g.SpecContext, initialRacks, updatedRacks []string, existingRack, newRack string) {
			g.By("Running ScyllaDBDatacenter controller")
			runScyllaDBDatacenterController(ctx, env)

			g.By("Creating ScyllaOperatorConfig singleton")
			createScyllaOperatorConfig(ctx, env)

			g.By("Creating a ScyllaDBDatacenter")
			sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), initialRacks)
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for the first StatefulSet to be created")
			existingRackStatefulSetName := naming.StatefulSetNameForRack(makeRackSpec(existingRack), sdc)
			waitForStatefulSet(ctx, env, existingRackStatefulSetName)

			g.By("Marking the first rack StatefulSet as not rolled out")
			markStatefulSetAsNotRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), existingRackStatefulSetName)

			g.By("Adding a new rack")
			sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			sdc.Spec.Racks = makeRackSpecs(updatedRacks...)
			_, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the new rack StatefulSet is not created")
			newRackStatefulSetName := naming.StatefulSetNameForRack(makeRackSpec(newRack), sdc)
			o.Consistently(func(co o.Gomega, ctx context.Context) {
				_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, newRackStatefulSetName, metav1.GetOptions{})
				co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
			}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
		},
		g.Entry("when new rack is prepended", []string{"rack-b"}, []string{"rack-a", "rack-b"}, "rack-b", "rack-a"),
		g.Entry("when new rack is appended", []string{"rack-a"}, []string{"rack-a", "rack-b"}, "rack-a", "rack-b"),
	)
})

func waitForStatefulSet(ctx context.Context, e *envtest.Environment, name string) *appsv1.StatefulSet {
	g.GinkgoHelper()

	var statefulSet *appsv1.StatefulSet
	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		var err error
		statefulSet, err = e.TypedKubeClient().AppsV1().StatefulSets(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerEventuallyTimeout).Should(o.Succeed())

	return statefulSet
}

func makeEnvtestScyllaDBDatacenter(namespace string, racks []string) *scyllav1alpha1.ScyllaDBDatacenter {
	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envtest-sdc",
			Namespace: namespace,
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ClusterName: "envtest-cluster",
			DNSDomains:  []string{"envtest.local"},
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image:               "scylladb/scylla:envtest",
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

	return sdc
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
