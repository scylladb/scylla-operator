// Copyright (c) 2023 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("MultiDC cluster", func() {
	defer g.GinkgoRecover()

	f1 := framework.NewFramework("scyllacluster")
	f2 := framework.NewFramework("scyllacluster")

	g.It("should form when external seeds are provided to ScyllaClusters", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc1 := f1.GetDefaultScyllaCluster()
		sc1.Name = "basic-cluster"
		sc1.Spec.Datacenter.Name = "us-east-1"
		sc1.Spec.Datacenter.Racks[0].Name = "us-east-1a"
		sc1.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating first ScyllaCluster")
		sc1, err := f1.ScyllaClient().ScyllaV1().ScyllaClusters(f1.Namespace()).Create(ctx, sc1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the first ScyllaCluster to rollout (RV=%s)", sc1.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc1)
		defer waitCtx1Cancel()
		sc1, err = utils.WaitForScyllaClusterState(waitCtx1, f1.ScyllaClient().ScyllaV1(), sc1.Namespace, sc1.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f1.KubeClient(), sc1)
		waitForFullQuorum(ctx, f1.KubeClient().CoreV1(), sc1)

		hosts1, hostIDs1, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f1.KubeClient().CoreV1(), sc1)
		o.Expect(err).NotTo(o.HaveOccurred())
		di1 := insertAndVerifyCQLData(ctx, hosts1)
		defer di1.Close()

		sc2 := f2.GetDefaultScyllaCluster()
		sc2.Name = "basic-cluster"
		sc2.Spec.Datacenter.Name = "us-east-2"
		sc2.Spec.Datacenter.Racks[0].Name = "us-east-2a"
		sc2.Spec.Datacenter.Racks[0].Members = 3
		sc2.Spec.ExternalSeeds = []string{
			naming.CrossNamespaceServiceNameForCluster(sc1),
		}

		framework.By("Creating second ScyllaCluster")
		sc2, err = f2.ScyllaClient().ScyllaV1().ScyllaClusters(f2.Namespace()).Create(ctx, sc2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the second ScyllaCluster to rollout (RV=%s)", sc2.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc2)
		defer waitCtx2Cancel()
		sc2, err = utils.WaitForScyllaClusterState(waitCtx2, f2.ScyllaClient().ScyllaV1(), sc2.Namespace, sc2.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f2.KubeClient(), sc2)

		framework.By("Verifying a multi datacenter cluster was formed with the first ScyllaCluster")
		dcClientMap := make(map[string]corev1client.CoreV1Interface, 2)
		dcClientMap[sc1.Spec.Datacenter.Name] = f1.KubeClient().CoreV1()
		dcClientMap[sc2.Spec.Datacenter.Name] = f2.KubeClient().CoreV1()

		waitForFullMultiDCQuorum(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc1, sc2})

		hostsByDC, hostIDsByDC, err := utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc1, sc2})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hostsByDC).To(o.HaveKey(sc1.Spec.Datacenter.Name))
		o.Expect(hostsByDC).To(o.HaveKey(sc2.Spec.Datacenter.Name))
		o.Expect(hostIDsByDC[sc1.Spec.Datacenter.Name]).To(o.ConsistOf(hostIDs1))
		o.Expect(hostIDsByDC[sc2.Spec.Datacenter.Name]).To(o.HaveLen(int(utils.GetMemberCount(sc2))))

		di2 := insertAndVerifyCQLDataByDC(ctx, hostsByDC)
		defer di2.Close()

		framework.By("Verifying data of datacenter %q", sc1.Spec.Datacenter.Name)
		verifyCQLData(ctx, di1)

		framework.By("Verifying datacenter allocation of hosts")
		scyllaClient, _, err := utils.GetScyllaClient(ctx, f2.KubeClient().CoreV1(), sc2)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		for expectedDC, hosts := range hostsByDC {
			for _, host := range hosts {
				gotDC, err := scyllaClient.GetSnitchDatacenter(ctx, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(gotDC).To(o.Equal(expectedDC))
			}
		}
	})
})
