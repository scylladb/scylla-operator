// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/tools"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster sysctl", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should set container sysctl", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		fsAIOMaxNRKey := "fs.aio-max-nr"
		fsAIOMaxNRValue := 2424242 // A unique value.
		o.Expect(sc.Spec.Sysctls).To(o.BeEmpty())
		sc.Spec.Sysctls = []string{fmt.Sprintf("%s=%d", fsAIOMaxNRKey, fsAIOMaxNRValue)}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := insertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Checking the sysctl value")
		podName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc))
		stdout, stderr, err := tools.PodExec(
			f.KubeClient().CoreV1().RESTClient(),
			f.ClientConfig(),
			f.Namespace(),
			podName,
			"scylla",
			[]string{"/usr/sbin/sysctl", "--values", fsAIOMaxNRKey},
			nil,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stderr).To(o.BeEmpty())
		o.Expect(stdout).To(o.Equal(fmt.Sprintf("%d\n", fsAIOMaxNRValue)))
	})
})
