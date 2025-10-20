// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/tools"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Despite the sysctl configuration being a part of ScyllaCluster's API, it configures kernel parameters on the host.
// Therefore, the test is disruptive and needs to run serially.
var _ = g.Describe("ScyllaCluster sysctl", framework.Serial, framework.NotSupportedOnKind, func() {
	f := framework.NewFramework("scyllacluster")

	// Delete any NodeConfigs before each test to ensure we don't race against NodeConfig's sysctl functionality.
	// NodeConfigs are restored after the test.
	g.JustBeforeEach(func(ctx g.SpecContext) {
		ncs, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, nc := range ncs.Items {
			rc := framework.NewRestoringCleaner(
				ctx,
				f.AdminClientConfig(),
				f.KubeAdminClient(),
				f.DynamicAdminClient(),
				collect.ResourceInfo{
					Resource: schema.GroupVersionResource{
						Group:    "scylla.scylladb.com",
						Version:  "v1alpha1",
						Resource: "nodeconfigs",
					},
					Scope: meta.RESTScopeRoot,
				},
				nc.Namespace,
				nc.Name,
				framework.RestoreStrategyRecreate,
			)
			f.AddCleaners(rc)
			rc.DeleteObject(ctx, false)
		}
	})

	g.It("should set container sysctl", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		fsAIOMaxNRKey := "fs.aio-max-nr"
		fsAIOMaxNRValue := 2424242 // A unique value.
		o.Expect(sc.Spec.Sysctls).To(o.BeEmpty())
		sc.Spec.Sysctls = []string{fmt.Sprintf("%s=%d", fsAIOMaxNRKey, fsAIOMaxNRValue)}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Checking the sysctl value")
		podName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRackForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc))
		stdout, stderr, err := tools.PodExec(
			f.KubeClient().CoreV1().RESTClient(),
			f.ClientConfig(),
			f.Namespace(),
			podName,
			naming.ScyllaDBIgnitionContainerName,
			[]string{"/usr/sbin/sysctl", "--values", fsAIOMaxNRKey},
			nil,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stderr).To(o.BeEmpty())
		o.Expect(stdout).To(o.Equal(fmt.Sprintf("%d\n", fsAIOMaxNRValue)))
	})
})
