//go:build envtest

// Copyright (c) 2026 ScyllaDB.

package controllers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

const (
	// statefulSetControllerProgressingConditionType mirrors the unexported condition type of the ScyllaDBDatacenter
	// controller's StatefulSet sync.
	statefulSetControllerProgressingConditionType = "StatefulSetControllerProgressing"

	envtestScyllaDBBaseImage         = unit.ScyllaDBImageRepository + ":2025.1.0"
	envtestScyllaDBPatchImage        = unit.ScyllaDBImageRepository + ":2025.1.1"
	envtestScyllaDBNextMinorImage    = unit.ScyllaDBImageRepository + ":2025.2.0"
	envtestScyllaDBUpgradeEventually = 30 * time.Second
)

var _ = g.Describe("ScyllaDBDatacenter controller StatefulSet sync", func() {
	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	// bringUpRolledOutRacks runs the controller and brings up the racks one by one, marking every StatefulSet as
	// rolled out, and waits for the StatefulSet sync to settle. It returns the ScyllaDBDatacenter.
	bringUpRolledOutRacks := func(ctx g.SpecContext, racks []string, mutators ...func(*scyllav1alpha1.ScyllaDBDatacenter)) *scyllav1alpha1.ScyllaDBDatacenter {
		g.GinkgoHelper()

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), racks, append([]func(*scyllav1alpha1.ScyllaDBDatacenter){withScyllaDBImage(envtestScyllaDBBaseImage)}, mutators...)...)
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, rack := range sdc.Spec.Racks {
			g.By(fmt.Sprintf("Waiting for rack %q StatefulSet to be created and marking it as rolled out", rack.Name))
			stsName := naming.StatefulSetNameForRack(rack, sdc)
			waitForStatefulSet(ctx, env, stsName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), stsName)
		}

		g.By("Waiting for the StatefulSet sync to settle")
		waitForStatefulSetSyncToSettle(ctx, env, sdc.Name)

		return sdc
	}

	g.It("should wait for the ScyllaDB node exporter image before creating any StatefulSet", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton without the node exporter image in its status")
		soc := &scyllav1alpha1.ScyllaOperatorConfig{
			ObjectMeta: metav1.ObjectMeta{Name: naming.SingletonName},
			Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
				ScyllaUtilsImage: "docker.io/scylladb/scylla:6.2.0",
			},
		}
		_, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Create(ctx, soc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBDatacenter")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{"rack-a"})
		sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the StatefulSet sync to report it is waiting for the image")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			cond := apimeta.FindStatusCondition(sdc.Status.Conditions, statefulSetControllerProgressingConditionType)
			eo.Expect(cond).NotTo(o.BeNil())
			eo.Expect(cond.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(cond.Reason).To(o.Equal("WaitingForScyllaDBNodeExporterImage"))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying no StatefulSet is created")
		stsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, stsName, metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should scale a rack StatefulSet up when the node count is raised", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		sdc := bringUpRolledOutRacks(ctx, []string{"rack-a"})
		stsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

		g.By("Raising the node count to two")
		scaleRackTemplate(ctx, env, sdc.Name, 2)

		g.By("Waiting for the rack StatefulSet to be scaled up")
		waitForStatefulSetReplicas(ctx, env, stsName, 2)
	})

	g.It("should prune the StatefulSet of a removed rack and drop its status", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		// A rack can only be removed once it has no members, so the rack to be removed is brought up empty.
		sdc := bringUpRolledOutRacks(ctx, []string{"rack-a", "rack-b"}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.Racks[1].Nodes = new(int32(0))
		})
		removedStsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)

		g.By("Removing the second rack")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.Racks = makeRackSpecs("rack-a")
		})

		g.By("Waiting for the second rack StatefulSet to be deleted and its status dropped")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, removedStsName, metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())

			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			rackNames := make([]string, 0, len(sdc.Status.Racks))
			for _, rackStatus := range sdc.Status.Racks {
				rackNames = append(rackNames, rackStatus.Name)
			}
			eo.Expect(rackNames).To(o.Equal([]string{"rack-a"}))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should roll out a patch version change one rack at a time without running upgrade hooks", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		sdc := bringUpRolledOutRacks(ctx, []string{"rack-a", "rack-b"})
		firstStsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		secondStsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)

		g.By("Bumping the ScyllaDB patch version")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.Image = envtestScyllaDBPatchImage
		})

		g.By("Waiting for the first rack StatefulSet to be updated")
		waitForStatefulSetScyllaDBImage(ctx, env, firstStsName, envtestScyllaDBPatchImage)

		g.By("Verifying the second rack StatefulSet is not updated before the first one rolls out and no upgrade context is created")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			co.Expect(getStatefulSetScyllaDBImage(ctx, env, secondStsName)).To(o.Equal(envtestScyllaDBBaseImage))

			_, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, naming.UpgradeContextConfigMapName(sdc), metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Marking the first rack StatefulSet as rolled out")
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), firstStsName)

		g.By("Waiting for the second rack StatefulSet to be updated")
		waitForStatefulSetScyllaDBImage(ctx, env, secondStsName, envtestScyllaDBPatchImage)
	})

	g.It("should run the upgrade hooks and roll the nodes one by one on a minor version change", func(ctx g.SpecContext) {
		const (
			rackName = "rack-a"
			nodes    = int32(2)
		)

		fakeAPI := newFakeScyllaDBUpgradeAPI([]string{"system", "system_schema", "data_keyspace"})

		g.By("Running ScyllaDBDatacenter controller against a fake ScyllaDB API")
		runScyllaDBDatacenterController(ctx, env, scylladbdatacenter.WithScyllaDBClientFactory(newFakeScyllaDBUpgradeClientFactory(fakeAPI)))

		sdc := bringUpRolledOutRacks(ctx, []string{rackName}, withRackTemplateNodes(nodes))
		stsName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		sts := waitForStatefulSet(ctx, env, stsName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

		g.By("Creating the member Pods in place of the StatefulSet controller")
		hosts := make([]string, nodes)
		for ordinal := range nodes {
			svc := waitForService(ctx, env, fmt.Sprintf("%s-%d", stsName, ordinal), scyllaDBDatacenterControllerDefaultEventuallyTimeout)
			o.Expect(svc.Spec.ClusterIP).NotTo(o.BeEmpty())
			hosts[ordinal] = svc.Spec.ClusterIP
			createMemberPod(ctx, env, sts, ordinal, envtestScyllaDBBaseImage)
		}

		g.By("Bumping the ScyllaDB minor version")
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.Image = envtestScyllaDBNextMinorImage
		})

		g.By("Waiting for the upgrade to start and block on schema agreement in the pre-hooks phase")
		var upgradeContext *internalapi.DatacenterUpgradeContext
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			upgradeContext = getUpgradeContext(ctx, eo, env, sdc)
			eo.Expect(upgradeContext.State).To(o.Equal(internalapi.PreHooksUpgradePhase))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
		o.Expect(upgradeContext.FromVersion).To(o.Equal("2025.1.0"))
		o.Expect(upgradeContext.ToVersion).To(o.Equal("2025.2.0"))
		o.Expect(upgradeContext.SystemSnapshotTag).To(o.HavePrefix("so_system_"))
		o.Expect(upgradeContext.DataSnapshotTag).To(o.HavePrefix("so_data_"))

		o.Consistently(func(co o.Gomega, ctx context.Context) {
			co.Expect(getUpgradeContext(ctx, co, env, sdc).State).To(o.Equal(internalapi.PreHooksUpgradePhase))
			co.Expect(getStatefulSetScyllaDBImage(ctx, env, stsName)).To(o.Equal(envtestScyllaDBBaseImage))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Agreeing on the schema")
		fakeAPI.SetSchemaAgreed(true)

		g.By("Waiting for the rollout to be initialized with a full partition and the updated template")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, stsName, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sts.Spec.UpdateStrategy.RollingUpdate).NotTo(o.BeNil())
			eo.Expect(*sts.Spec.UpdateStrategy.RollingUpdate.Partition).To(o.Equal(nodes))
			eo.Expect(getStatefulSetScyllaDBImage(ctx, env, stsName)).To(o.Equal(envtestScyllaDBNextMinorImage))
			eo.Expect(getUpgradeContext(ctx, eo, env, sdc).State).To(o.Equal(internalapi.RolloutRunUpgradePhase))
		}).WithContext(ctx).WithTimeout(envtestScyllaDBUpgradeEventually).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the system keyspaces were snapshotted on every node")
		for _, host := range hosts {
			o.Expect(fakeAPI.Snapshots(host)).To(o.HaveKeyWithValue(upgradeContext.SystemSnapshotTag, apimachineryutilsets.New("system", "system_schema")))
		}

		g.By("Marking the StatefulSet as rolled out after the rollout init")
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), stsName)

		for ordinal := nodes - 1; ordinal >= 0; ordinal-- {
			podName := fmt.Sprintf("%s-%d", stsName, ordinal)
			host := hosts[ordinal]

			g.By(fmt.Sprintf("Waiting for node %d to be drained, snapshotted, deleted and its partition moved", ordinal))
			o.Eventually(func(eo o.Gomega, ctx context.Context) {
				sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, stsName, metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(*sts.Spec.UpdateStrategy.RollingUpdate.Partition).To(o.Equal(ordinal))
			}).WithContext(ctx).WithTimeout(envtestScyllaDBUpgradeEventually).WithPolling(100 * time.Millisecond).Should(o.Succeed())

			o.Expect(fakeAPI.IsDrained(host)).To(o.BeTrue())
			o.Expect(fakeAPI.Snapshots(host)).To(o.HaveKeyWithValue(upgradeContext.DataSnapshotTag, apimachineryutilsets.New("data_keyspace")))
			_, err := env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Get(ctx, podName, metav1.GetOptions{})
			o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "pod %q should have been deleted", podName)

			if ordinal < nodes-1 {
				g.By(fmt.Sprintf("Verifying the data snapshot of the previously upgraded node %d was removed", ordinal+1))
				o.Expect(fakeAPI.Snapshots(hosts[ordinal+1])).NotTo(o.HaveKey(upgradeContext.DataSnapshotTag))
			}

			g.By(fmt.Sprintf("Recreating the Pod of node %d with the new image in place of the StatefulSet controller", ordinal))
			fakeAPI.SetDrained(host, false)
			createMemberPod(ctx, env, sts, ordinal, envtestScyllaDBNextMinorImage)
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), stsName)
		}

		g.By("Waiting for the post-hooks to finish and the upgrade context to be removed")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, naming.UpgradeContextConfigMapName(sdc), metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(envtestScyllaDBUpgradeEventually).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying all snapshots were removed and no node is left in maintenance mode")
		for ordinal, host := range hosts {
			o.Expect(fakeAPI.Snapshots(host)).To(o.BeEmpty(), "host %q", host)

			svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, fmt.Sprintf("%s-%d", stsName, ordinal), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(svc.Labels).NotTo(o.HaveKey(naming.NodeMaintenanceLabel))
		}

		g.By("Waiting for the StatefulSet sync to settle")
		waitForStatefulSetSyncToSettle(ctx, env, sdc.Name)
	})
})

