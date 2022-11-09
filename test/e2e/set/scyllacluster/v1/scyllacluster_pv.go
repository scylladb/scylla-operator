// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster Orphaned PV", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should replace a node with orphaned PV", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 3
		sc.Spec.AutomaticOrphanedNodeCleanup = true

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(3))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Simulating a PV on node that's gone")
		stsName := fmt.Sprintf("%s-%s-%s", sc.Name, sc.Spec.Datacenter.Name, sc.Spec.Datacenter.Racks[0].Name)
		podName := fmt.Sprintf("%s-%d", stsName, sc.Spec.Datacenter.Racks[0].Members-1)
		pvcName := naming.PVCNameForPod(podName)

		pvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pvc.Spec.VolumeName).NotTo(o.BeEmpty())

		pv, err := f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

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

		pv, err = f.KubeAdminClient().CoreV1().PersistentVolumes().Patch(ctx, pv.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pv.Spec.NodeAffinity).NotTo(o.BeNil())

		// We are not listening to PV changes, so we will make a dummy edit on the ScyllaCluster.
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.MergePatchType,
			[]byte(`{"metadata": {"annotations": {"foo": "bar"} } }`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the PVC to be replaced")
		waitCtx2, waitCtx2Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		pvc, err = utils.WaitForPVCState(waitCtx2, f.KubeClient().CoreV1(), pvc.Namespace, pvc.Name, utils.WaitForStateOptions{TolerateDelete: true}, func(freshPVC *corev1.PersistentVolumeClaim) (bool, error) {
			return freshPVC.UID != pvc.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to observe the degradation")
		waitCtx3, waitCtx3Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, func(sc *scyllav1.ScyllaCluster) (bool, error) {
			rolledOut, err := v1utils.IsScyllaClusterRolledOut(sc)
			return !rolledOut, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy (RV=%s)", sc.ResourceVersion)
		waitCtx4, waitCtx4Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(len(oldHosts)))
		o.Expect(hosts).NotTo(o.ConsistOf(oldHosts))
		err = di.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verifyCQLData(ctx, di)
	})
})
