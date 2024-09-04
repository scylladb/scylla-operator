// Copyright (c) 2022 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster HostID", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should be reflected as a Service annotation", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(2))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Verifying annotations")
		scyllaClient, _, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		svcs, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: utils.GetMemberServiceSelector(sc).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, svc := range svcs.Items {
			host, err := utils.GetBroadcastRPCAddress(ctx, f.KubeClient().CoreV1(), sc, &svc)
			o.Expect(err).NotTo(o.HaveOccurred())

			hostID, err := scyllaClient.GetLocalHostId(ctx, host, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(hostID).NotTo(o.BeEmpty())

			o.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.HostIDAnnotation, hostID))
		}
	})
})
