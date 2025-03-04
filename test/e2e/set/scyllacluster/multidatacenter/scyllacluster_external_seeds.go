// Copyright (C) 2024 ScyllaDB

package multidatacenter

import (
	"context"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("MultiDC cluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should form when external seeds are provided to ScyllaClusters", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		const clusterName = "multi-datacenter-cluster"

		sc0 := f.GetDefaultZonalScyllaClusterWithThreeRacks()
		sc0.Name = clusterName
		sc0.Spec.Datacenter.Name = "dc0"

		ns0, ns0Client, ok := f.Cluster(0).DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		framework.By("Creating ScyllaCluster #0")
		sc0, err := ns0Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns0.GetName()).Create(ctx, sc0, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster #0 to rollout (RV=%s)", sc0.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc0)
		defer waitCtx1Cancel()
		sc0, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, ns0Client.ScyllaClient().ScyllaV1().ScyllaClusters(sc0.Namespace), sc0.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, ns0Client.KubeClient(), ns0Client.ScyllaClient(), sc0)
		scyllaclusterverification.WaitForFullQuorum(ctx, ns0Client.KubeClient().CoreV1(), sc0)

		hosts0, hostIDs0, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, ns0Client.KubeClient().CoreV1(), sc0)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts0).To(o.HaveLen(int(utils.GetMemberCount(sc0))))
		o.Expect(hostIDs0).To(o.HaveLen(int(utils.GetMemberCount(sc0))))

		di0 := scyllaclusterverification.InsertAndVerifyCQLData(ctx, hosts0)
		defer di0.Close()

		seeds0, err := utils.GetBroadcastAddresses(ctx, ns0Client.KubeClient().CoreV1(), sc0)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(seeds0).To(o.HaveLen(int(utils.GetMemberCount(sc0))))

		sc1 := f.GetDefaultZonalScyllaClusterWithThreeRacks()
		sc1.Name = clusterName
		sc1.Spec.Datacenter.Name = "dc1"
		sc1.Spec.ExternalSeeds = append(make([]string, 0, len(seeds0)), seeds0...)

		framework.By("Creating namespace in cluster #1")
		ns1, ns1Client := f.Cluster(1).CreateUserNamespace(ctx)

		framework.By("Creating ScyllaCluster #1")
		sc1, err = ns1Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns1.GetName()).Create(ctx, sc1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster #1 to rollout (RV=%s)", sc1.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterRollout(ctx, sc1)
		defer waitCtx2Cancel()
		sc1, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, ns1Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns1.GetName()), sc1.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, ns1Client.KubeClient(), ns1Client.ScyllaClient(), sc1)

		framework.By("Verifying a multi-datacenter cluster was formed with ScyllaCluster #0")
		dcClientMap := make(map[string]corev1client.CoreV1Interface, 2)
		dcClientMap[sc0.Spec.Datacenter.Name] = ns0Client.KubeClient().CoreV1()
		dcClientMap[sc1.Spec.Datacenter.Name] = ns1Client.KubeClient().CoreV1()

		scyllaclusterverification.WaitForFullMultiDCQuorum(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc0, sc1})

		hostsByDC, hostIDsByDC, err := utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc0, sc1})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hostsByDC).To(o.HaveKey(sc0.Spec.Datacenter.Name))
		o.Expect(hostsByDC).To(o.HaveKey(sc1.Spec.Datacenter.Name))
		o.Expect(hostIDsByDC[sc0.Spec.Datacenter.Name]).To(o.ConsistOf(hostIDs0))
		o.Expect(hostIDsByDC[sc1.Spec.Datacenter.Name]).To(o.HaveLen(int(utils.GetMemberCount(sc1))))

		hostIDs1 := hostIDsByDC[sc1.Spec.Datacenter.Name]

		di1 := scyllaclusterverification.InsertAndVerifyCQLDataByDC(ctx, hostsByDC)
		defer di1.Close()

		framework.By("Verifying data of datacenter %q", sc0.Spec.Datacenter.Name)
		scyllaclusterverification.VerifyCQLData(ctx, di0)

		framework.By("Verifying datacenter allocation of hosts")
		scyllaClient, _, err := utils.GetScyllaClient(ctx, ns1Client.KubeClient().CoreV1(), sc1)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		for expectedDC, hosts := range hostsByDC {
			for _, host := range hosts {
				gotDC, err := scyllaClient.GetSnitchDatacenter(ctx, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(gotDC).To(o.Equal(expectedDC))
			}
		}

		seeds1, err := utils.GetBroadcastAddresses(ctx, ns1Client.KubeClient().CoreV1(), sc1)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(seeds1).To(o.HaveLen(int(utils.GetMemberCount(sc1))))

		sc2 := f.GetDefaultZonalScyllaClusterWithThreeRacks()
		sc2.Name = clusterName
		sc2.Spec.Datacenter.Name = "dc3"
		sc2.Spec.ExternalSeeds = append(append(make([]string, 0, len(seeds0)+len(seeds1)), seeds0...), seeds1...)

		framework.By("Creating namespace in cluster #2")
		ns2, ns2Client := f.Cluster(2).CreateUserNamespace(ctx)

		framework.By("Creating ScyllaCluster #2")
		sc2, err = ns2Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns2.GetName()).Create(ctx, sc2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster #2 to rollout (RV=%s)", sc2.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForMultiDatacenterRollout(ctx, sc2)
		defer waitCtx3Cancel()
		sc2, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, ns2Client.ScyllaClient().ScyllaV1().ScyllaClusters(sc2.Namespace), sc2.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, ns2Client.KubeClient(), ns2Client.ScyllaClient(), sc2)

		framework.By("Verifying a multi-datacenter cluster was formed with ScyllaClusters #0 and #1")
		dcClientMap = make(map[string]corev1client.CoreV1Interface, 3)
		dcClientMap[sc0.Spec.Datacenter.Name] = ns0Client.KubeClient().CoreV1()
		dcClientMap[sc1.Spec.Datacenter.Name] = ns1Client.KubeClient().CoreV1()
		dcClientMap[sc2.Spec.Datacenter.Name] = ns2Client.KubeClient().CoreV1()

		scyllaclusterverification.WaitForFullMultiDCQuorum(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc0, sc1, sc2})

		hostsByDC, hostIDsByDC, err = utils.GetBroadcastRPCAddressesAndUUIDsByDC(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc0, sc1, sc2})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hostsByDC).To(o.HaveKey(sc0.Spec.Datacenter.Name))
		o.Expect(hostsByDC).To(o.HaveKey(sc1.Spec.Datacenter.Name))
		o.Expect(hostsByDC).To(o.HaveKey(sc2.Spec.Datacenter.Name))
		o.Expect(hostIDsByDC[sc0.Spec.Datacenter.Name]).To(o.ConsistOf(hostIDs0))
		o.Expect(hostIDsByDC[sc1.Spec.Datacenter.Name]).To(o.ConsistOf(hostIDs1))
		o.Expect(hostIDsByDC[sc2.Spec.Datacenter.Name]).To(o.HaveLen(int(utils.GetMemberCount(sc2))))

		di2 := scyllaclusterverification.InsertAndVerifyCQLDataByDC(ctx, hostsByDC)
		defer di2.Close()

		framework.By("Verifying data of datacenter %q", sc0.Spec.Datacenter.Name)
		scyllaclusterverification.VerifyCQLData(ctx, di0)

		framework.By("Verifying data of datacenter %q", sc1.Spec.Datacenter.Name)
		scyllaclusterverification.VerifyCQLData(ctx, di1)

		framework.By("Verifying datacenter allocation of hosts")
		scyllaClient, _, err = utils.GetScyllaClient(ctx, ns2Client.KubeClient().CoreV1(), sc2)
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
