//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/scylladb/scylla-operator/test/envtest"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

// The upgrade hooks talk to the ScyllaDB API of every node, so the controller runs against a fake one and the member
// Pods are created in place of the StatefulSet controller, which envtest doesn't run.
var _ = g.Describe("ScyllaDBDatacenter controller upgrade", func() {
	const (
		rackName      = "rack-a"
		otherRackName = "rack-b"
		nodes         = int32(2)
	)

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should upgrade ScyllaDB through the hook phases one rack and one node at a time", func(ctx g.SpecContext) {
		systemKeyspaces := []string{"system", "system_schema"}
		dataKeyspaces := []string{"ks1", "ks2"}

		fromImage := unit.ScyllaDBImageRepository + ":" + configassets.Project.OperatorTests.ScyllaDBVersions.UpgradeFrom
		toImage := unit.ScyllaDBImageRepository + ":" + configassets.Project.Operator.ScyllaDBVersion
		fromVersion, err := naming.ImageToVersion(fromImage)
		o.Expect(err).NotTo(o.HaveOccurred())
		toVersion, err := naming.ImageToVersion(toImage)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Running ScyllaDBDatacenter controller against a fake ScyllaDB API")
		fakeAPI := newFakeScyllaDBUpgradeAPI(slices.Concat(systemKeyspaces, dataKeyspaces))
		newScyllaClient := newFakeScyllaDBClientFactory(fakeAPI)
		runScyllaDBDatacenterControllerWithOptions(ctx, env, scyllaDBDatacenterControllerRunOptions{
			controllerOptions: []scylladbdatacenter.ControllerOption{
				scylladbdatacenter.WithNewScyllaClientFunc(func([]string, string) (*scyllaclient.Client, error) {
					return newScyllaClient()
				}),
			},
		})

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By(fmt.Sprintf("Creating a ScyllaDBDatacenter with two racks of %d nodes running ScyllaDB %s", nodes, fromVersion))
		// Parallel node operations aren't supported by the version the upgrade starts from, so the racks are created
		// one by one, each once the previous one is reported as rolled out.
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName, otherRackName}, withRackTemplateNodes(nodes), withEnableParallelNodeOperations(false), func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.Image = fromImage
		})
		sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the rack StatefulSets and member Services and creating the member Pods in place of the StatefulSet controller")
		rackStatefulSets := map[string]*appsv1.StatefulSet{}
		// nodeHosts maps the member Service names to the hosts the controller reaches their nodes at.
		nodeHosts := map[string]string{}
		for _, rack := range sdc.Spec.Racks {
			rackStatefulSetName := naming.StatefulSetNameForRack(rack, sdc)
			rackStatefulSets[rack.Name] = waitForStatefulSet(ctx, env, rackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)

			for ordinal := range int(nodes) {
				memberServiceName := naming.MemberServiceName(rack, sdc, ordinal)
				svc := waitForService(ctx, env, memberServiceName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
				o.Expect(svc.Spec.ClusterIP).NotTo(o.BeEmpty())
				nodeHosts[memberServiceName] = svc.Spec.ClusterIP

				createMemberPod(ctx, env, rackStatefulSets[rack.Name], ordinal, fromImage)
			}

			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)
		}

		g.By("Waiting for the racks to be reported at the current version")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, rack := range sdc.Spec.Racks {
				rackStatus := getRackStatus(ctx, env, sdc.Name, rack.Name)
				eo.Expect(rackStatus).NotTo(o.BeNil())
				eo.Expect(rackStatus.CurrentVersion).To(o.Equal(fromVersion))
				eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()))
			}
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Upgrading ScyllaDB to %s", toVersion))
		updateScyllaDBDatacenter(ctx, env, sdc.Name, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) {
			sdc.Spec.ScyllaDB.Image = toImage
		})

		g.By("Waiting for the upgrade to start with the pre-upgrade hooks and the upgrade context to be recorded")
		upgradeContextConfigMapName := naming.UpgradeContextConfigMapName(sdc)
		var upgradeContext *internalapi.DatacenterUpgradeContext
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			upgradeContext = getUpgradeContext(ctx, env, upgradeContextConfigMapName)
			eo.Expect(upgradeContext).NotTo(o.BeNil())
			eo.Expect(upgradeContext.FromVersion).To(o.Equal(fromVersion))
			eo.Expect(upgradeContext.ToVersion).To(o.Equal(toVersion))
			eo.Expect(upgradeContext.SystemSnapshotTag).NotTo(o.BeEmpty())
			eo.Expect(upgradeContext.DataSnapshotTag).NotTo(o.BeEmpty())
			eo.Expect(upgradeContext.DataSnapshotTag).NotTo(o.Equal(upgradeContext.SystemSnapshotTag))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Waiting for the rollout to be initialized with every StatefulSet partitioned at its node count and updated to the new version")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			for _, rack := range sdc.Spec.Racks {
				sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, naming.StatefulSetNameForRack(rack, sdc), metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(sts.Spec.Replicas).To(o.HaveValue(o.Equal(nodes)))
				eo.Expect(sts.Spec.UpdateStrategy.RollingUpdate).NotTo(o.BeNil())
				eo.Expect(sts.Spec.UpdateStrategy.RollingUpdate.Partition).To(o.HaveValue(o.Equal(nodes)))
				eo.Expect(getStatefulSetScyllaDBImage(sts)).To(o.Equal(toImage))
			}

			upgradeContext = getUpgradeContext(ctx, env, upgradeContextConfigMapName)
			eo.Expect(upgradeContext).NotTo(o.BeNil())
			eo.Expect(upgradeContext.State).To(o.Equal(internalapi.RolloutRunUpgradePhase))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the pre-upgrade hooks checked the schema agreement and snapshotted the system keyspaces on every node")
		o.Expect(fakeAPI.SchemaAgreementChecks()).To(o.BeNumerically(">=", 1))
		schemaAgreementChecks := fakeAPI.SchemaAgreementChecks()
		for _, host := range nodeHosts {
			o.Expect(fakeAPI.Snapshots(host)).To(o.HaveKeyWithValue(upgradeContext.SystemSnapshotTag, apimachineryutilsets.New(systemKeyspaces...)), "host %q", host)
			o.Expect(fakeAPI.SnapshotsTaken(host, upgradeContext.SystemSnapshotTag)).To(o.Equal(len(systemKeyspaces)), "host %q", host)
		}

		// The hooks are reentrant, so an unknown phase, e.g. one left behind by another version of the controller,
		// is reset to the first one and the phases run again. The rollout can't proceed until the partitioned
		// StatefulSets are reported as rolled out, so the reset is observed before any node is touched.
		g.By("Recording an unknown upgrade phase")
		setUpgradeContextState(ctx, env, upgradeContextConfigMapName, "Unknown")

		g.By("Marking the partitioned StatefulSets as rolled out")
		for _, rack := range sdc.Spec.Racks {
			markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), naming.StatefulSetNameForRack(rack, sdc))
		}

		g.By("Waiting for the phases to be run again from the pre-upgrade hooks without taking the snapshots again")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			eo.Expect(fakeAPI.SchemaAgreementChecks()).To(o.BeNumerically(">", schemaAgreementChecks))

			upgradeContext = getUpgradeContext(ctx, env, upgradeContextConfigMapName)
			eo.Expect(upgradeContext).NotTo(o.BeNil())
			eo.Expect(upgradeContext.State).To(o.Equal(internalapi.RolloutRunUpgradePhase))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
		for _, host := range nodeHosts {
			o.Expect(fakeAPI.SnapshotsTaken(host, upgradeContext.SystemSnapshotTag)).To(o.Equal(len(systemKeyspaces)), "host %q", host)
		}

		// The nodes are upgraded one at a time, from the highest ordinal of the first rack down to the lowest ordinal
		// of the last one. Every node is drained and its data keyspaces snapshotted under maintenance, then its Pod is
		// deleted for the StatefulSet controller to recreate it at the new version, and the partition is moved below
		// it. The snapshot is removed once the new Pod is ready.
		for rackIdx, rack := range sdc.Spec.Racks {
			rackStatefulSetName := naming.StatefulSetNameForRack(rack, sdc)

			for ordinal := int(nodes) - 1; ordinal >= 0; ordinal-- {
				memberServiceName := naming.MemberServiceName(rack, sdc, ordinal)
				host := nodeHosts[memberServiceName]

				g.By(fmt.Sprintf("Waiting for node %q to be drained, snapshotted and deleted, and the partition to move below it", memberServiceName))
				o.Eventually(func(eo o.Gomega, ctx context.Context) {
					_, err := env.TypedKubeClient().CoreV1().Pods(env.Namespace()).Get(ctx, memberServiceName, metav1.GetOptions{})
					eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "Pod %q should be deleted: %v", memberServiceName, err)

					sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
					eo.Expect(err).NotTo(o.HaveOccurred())
					eo.Expect(sts.Spec.UpdateStrategy.RollingUpdate.Partition).To(o.HaveValue(o.BeEquivalentTo(ordinal)))
				}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

				o.Expect(fakeAPI.Drained(host)).To(o.BeTrue())
				o.Expect(fakeAPI.Snapshots(host)).To(o.HaveKeyWithValue(upgradeContext.DataSnapshotTag, apimachineryutilsets.New(dataKeyspaces...)))

				svc, err := env.TypedKubeClient().CoreV1().Services(env.Namespace()).Get(ctx, memberServiceName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(svc.Labels).NotTo(o.HaveKey(naming.NodeMaintenanceLabel), "the maintenance mode should be over once the node is drained and snapshotted")

				if rackIdx == 0 {
					g.By(fmt.Sprintf("Verifying the %q rack is not touched while the %q rack is being upgraded", otherRackName, rackName))
					otherSts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc), metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(otherSts.Spec.UpdateStrategy.RollingUpdate.Partition).To(o.HaveValue(o.Equal(nodes)))
				}

				g.By(fmt.Sprintf("Recreating node %q at the new version in place of the StatefulSet controller and marking the rack as rolled out", memberServiceName))
				createMemberPod(ctx, env, rackStatefulSets[rack.Name], ordinal, toImage)
				markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

				g.By(fmt.Sprintf("Waiting for the data snapshot of node %q to be removed", memberServiceName))
				o.Eventually(func(eo o.Gomega, ctx context.Context) {
					eo.Expect(fakeAPI.Snapshots(host)).NotTo(o.HaveKey(upgradeContext.DataSnapshotTag))
				}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
			}
		}

		g.By("Waiting for the post-upgrade hooks to remove the system snapshots and the upgrade context")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, upgradeContextConfigMapName, metav1.GetOptions{})
			eo.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(), "ConfigMap %q should be deleted: %v", upgradeContextConfigMapName, err)

			for _, host := range nodeHosts {
				eo.Expect(fakeAPI.Snapshots(host)).To(o.BeEmpty(), "host %q", host)
			}
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Waiting for the racks to be reported at the new version with the StatefulSet sync settled")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sdc.Name, metav1.GetOptions{})
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(sdc.Status.ObservedGeneration).To(o.HaveValue(o.Equal(sdc.Generation)))

			for _, rack := range sdc.Spec.Racks {
				rackStatus := getRackStatus(ctx, env, sdc.Name, rack.Name)
				eo.Expect(rackStatus).NotTo(o.BeNil())
				eo.Expect(rackStatus.CurrentVersion).To(o.Equal(toVersion))
				eo.Expect(rackStatus.UpdatedVersion).To(o.Equal(toVersion))
				eo.Expect(rackStatus.Stale).To(o.HaveValue(o.BeFalse()))

				sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, naming.StatefulSetNameForRack(rack, sdc), metav1.GetOptions{})
				eo.Expect(err).NotTo(o.HaveOccurred())
				eo.Expect(sts.Spec.UpdateStrategy.RollingUpdate.Partition).To(o.HaveValue(o.BeEquivalentTo(0)))
			}

			expectConditionStatus(eo, sdc, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.ProgressingCondition), metav1.ConditionFalse)
			expectConditionStatus(eo, sdc, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.AvailableCondition), metav1.ConditionTrue)
			expectConditionStatus(eo, sdc, scyllav1alpha1.DegradedCondition, metav1.ConditionFalse)
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

