// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster evictions", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should allow one disruption", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(2))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Allowing the first pod to be evicted")
		e := &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.GetNodeName(sc, 0),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Evict(ctx, e)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Forbidding to evict a second pod")
		e = &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.GetNodeName(sc, 1),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Evict(ctx, e)
		o.Expect(err).Should(o.MatchError("Cannot evict pod as it would violate the pod's disruption budget."))
	})
})
