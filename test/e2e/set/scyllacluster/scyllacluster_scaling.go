// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should support horizontal scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(1))
		diRF1 := insertAndVerifyCQLData(ctx, hosts)
		defer diRF1.Close()

		framework.By("Scaling the ScyllaCluster to 3 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(1))
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.ContainElements(oldHosts))

		verifyCQLData(ctx, diRF1)

		// Statistically, some data should land on the 3rd node that will give us a chance to ensure
		// it was stream correctly when downscaling.
		diRF2 := insertAndVerifyCQLData(ctx, hosts[0:2])
		defer diRF2.Close()

		diRF3 := insertAndVerifyCQLData(ctx, hosts)
		defer diRF3.Close()

		framework.By("Scaling the ScyllaCluster down to 2 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 2}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(2))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.HaveLen(2))
		o.Expect(oldHosts).To(o.ContainElements(hosts))

		verifyCQLData(ctx, diRF1)

		// The 2 nodes out of 3 we used earlier may not be the ones that got left. Although discovery will still
		// make sure the missing one is picked up, let's avoid having a down node in the pool and refresh it.
		err = diRF2.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verifyCQLData(ctx, diRF2)

		podName := naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc) + "-1"
		svcName := podName
		framework.By("Marking ScyllaCluster node #2 (%s) for maintenance", podName)
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					naming.NodeMaintenanceLabel: "",
				},
			},
		}
		patch, err := helpers.CreateTwoWayMergePatch(&corev1.Service{}, svc)
		o.Expect(err).NotTo(o.HaveOccurred())
		svc, err = f.KubeClient().CoreV1().Services(sc.Namespace).Patch(
			ctx,
			svcName,
			types.StrategicMergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Manually draining ScyllaCluster node #2 (%s)", podName)
		ec := &corev1.EphemeralContainer{
			TargetContainerName: naming.ScyllaContainerName,
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:            "e2e-drain-scylla",
				Image:           scyllacluster.ImageForCluster(sc),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/usr/bin/nodetool", "drain"},
				Args:            []string{},
			},
		}
		pod, err := utils.RunEphemeralContainerAndWaitForCompletion(ctx, f.KubeClient().CoreV1().Pods(sc.Namespace), podName, ec)
		o.Expect(err).NotTo(o.HaveOccurred())
		ephemeralContainerState := controllerhelpers.FindContainerStatus(pod, ec.Name)
		o.Expect(ephemeralContainerState).NotTo(o.BeNil())
		o.Expect(ephemeralContainerState.State.Terminated).NotTo(o.BeNil())
		o.Expect(ephemeralContainerState.State.Terminated.ExitCode).To(o.BeEquivalentTo(0))

		framework.By("Scaling the ScyllaCluster down to 1 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx5, waitCtx5Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx5Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(2))
		o.Expect(hosts).To(o.HaveLen(1))
		o.Expect(oldHosts).To(o.ContainElements(hosts))

		verifyCQLData(ctx, diRF1)

		framework.By("Scaling the ScyllaCluster back to 3 replicas to make sure there isn't an old (decommissioned) storage in place")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx6, waitCtx6Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx6Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx6, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(1))
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.ContainElements(oldHosts))

		verifyCQLData(ctx, diRF1)
		verifyCQLData(ctx, diRF2)
		verifyCQLData(ctx, diRF3)
	})

	g.It("should support vertical scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		const membersCount = 3
		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = membersCount
		o.Expect(sc.Spec.Datacenter.Racks[0].Resources.Limits).NotTo(o.BeNil())
		sc.Spec.Datacenter.Racks[0].Resources.Limits[corev1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)

		framework.By("Creating a ScyllaCluster with %d members using 1 cpu each", membersCount)
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(membersCount))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Raising CPU limits to 2 cpu each")
		sc.ManagedFields = nil
		sc.ResourceVersion = ""
		sc.Spec.Datacenter.Racks[0].Resources.Limits[corev1.ResourceCPU] = *resource.NewQuantity(2, resource.DecimalSI)
		scData, err := runtime.Encode(scheme.Codecs.LegacyCodec(scyllav1.GroupVersion), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		// TODO: Use generated Apply method when our clients have it.
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.ApplyPatchType, scData, metav1.PatchOptions{
			FieldManager: f.FieldManager(),
			Force:        pointer.Bool(true),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Resources.Limits).To(o.HaveKeyWithValue(
			corev1.ResourceCPU,
			resource.MustParse("2"),
		))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(membersCount))
		o.Expect(hosts).To(o.ConsistOf(oldHosts))

		verifyCQLData(ctx, di)
	})
})
