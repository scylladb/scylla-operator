// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster smoke tests", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("basic ScyllaCluster deployment", func() {
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
	})
})
