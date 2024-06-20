// Copyright (C) 2024 ScyllaDB

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

var _ = g.Describe("MultiDC cluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should form when external seeds are provided to ScyllaClusters", func() {
		ctx, cancel := context.WithTimeout(context.Background(), multiDatacenterTestTimeout)
		defer cancel()

		sc1 := f.GetDefaultScyllaCluster()
		sc1.Name = "multi-datacenter-cluster"
		sc1.Spec.Datacenter.Name = "dc1"
		sc1.Spec.Datacenter.Racks = []scyllav1.RackSpec{
			{
				Name:    "a",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			{
				Name:    "b",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			{
				Name:    "c",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		}
		sc1.Spec.ScyllaArgs = "--enable-repair-based-node-ops=false"
		sc1.Spec.Sysctls = []string{"fs.aio-max-nr=30000000"}

		framework.By("Ensuring default namespace presence in the first cluster")
		ns1, ns1Client, ok := f.Cluster(0).DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		framework.By("Creating first ScyllaCluster")
		sc1, err := ns1Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns1.GetName()).Create(ctx, sc1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the first ScyllaCluster to rollout (RV=%s)", sc1.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc1)
		defer waitCtx1Cancel()
		sc1, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, ns1Client.ScyllaClient().ScyllaV1().ScyllaClusters(sc1.Namespace), sc1.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, ns1Client.KubeClient(), sc1)
		waitForFullQuorum(ctx, ns1Client.KubeClient().CoreV1(), sc1)

		hosts1, hostIDs1, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, ns1Client.KubeClient().CoreV1(), sc1)
		o.Expect(err).NotTo(o.HaveOccurred())
		di1 := insertAndVerifyCQLData(ctx, hosts1)
		defer di1.Close()

		sc2 := f.GetDefaultScyllaCluster()
		sc2.Name = "multi-datacenter-cluster"
		sc2.Spec.Datacenter.Name = "dc2"
		sc2.Spec.Datacenter.Racks = []scyllav1.RackSpec{
			{
				Name:    "a",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			{
				Name:    "b",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			{
				Name:    "c",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		}
		sc2.Spec.ExternalSeeds = append(make([]string, 0, len(hosts1)), hosts1...)
		sc2.Spec.ScyllaArgs = "--enable-repair-based-node-ops=false"
		sc2.Spec.Sysctls = []string{"fs.aio-max-nr=30000000"}

		framework.By("Creating namespace in the second cluster")
		ns2, ns2Client := f.Cluster(1).CreateUserNamespace(ctx)

		framework.By("Creating second ScyllaCluster")
		sc2, err = ns2Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns2.GetName()).Create(ctx, sc2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the second ScyllaCluster to rollout (RV=%s)", sc2.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForMultiDatacenterRollout(ctx, sc2)
		defer waitCtx2Cancel()
		sc2, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, ns2Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns2.GetName()), sc2.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, ns2Client.KubeClient(), sc2)

		framework.By("Verifying a multi-datacenter cluster was formed with the first ScyllaCluster")
		dcClientMap := make(map[string]corev1client.CoreV1Interface, 2)
		dcClientMap[sc1.Spec.Datacenter.Name] = ns1Client.KubeClient().CoreV1()
		dcClientMap[sc2.Spec.Datacenter.Name] = ns2Client.KubeClient().CoreV1()

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
		scyllaClient, _, err := utils.GetScyllaClient(ctx, ns2Client.KubeClient().CoreV1(), sc2)
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
		sc3.Name = "multi-datacenter-cluster"
		sc3.Spec.Datacenter.Name = "dc3"
		sc3.Spec.Datacenter.Racks = []scyllav1.RackSpec{
			{
				Name:    "a",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			{
				Name:    "b",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			{
				Name:    "c",
				Members: 1,
				Storage: scyllav1.Storage{
					Capacity: "1Gi",
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
		}
		sc3.Spec.ExternalSeeds = append(make([]string, 0, len(hosts1)), hosts1...)
		sc3.Spec.ScyllaArgs = "--enable-repair-based-node-ops=false"
		sc3.Spec.Sysctls = []string{"fs.aio-max-nr=30000000"}

		framework.By("Creating namespace in the third cluster")
		ns3, ns3Client := f.Cluster(2).CreateUserNamespace(ctx)

		framework.By("Creating third ScyllaCluster")
		sc3, err = ns3Client.ScyllaClient().ScyllaV1().ScyllaClusters(ns3.GetName()).Create(ctx, sc3, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the third ScyllaCluster to rollout (RV=%s)", sc3.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForMultiDatacenterRollout(ctx, sc3)
		defer waitCtx3Cancel()
		sc3, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, ns3Client.ScyllaClient().ScyllaV1().ScyllaClusters(sc3.Namespace), sc3.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, ns3Client.KubeClient(), sc3)

		framework.By("Verifying a multi-datacenter cluster was formed with the first two ScyllaClusters")
		dcClientMap = make(map[string]corev1client.CoreV1Interface, 3)
		dcClientMap[sc1.Spec.Datacenter.Name] = ns1Client.KubeClient().CoreV1()
		dcClientMap[sc2.Spec.Datacenter.Name] = ns2Client.KubeClient().CoreV1()
		dcClientMap[sc3.Spec.Datacenter.Name] = ns3Client.KubeClient().CoreV1()

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
		scyllaClient, _, err = utils.GetScyllaClient(ctx, ns3Client.KubeClient().CoreV1(), sc3)
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