// getUpgradeContext returns the upgrade context recorded in the named ConfigMap, or nil if there is none.
func getUpgradeContext(ctx context.Context, e *envtest.Environment, name string) *internalapi.DatacenterUpgradeContext {
	g.GinkgoHelper()

	cm, err := e.TypedKubeClient().CoreV1().ConfigMaps(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	upgradeContext := &internalapi.DatacenterUpgradeContext{}
	err = upgradeContext.Decode(strings.NewReader(cm.Data[naming.UpgradeContextConfigMapKey]))
	o.Expect(err).NotTo(o.HaveOccurred())

	return upgradeContext
}

// setUpgradeContextState overwrites the phase recorded in the named upgrade context ConfigMap.
func setUpgradeContextState(ctx context.Context, e *envtest.Environment, name string, state internalapi.UpgradePhase) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm, err := e.TypedKubeClient().CoreV1().ConfigMaps(e.Namespace()).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get ConfigMap %q: %w", naming.ManualRef(e.Namespace(), name), err)
		}

		upgradeContext := &internalapi.DatacenterUpgradeContext{}
		err = upgradeContext.Decode(strings.NewReader(cm.Data[naming.UpgradeContextConfigMapKey]))
		if err != nil {
			return fmt.Errorf("can't decode upgrade context from ConfigMap %q: %w", naming.ObjRef(cm), err)
		}

		upgradeContext.State = state
		data, err := upgradeContext.Encode()
		if err != nil {
			return fmt.Errorf("can't encode upgrade context: %w", err)
		}

		cm.Data[naming.UpgradeContextConfigMapKey] = string(data)
		_, err = e.TypedKubeClient().CoreV1().ConfigMaps(e.Namespace()).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update ConfigMap %q: %w", naming.ObjRef(cm), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// getStatefulSetScyllaDBImage returns the image of the ScyllaDB container of the StatefulSet's Pod template.
