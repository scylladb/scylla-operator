// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func addQuantity(lhs resource.Quantity, rhs resource.Quantity) *resource.Quantity {
	res := lhs.DeepCopy()
	res.Add(rhs)

	// Pre-cache the string so DeepEqual works.
	_ = res.String()

	return &res
}

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should rolling restart cluster when forceRedeploymentReason is changed", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		initialPodUID := pod.UID
		framework.Infof("Initial pod %q UID is %q", pod.Name, initialPodUID)

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/forceRedeploymentReason", "value": "foo"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		pod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(pod.UID).NotTo(o.Equal(initialPodUID))
	})

	g.It("should reconcile resource changes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		type resourcePair struct {
			cpu    resource.Quantity
			memory resource.Quantity
		}

		for _, rp := range []resourcePair{
			{
				cpu: resource.MustParse("5"),
			},
			{
				cpu: resource.MustParse("2"),
			},
		} {
			framework.By("Adding %q cpu, %q memory to pod resources", rp.cpu, rp.memory)
			oldResources := *sc.Spec.Datacenter.Racks[0].Resources.DeepCopy()
			newResources := *oldResources.DeepCopy()
			o.Expect(newResources.Requests).To(o.HaveKey(corev1.ResourceCPU))
			o.Expect(newResources.Requests).To(o.HaveKey(corev1.ResourceMemory))
			o.Expect(newResources.Limits).To(o.HaveKey(corev1.ResourceCPU))
			o.Expect(newResources.Limits).To(o.HaveKey(corev1.ResourceMemory))

			newResources.Limits[corev1.ResourceCPU] = rp.cpu
			o.Expect(newResources.Limits).NotTo(o.BeEquivalentTo(oldResources.Limits))
			o.Expect(newResources).NotTo(o.BeEquivalentTo(oldResources))

			newResourcesJSON, err := json.Marshal(newResources)
			o.Expect(err).NotTo(o.HaveOccurred())

			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
				ctx,
				sc.Name,
				types.JSONPatchType,
				[]byte(fmt.Sprintf(
					`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources", "value": %s}]`,
					newResourcesJSON,
				)),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sc.Spec.Datacenter.Racks[0].Resources).To(o.BeEquivalentTo(newResources))

			framework.By("Waiting for the ScyllaCluster to redeploy")
			waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
			defer waitCtx2Cancel()
			sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaCluster(ctx, f.KubeClient(), sc)
			waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

			oldHosts := hosts
			oldHostIDs := hostIDs
			hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(hosts).To(o.HaveLen(len(oldHosts)))
			o.Expect(hostIDs).To(o.ConsistOf(oldHostIDs))

			// Reset hosts as the client won't be able to discover a single node after rollout.
			err = di.SetClientEndpoints(hosts)
			o.Expect(err).NotTo(o.HaveOccurred())
			verifyCQLData(ctx, di)
		}

		framework.By("Scaling the ScyllaCluster up to create a new replica")
		oldMembers := sc.Spec.Datacenter.Racks[0].Members
		newMebmers := oldMembers + 1
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`,
				newMebmers,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.Equal(newMebmers))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHostIDs := hostIDs
		_, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)

		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHostIDs).To(o.HaveLen(int(oldMembers)))
		o.Expect(hostIDs).To(o.HaveLen(int(newMebmers)))
		o.Expect(hostIDs).To(o.ContainElements(oldHostIDs))
		verifyCQLData(ctx, di)
	})
})
