/*
 * // Copyright (C) 2024 ScyllaDB
 */

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("MultiDC cluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("multi-datacenter-scyllacluster")

	g.It("should form when external seeds are provided to ScyllaClusters", func() {
		cs := f.Clusters()
		o.Expect(len(cs)).To(o.BeNumerically(">=", 3))
		c1, c2, c3 := cs[0], cs[1], cs[2]

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc1 := f.GetDefaultScyllaCluster()
		sc1.Name = "basic-cluster"
		sc1.Spec.Datacenter.Name = "us-central1"
		sc1.Spec.Datacenter.Racks[0].Name = "us-central1-a"
		sc1.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating first ScyllaCluster")
		sc1, err := c1.ScyllaClient().ScyllaV1().ScyllaClusters(c1.Namespace()).Create(ctx, sc1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the first ScyllaCluster to rollout (RV=%s)", sc1.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc1)
		defer waitCtx1Cancel()
		sc1, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, c1.ScyllaClient().ScyllaV1().ScyllaClusters(sc1.Namespace), sc1.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, c1.KubeClient(), sc1)
		waitForFullQuorum(ctx, c1.KubeClient().CoreV1(), sc1)

		hosts1, hostIDs1, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, c1.KubeClient().CoreV1(), sc1)
		o.Expect(err).NotTo(o.HaveOccurred())
		di1 := insertAndVerifyCQLData(ctx, hosts1)
		defer di1.Close()

		sc2 := f.GetDefaultScyllaCluster()
		sc2.Name = "basic-cluster"
		sc2.Spec.Datacenter.Name = "us-west1"
		sc2.Spec.Datacenter.Racks[0].Name = "us-west1-a"
		sc2.Spec.Datacenter.Racks[0].Members = 3
		sc2.Spec.ExternalSeeds = append(make([]string, 0, len(hosts1)), hosts1...)

		framework.By("Creating second ScyllaCluster")
		sc2, err = c2.ScyllaClient().ScyllaV1().ScyllaClusters(c2.Namespace()).Create(ctx, sc2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the second ScyllaCluster to rollout (RV=%s)", sc2.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc2)
		defer waitCtx2Cancel()
		sc2, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, c2.ScyllaClient().ScyllaV1().ScyllaClusters(sc2.Namespace), sc2.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, c2.KubeClient(), sc2)

		framework.By("Verifying a multi-datacenter cluster was formed with the first ScyllaCluster")
		dcClientMap := make(map[string]corev1client.CoreV1Interface, 2)
		dcClientMap[sc1.Spec.Datacenter.Name] = c1.KubeClient().CoreV1()
		dcClientMap[sc2.Spec.Datacenter.Name] = c2.KubeClient().CoreV1()

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
		scyllaClient, _, err := utils.GetScyllaClient(ctx, c2.KubeClient().CoreV1(), sc2)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		for expectedDC, hosts := range hostsByDC {
			for _, host := range hosts {
				gotDC, err := scyllaClient.GetSnitchDatacenter(ctx, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(gotDC).To(o.Equal(expectedDC))
			}
		}

		sc3 := f.GetDefaultScyllaCluster()
		sc3.Name = "basic-cluster"
		sc3.Spec.Datacenter.Name = "us-east1"
		sc3.Spec.Datacenter.Racks[0].Name = "us-east1-b"
		sc3.Spec.Datacenter.Racks[0].Members = 3
		sc3.Spec.ExternalSeeds = append(make([]string, 0, len(hosts1)), hosts1...)

		framework.By("Creating third ScyllaCluster")
		sc3, err = c3.ScyllaClient().ScyllaV1().ScyllaClusters(c3.Namespace()).Create(ctx, sc3, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the third ScyllaCluster to rollout (RV=%s)", sc3.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc3)
		defer waitCtx3Cancel()
		sc3, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, c3.ScyllaClient().ScyllaV1().ScyllaClusters(sc3.Namespace), sc3.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, c3.KubeClient(), sc3)

		framework.By("Verifying a multi-datacenter cluster was formed with the first two ScyllaClusters")
		dcClientMap = make(map[string]corev1client.CoreV1Interface, 3)
		dcClientMap[sc1.Spec.Datacenter.Name] = c1.KubeClient().CoreV1()
		dcClientMap[sc2.Spec.Datacenter.Name] = c2.KubeClient().CoreV1()
		dcClientMap[sc3.Spec.Datacenter.Name] = c3.KubeClient().CoreV1()

		waitForFullMultiDCQuorum(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc1, sc2, sc3})

		hostsByDC, hostIDsByDC, err = utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc1, sc2, sc3})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hostsByDC).To(o.HaveKey(sc1.Spec.Datacenter.Name))
		o.Expect(hostsByDC).To(o.HaveKey(sc2.Spec.Datacenter.Name))
		o.Expect(hostsByDC).To(o.HaveKey(sc3.Spec.Datacenter.Name))
		o.Expect(hostIDsByDC[sc1.Spec.Datacenter.Name]).To(o.ConsistOf(hostIDs1))
		o.Expect(hostIDsByDC[sc2.Spec.Datacenter.Name]).To(o.HaveLen(int(utils.GetMemberCount(sc2))))
		o.Expect(hostIDsByDC[sc3.Spec.Datacenter.Name]).To(o.HaveLen(int(utils.GetMemberCount(sc3))))

		di3 := insertAndVerifyCQLDataByDC(ctx, hostsByDC)
		defer di3.Close()

		framework.By("Verifying data of datacenter %q", sc1.Spec.Datacenter.Name)
		verifyCQLData(ctx, di1)

		framework.By("Verifying data of datacenter %q", sc2.Spec.Datacenter.Name)
		verifyCQLData(ctx, di2)

		framework.By("Verifying datacenter allocation of hosts")
		scyllaClient, _, err = utils.GetScyllaClient(ctx, c3.KubeClient().CoreV1(), sc3)
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