func getStatefulSetScyllaDBImage(sts *appsv1.StatefulSet) string {
	g.GinkgoHelper()

	idx, err := naming.FindScyllaContainer(sts.Spec.Template.Spec.Containers)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sts.Spec.Template.Spec.Containers[idx].Image
}

// fakeScyllaDBUpgradeAPI is the surface of the ScyllaDB API the upgrade hooks use, for a cluster whose nodes agree on
// the schema. Its state is kept per node, told apart by the host the request is addressed to. Every node is in the
// normal operation mode until it is drained, and every snapshot is recorded until it is deleted.
type fakeScyllaDBUpgradeAPI struct {
	keyspaces []string

	mu                    sync.Mutex
	schemaAgreementChecks int
	drained               apimachineryutilsets.Set[string]
	// snapshots maps a host to its snapshot tags to the keyspaces snapshotted under the tag.
	snapshots map[string]map[string]apimachineryutilsets.Set[string]
	// snapshotsTaken maps a host to its snapshot tags to how many times a snapshot was taken under the tag.
	snapshotsTaken map[string]map[string]int
}

func newFakeScyllaDBUpgradeAPI(keyspaces []string) *fakeScyllaDBUpgradeAPI {
	return &fakeScyllaDBUpgradeAPI{
		keyspaces:      keyspaces,
		drained:        apimachineryutilsets.New[string](),
		snapshots:      map[string]map[string]apimachineryutilsets.Set[string]{},
		snapshotsTaken: map[string]map[string]int{},
	}
}

