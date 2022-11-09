// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
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
		waitCtx, waitCtxCancel := v1utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = v1utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(2))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Verifying annotations")
		scyllaClient, _, err := v1utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		svcs, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: v1utils.GetMemberServiceSelector(sc.Name).String(),
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
