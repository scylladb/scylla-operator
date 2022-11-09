// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
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

	g.It("should reconcile resource changes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Changing pod resources")
		oldResources := *sc.Spec.Datacenter.Racks[0].Resources.DeepCopy()
		newResources := *oldResources.DeepCopy()
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceMemory))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceMemory))

		newResources.Requests[corev1.ResourceCPU] = *addQuantity(newResources.Requests[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Requests[corev1.ResourceMemory] = *addQuantity(newResources.Requests[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Requests).NotTo(o.BeEquivalentTo(oldResources.Requests))

		newResources.Limits[corev1.ResourceCPU] = *addQuantity(newResources.Limits[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Limits[corev1.ResourceMemory] = *addQuantity(newResources.Limits[corev1.ResourceMemory], resource.MustParse("1Mi"))
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

		framework.By("Waiting for the ScyllaCluster to redeploy (RV=%s)", sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		o.Expect(hosts).To(o.ConsistOf(getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)))
		verifyCQLData(ctx, di)

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

		framework.By("Waiting for the ScyllaCluster to redeploy (RV=%s)", sc.ResourceVersion)
		waitCtx4, waitCtx4Cancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = v1utils.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(int(oldMembers)))
		o.Expect(hosts).To(o.HaveLen(int(newMebmers)))
		o.Expect(hosts).To(o.ContainElements(oldHosts))
		verifyCQLData(ctx, di)
	})
})
