// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/load"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = g.Describe("ScyllaCluster re-bootstrap", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should re-bootstrap from old PVCs", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimout)
		defer cancel()

		sc := scyllaclusterfixture.BasicScyllaCluster.ReadOrFail()

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

		framework.By("Writing data")
		hosts, err := getHosts(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		l, err := load.NewLoad(hosts...)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer l.Close()

		err = l.Write(10, int(sc.Spec.Datacenter.Racks[0].Members))
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Deleting ScyllaCluster")
		var gracePeriodSeconds int64 = 0
		var propagationPolicy = metav1.DeletePropagationForeground
		err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Delete(ctx, sc.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriodSeconds,
			PropagationPolicy:  &propagationPolicy,
			Preconditions: &metav1.Preconditions{
				UID: &sc.UID,
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx2, waitCtx2Cancel := context.WithCancel(context.Background())
		defer waitCtx2Cancel()
		err = framework.WaitForObjectDeletion(waitCtx2, f.DynamicAdminClient(), schema.GroupVersionResource{Group: "scylla.scylladb.com", Version: "v1", Resource: "scyllaclusters"}, f.Namespace(), sc.Name, sc.UID)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Redeploying ScyllaCluster")
		sc = scyllaclusterfixture.BasicScyllaCluster.ReadOrFail()
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to redeploy")
		waitCtx3, waitCtx3Cancel := contextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = waitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Verifying data consistency")
		hosts, err = getHosts(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = l.Reset(hosts...)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = l.VerifyRead()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
