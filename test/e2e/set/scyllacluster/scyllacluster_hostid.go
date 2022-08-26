// Copyright (c) 2022 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster HostID", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should be reflected as a Service annotation", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, utils.GetMemberCount(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

		framework.By("Verifying annotations")
		scyllaClient, _, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		svcs, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: utils.GetMemberServiceSelector(sc.Name).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, svc := range svcs.Items {
			host := svc.Spec.ClusterIP
			o.Expect(host).NotTo(o.Equal(corev1.ClusterIPNone))

			hostID, err := scyllaClient.GetLocalHostId(ctx, host, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(hostID).NotTo(o.BeEmpty())

			o.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.HostIDAnnotation, hostID))
		}
	})
})
