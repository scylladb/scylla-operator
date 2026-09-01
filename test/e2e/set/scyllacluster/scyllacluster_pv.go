// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var persistentVolumeResourceInfo = collect.ResourceInfo{
	Resource: schema.GroupVersionResource{
		Group:    corev1.GroupName,
		Version:  corev1.SchemeGroupVersion.Version,
		Resource: "persistentvolumes",
	},
	Scope: meta.RESTScopeRoot,
}

// This test simulates the loss of a Node by pointing the nodeAffinity of a PersistentVolume backing one of the ScyllaDB
// nodes at a Node that doesn't exist, which triggers the orphaned PV controller to initiate the replacement procedure.
// Updating an already set nodeAffinity requires the MutablePVNodeAffinity feature gate, alpha as of Kubernetes 1.35.
// It is only a part of relevant suites running in KinD.
// It can be added to the remaining suites when they have this feature gate enabled.
var _ = g.Describe("ScyllaCluster Orphaned PV controller", framework.SuiteKindFast, framework.SuiteKindClusterTopology, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should replace a node with orphaned PV", func(ctx g.SpecContext) {
		// Use 3 racks of 1 member each instead of a single rack of 3 members, so that the default keyspace
		// replication of a replica per rack replicates the test data across 3 nodes.
		sc := f.GetDefaultZonalScyllaClusterWithThreeRacks()
		sc.Spec.AutomaticOrphanedNodeCleanup = true

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, _, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(3))
		// The node's storage is discarded and the replacement streams from the other replicas, so the data only
		// survives if it is replicated. The default replication puts a replica on each of the 3 racks.
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Simulating a PV on node that's gone")
		podName := naming.PodNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, int(sc.Spec.Datacenter.Racks[0].Members-1))
		pvcName := naming.PVCNameForPod(podName)

		pvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pvc.Spec.VolumeName).NotTo(o.BeEmpty())

		pv, err := f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Some provisioners reclaim volumes from the Node resolved from the PV's node affinity, so leaving the affinity
		// pointing at a Node that doesn't exist can leave the volume, and the data backing it, behind.
		// PersistentVolumes are cluster scoped, so they aren't covered by the namespace teardown. Restore the original
		// node affinity to let the volume be reclaimed.
		f.AddCleaners(framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			persistentVolumeResourceInfo,
			pv.Namespace,
			pv.Name,
			framework.RestoreStrategyUpdateIfExists,
		))

		pvCopy := pv.DeepCopy()
		pvCopy.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      corev1.LabelHostname,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"this-node-does-not-exist-42"},
							},
						},
					},
				},
			},
		}

		patchData, err := controllerhelpers.GenerateMergePatch(pv, pvCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = f.KubeAdminClient().CoreV1().PersistentVolumes().Patch(ctx, pv.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Orphaned PV controller is not handling PV events directly, so we patch the SC with a dummy annotation to trigger reconciliation.
		framework.Infof("Annotating the ScyllaCluster to trigger the orphaned PV controller reconciliation")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.MergePatchType,
			[]byte(`{"metadata": {"annotations": {"foo": "bar"} } }`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the PVC to be replaced")
		pvcReplacementCtx, pvcReplacementCtxCancel := utils.ContextForRollout(ctx, sc)
		defer pvcReplacementCtxCancel()
		pvc, err = controllerhelpers.WaitForPVCState(pvcReplacementCtx, f.KubeClient().CoreV1().PersistentVolumeClaims(pvc.Namespace), pvc.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(freshPVC *corev1.PersistentVolumeClaim) (bool, error) {
			return freshPVC.UID != pvc.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to observe the degradation")
		degradationCtx, degradationCtxCancel := utils.ContextForRollout(ctx, sc)
		defer degradationCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(degradationCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, func(sc *scyllav1.ScyllaCluster) (bool, error) {
			rolledOut, err := utils.IsScyllaClusterRolledOut(sc)
			return !rolledOut, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		postReplacementRolloutCtx, postReplacementRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer postReplacementRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(postReplacementRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err = utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(3))
		verification.VerifyCQLData(ctx, di)
	})
})