func withScyllaDBImage(image string) func(*scyllav1alpha1.ScyllaDBDatacenter) {
	return func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
		sdc.Spec.ScyllaDB.Image = image
	}
}

func updateScyllaDBDatacenter(ctx context.Context, e *envtest.Environment, name string, mutate func(*scyllav1alpha1.ScyllaDBDatacenter)) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(e.Namespace(), name), err)
		}

		mutate(sdc)
		_, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Update(ctx, sdc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// waitForStatefulSetSyncToSettle waits for the StatefulSet sync of the ScyllaDBDatacenter to stop progressing.
func waitForStatefulSetSyncToSettle(ctx context.Context, e *envtest.Environment, name string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		sdc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(sdc.Status.ObservedGeneration).NotTo(o.BeNil())
		eo.Expect(*sdc.Status.ObservedGeneration).To(o.Equal(sdc.Generation))
		eo.Expect(apimeta.IsStatusConditionFalse(sdc.Status.Conditions, statefulSetControllerProgressingConditionType)).To(o.BeTrue())
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

func getStatefulSetScyllaDBImage(ctx context.Context, e *envtest.Environment, name string) string {
	g.GinkgoHelper()

	sts, err := e.TypedKubeClient().AppsV1().StatefulSets(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	idx, err := naming.FindScyllaContainer(sts.Spec.Template.Spec.Containers)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sts.Spec.Template.Spec.Containers[idx].Image
}

func waitForStatefulSetScyllaDBImage(ctx context.Context, e *envtest.Environment, name, image string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		eo.Expect(getStatefulSetScyllaDBImage(ctx, e, name)).To(o.Equal(image))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

// getUpgradeContext reads the upgrade context recorded for the ScyllaDBDatacenter, failing the given assertion scope
// when it's not there.
func getUpgradeContext(ctx context.Context, ao o.Gomega, e *envtest.Environment, sdc *scyllav1alpha1.ScyllaDBDatacenter) *internalapi.DatacenterUpgradeContext {
	g.GinkgoHelper()

	cm, err := e.TypedKubeClient().CoreV1().ConfigMaps(e.Namespace()).Get(ctx, naming.UpgradeContextConfigMapName(sdc), metav1.GetOptions{})
	ao.Expect(err).NotTo(o.HaveOccurred())

	uc := &internalapi.DatacenterUpgradeContext{}
	ao.Expect(uc.Decode(strings.NewReader(cm.Data[naming.UpgradeContextConfigMapKey]))).To(o.Succeed())

	return uc
}

// createMemberPod creates the Pod of the given ordinal in place of the StatefulSet controller, owned by the
// StatefulSet and running the given ScyllaDB image.
func createMemberPod(ctx context.Context, e *envtest.Environment, sts *appsv1.StatefulSet, ordinal int32, image string) {
	g.GinkgoHelper()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", sts.Name, ordinal),
			Namespace: sts.Namespace,
			Labels:    sts.Spec.Template.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sts, appsv1.SchemeGroupVersion.WithKind("StatefulSet")),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  naming.ScyllaContainerName,
					Image: image,
				},
			},
		},
	}

	_, err := e.TypedKubeClient().CoreV1().Pods(e.Namespace()).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// fakeScyllaDBUpgradeAPI is a fake of the subset of the ScyllaDB API used by the upgrade hooks. It keeps per-node
