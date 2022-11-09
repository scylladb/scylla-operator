// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster evictions", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should allow one disruption", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 2

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
		o.Expect(hosts).To(o.HaveLen(2))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Allowing the first pod to be evicted")
		e := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1utils.GetNodeName(sc, 0),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).EvictV1(ctx, e)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Forbidding to evict a second pod")
		e = &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1utils.GetNodeName(sc, 1),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).EvictV1(ctx, e)
		o.Expect(err).Should(o.MatchError("Cannot evict pod as it would violate the pod's disruption budget."))
	})
})