func (f *fakeScyllaDBUpgradeAPI) SchemaAgreementChecks() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.schemaAgreementChecks
}

func (f *fakeScyllaDBUpgradeAPI) Drained(host string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.drained.Has(host)
}

// Snapshots returns the snapshots of the host by tag, with the keyspaces snapshotted under each.
func (f *fakeScyllaDBUpgradeAPI) Snapshots(host string) map[string]apimachineryutilsets.Set[string] {
	f.mu.Lock()
	defer f.mu.Unlock()

	res := map[string]apimachineryutilsets.Set[string]{}
	for tag, keyspaces := range f.snapshots[host] {
		res[tag] = keyspaces.Clone()
	}

	return res
}

// SnapshotsTaken returns how many times a snapshot was taken on the host under the tag, deleted ones included.
func (f *fakeScyllaDBUpgradeAPI) SnapshotsTaken(host, tag string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.snapshotsTaken[host][tag]
}

func (f *fakeScyllaDBUpgradeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/storage_proxy/schema_versions":
		f.schemaAgreementChecks++
		encodeJSON(w, r, []map[string]any{{"key": "schema-version", "value": []string{host}}})

	case r.Method == http.MethodGet && r.URL.Path == "/storage_service/keyspaces":
		encodeJSON(w, r, f.keyspaces)

	case r.Method == http.MethodGet && r.URL.Path == "/storage_service/operation_mode":
		if f.drained.Has(host) {
			encodeJSON(w, r, "DRAINED")
		} else {
			encodeJSON(w, r, "NORMAL")
		}

	case r.Method == http.MethodPost && r.URL.Path == "/storage_service/drain":
		f.drained.Insert(host)
		encodeJSON(w, r, nil)

	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/storage_service/keyspace_flush/"):
		encodeJSON(w, r, nil)

	case r.Method == http.MethodGet && r.URL.Path == "/storage_service/snapshots":
		snapshots := []map[string]any{}
		for tag, keyspaces := range f.snapshots[host] {
			tables := []map[string]any{}
			for _, keyspace := range apimachineryutilsets.List(keyspaces) {
				tables = append(tables, map[string]any{"ks": keyspace, "cf": "table"})
			}
			snapshots = append(snapshots, map[string]any{"key": tag, "value": tables})
		}
		encodeJSON(w, r, snapshots)

	case r.Method == http.MethodPost && r.URL.Path == "/storage_service/snapshots":
		tag := r.URL.Query().Get("tag")
		keyspace := r.URL.Query().Get("kn")
		if f.snapshots[host] == nil {
			f.snapshots[host] = map[string]apimachineryutilsets.Set[string]{}
		}
		if f.snapshots[host][tag] == nil {
			f.snapshots[host][tag] = apimachineryutilsets.New[string]()
		}
		f.snapshots[host][tag].Insert(keyspace)
		if f.snapshotsTaken[host] == nil {
			f.snapshotsTaken[host] = map[string]int{}
		}
		f.snapshotsTaken[host][tag]++
		encodeJSON(w, r, nil)

	case r.Method == http.MethodDelete && r.URL.Path == "/storage_service/snapshots":
		delete(f.snapshots[host], r.URL.Query().Get("tag"))
		encodeJSON(w, r, nil)

	default:
		g.GinkgoWriter.Printf("fake ScyllaDB API: unexpected request %s %s\n", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}