// state keyed by the host the request was sent to.
type fakeScyllaDBUpgradeAPI struct {
	mu           sync.Mutex
	schemaAgreed bool
	keyspaces    []string
	nodes        map[string]*fakeScyllaDBUpgradeNode
}

type fakeScyllaDBUpgradeNode struct {
	drained   bool
	snapshots map[string]apimachineryutilsets.Set[string]
}

func newFakeScyllaDBUpgradeAPI(keyspaces []string) *fakeScyllaDBUpgradeAPI {
	return &fakeScyllaDBUpgradeAPI{
		keyspaces: keyspaces,
		nodes:     map[string]*fakeScyllaDBUpgradeNode{},
	}
}

func (f *fakeScyllaDBUpgradeAPI) node(host string) *fakeScyllaDBUpgradeNode {
	n, ok := f.nodes[host]
	if !ok {
		n = &fakeScyllaDBUpgradeNode{snapshots: map[string]apimachineryutilsets.Set[string]{}}
		f.nodes[host] = n
	}
	return n
}

func (f *fakeScyllaDBUpgradeAPI) SetSchemaAgreed(agreed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.schemaAgreed = agreed
}

func (f *fakeScyllaDBUpgradeAPI) SetDrained(host string, drained bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.node(host).drained = drained
}

