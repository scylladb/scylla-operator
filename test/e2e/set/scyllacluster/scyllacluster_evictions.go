// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster evictions", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should allow one disruption", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimout)
		defer cancel()

		sc := scyllaclusterfixture.BasicFastScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaCluster")
		err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := contextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = waitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Allowing the first pod to be evicted")
		e := &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getNodeName(sc, 0),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Evict(ctx, e)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Forbidding to evict a second pod")
		e = &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getNodeName(sc, 1),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Evict(ctx, e)
		o.Expect(err).Should(o.MatchError("Cannot evict pod as it would violate the pod's disruption budget."))
	})
})
