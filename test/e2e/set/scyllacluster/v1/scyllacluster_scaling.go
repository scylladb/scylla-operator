// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should support scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout")
		waitCtx1, waitCtx1Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
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

		framework.By("Waiting for the ScyllaCluster to rollout")
		waitCtx2, waitCtx2Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

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

		framework.By("Waiting for the ScyllaCluster to rollout")
		waitCtx3, waitCtx3Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

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

		podName := fmt.Sprintf("%s-%s-%s-1", sc.Name, sc.Spec.Datacenter.Name, sc.Spec.Datacenter.Racks[0].Name)
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
				Image:           fmt.Sprintf("%s:%s", sc.Spec.Repository, sc.Spec.Version),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/usr/bin/nodetool", "drain"},
				Args:            []string{},
			},
		}
		pod, err := v1utils.RunEphemeralContainerAndWaitForCompletion(ctx, f.KubeClient().CoreV1().Pods(sc.Namespace), podName, ec)
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

		framework.By("Waiting for the ScyllaCluster to rollout")
		waitCtx5, waitCtx5Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx5Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

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

		framework.By("Waiting for the ScyllaCluster to rollout")
		waitCtx6, waitCtx6Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx6Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx6, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(1))
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.ContainElements(oldHosts))

		verifyCQLData(ctx, diRF1)
		verifyCQLData(ctx, diRF2)
		verifyCQLData(ctx, diRF3)
	})
})