func (f *fakeScyllaDBUpgradeAPI) IsDrained(host string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.node(host).drained
}

// Snapshots returns a copy of the snapshots taken on the host, keyed by tag.
func (f *fakeScyllaDBUpgradeAPI) Snapshots(host string) map[string]apimachineryutilsets.Set[string] {
	f.mu.Lock()
	defer f.mu.Unlock()

	res := map[string]apimachineryutilsets.Set[string]{}
	for tag, keyspaces := range f.node(host).snapshots {
		res[tag] = keyspaces.Clone()
	}
	return res
}

func (f *fakeScyllaDBUpgradeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	n := f.node(host)

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/storage_proxy/schema_versions":
		if f.schemaAgreed {
			encodeJSON(w, r, []map[string]any{{"key": "schema-v1", "value": []string{host}}})
		} else {
			encodeJSON(w, r, []map[string]any{
				{"key": "schema-v1", "value": []string{host}},
				{"key": "schema-v2", "value": []string{"other"}},
			})
		}

	case r.Method == http.MethodGet && r.URL.Path == "/storage_service/keyspaces":
		encodeJSON(w, r, f.keyspaces)

	case r.Method == http.MethodGet && r.URL.Path == "/storage_service/snapshots":
		snapshots := make([]map[string]any, 0, len(n.snapshots))
		for tag := range n.snapshots {
			snapshots = append(snapshots, map[string]any{"key": tag, "value": []any{}})
		}
		encodeJSON(w, r, snapshots)

	case r.Method == http.MethodPost && r.URL.Path == "/storage_service/snapshots":
		tag := r.URL.Query().Get("tag")
		keyspace := r.URL.Query().Get("kn")
		if _, ok := n.snapshots[tag]; !ok {
			n.snapshots[tag] = apimachineryutilsets.New[string]()
		}
		n.snapshots[tag].Insert(keyspace)

	case r.Method == http.MethodDelete && r.URL.Path == "/storage_service/snapshots":
		delete(n.snapshots, r.URL.Query().Get("tag"))

	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/storage_service/keyspace_flush/"):
		// Flushing has no observable effect in the fake.

	case r.Method == http.MethodGet && r.URL.Path == "/storage_service/operation_mode":
		if n.drained {
			encodeJSON(w, r, "DRAINED")
		} else {
			encodeJSON(w, r, "NORMAL")
		}

	case r.Method == http.MethodPost && r.URL.Path == "/storage_service/drain":
		n.drained = true

	default:
		w.WriteHeader(http.StatusNotImplemented)
		encodeJSON(w, r, map[string]any{"message": fmt.Sprintf("unexpected request %s %s", r.Method, r.URL.Path), "code": http.StatusNotImplemented})
	}
}

// newFakeScyllaDBUpgradeClientFactory returns a ScyllaDB client factory whose clients send every request, regardless
// of the ScyllaDB host it's addressed to, to a single httptest.Server serving the fake. The addressed host is preserved
// in the request so that the fake can keep per-node state. The server is torn down on spec cleanup.
func newFakeScyllaDBUpgradeClientFactory(fake *fakeScyllaDBUpgradeAPI) scylladbdatacenter.ScyllaDBClientFactory {
	g.GinkgoHelper()

	server := httptest.NewServer(fake)
	g.DeferCleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	o.Expect(err).NotTo(o.HaveOccurred())

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", serverURL.Host)
		},
	}

	return func(hosts []string, authToken string) (*scyllaclient.Client, error) {
		cfg := scyllaclient.DefaultConfig(authToken, hosts...)
		cfg.Scheme = "http"
		cfg.Transport = transport
		return scyllaclient.NewClient(cfg)
	}
}
