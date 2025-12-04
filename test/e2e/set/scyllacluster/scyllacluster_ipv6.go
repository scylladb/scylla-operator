// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type ipv6TestEntry struct {
	ipFamily            corev1.IPFamily
	ipFamilyPolicy      corev1.IPFamilyPolicy
	ipFamilies          []corev1.IPFamily
	exposeOptions       *scyllav1.ExposeOptions
	validateIPFamily    func(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster)
	validateScyllaAddrs func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod)
}

var _ = g.Describe("ScyllaCluster IPv6", g.Label(framework.IPv6LabelName), func() {
	f := framework.NewFramework("scyllacluster")

	g.DescribeTable("should support IPv6 with different expose options", func(e *ipv6TestEntry) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 3
		sc.Spec.IPFamily = &e.ipFamily
		sc.Spec.Network = scyllav1.Network{
			DNSPolicy:      corev1.DNSClusterFirst,
			IPFamilyPolicy: &e.ipFamilyPolicy,
			IPFamilies:     e.ipFamilies,
		}
		sc.Spec.ExposeOptions = e.exposeOptions

		framework.By("Creating ScyllaCluster with IP family %s", e.ipFamily)
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCtx, verifyCtxCancel := utils.ContextForRollout(ctx, sc)
		defer verifyCtxCancel()

		framework.By("Verifying IP family configuration")
		e.validateIPFamily(verifyCtx, f, sc)

		framework.By("Verifying ScyllaDB broadcast addresses")
		svcName := naming.MemberServiceNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, 0)
		svc, err := f.KubeClient().CoreV1().Services(sc.Namespace).Get(verifyCtx, svcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err := f.KubeClient().CoreV1().Pods(sc.Namespace).Get(verifyCtx, svcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var scyllaConfigIP string
		if e.ipFamily == corev1.IPv6Protocol {
			for _, podIP := range pod.Status.PodIPs {
				if ip, err := helpers.ParseIP(podIP.IP); err == nil && helpers.IsIPv6(ip) {
					scyllaConfigIP = podIP.IP
					break
				}
			}
		} else {
			scyllaConfigIP = pod.Status.PodIP
		}
		o.Expect(scyllaConfigIP).NotTo(o.BeEmpty(), "Could not find appropriate IP for ScyllaDB config client")

		configClient, err := utils.GetScyllaConfigClient(verifyCtx, f.KubeClient().CoreV1(), sc, scyllaConfigIP)
		o.Expect(err).NotTo(o.HaveOccurred())

		e.validateScyllaAddrs(verifyCtx, configClient, svc, pod)
	},
		g.Entry("IPv6 single-stack with PodIP broadcast", &ipv6TestEntry{
			ipFamily:       corev1.IPv6Protocol,
			ipFamilyPolicy: corev1.IPFamilyPolicySingleStack,
			ipFamilies:     []corev1.IPFamily{corev1.IPv6Protocol},
			exposeOptions: &scyllav1.ExposeOptions{
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
				},
			},
			validateIPFamily: validateIPv6SingleStack,
			validateScyllaAddrs: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				validatePodIPBroadcastAddresses(ctx, configClient, pod, corev1.IPv6Protocol)
			},
		}),
		g.Entry("IPv6 single-stack with ClusterIP broadcast", &ipv6TestEntry{
			ipFamily:       corev1.IPv6Protocol,
			ipFamilyPolicy: corev1.IPFamilyPolicySingleStack,
			ipFamilies:     []corev1.IPFamily{corev1.IPv6Protocol},
			exposeOptions: &scyllav1.ExposeOptions{
				NodeService: &scyllav1.NodeServiceTemplate{
					Type: scyllav1.NodeServiceTypeClusterIP,
				},
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
					},
				},
			},
			validateIPFamily: validateIPv6SingleStack,
			validateScyllaAddrs: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				validateClusterIPBroadcastAddresses(ctx, configClient, svc)
			},
		}),
		g.Entry("dual-stack (IPv4 primary) with PodIP broadcast", &ipv6TestEntry{
			ipFamily:       corev1.IPv4Protocol,
			ipFamilyPolicy: corev1.IPFamilyPolicyPreferDualStack,
			ipFamilies:     []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
			exposeOptions: &scyllav1.ExposeOptions{
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
				},
			},
			validateIPFamily: validateDualStackIPv4Primary,
			validateScyllaAddrs: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				validatePodIPBroadcastAddresses(ctx, configClient, pod, corev1.IPv4Protocol)
			},
		}),
		g.Entry("dual-stack (IPv6 primary) with PodIP broadcast", &ipv6TestEntry{
			ipFamily:       corev1.IPv6Protocol,
			ipFamilyPolicy: corev1.IPFamilyPolicyPreferDualStack,
			ipFamilies:     []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol},
			exposeOptions: &scyllav1.ExposeOptions{
				BroadcastOptions: &scyllav1.NodeBroadcastOptions{
					Nodes: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
					Clients: scyllav1.BroadcastOptions{
						Type: scyllav1.BroadcastAddressTypePodIP,
					},
				},
			},
			validateIPFamily: validateDualStackIPv6Primary,
			validateScyllaAddrs: func(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service, pod *corev1.Pod) {
				validatePodIPBroadcastAddresses(ctx, configClient, pod, corev1.IPv6Protocol)
			},
		}),
	)
})

func validateIPv6SingleStack(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	pods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pods.Items).ToNot(o.BeEmpty())

	for _, pod := range pods.Items {
		framework.By("Verifying pod %q has IPv6 address", pod.Name)

		hasIPv6 := false
		var ipv6Address string
		for _, podIP := range pod.Status.PodIPs {
			if ip, err := helpers.ParseIP(podIP.IP); err == nil && helpers.IsIPv6(ip) {
				hasIPv6 = true
				ipv6Address = podIP.IP
				break
			}
		}
		o.Expect(hasIPv6).To(o.BeTrue(), "Pod should have at least one IPv6 address")
		framework.Infof("Pod %q has IPv6 address: %s", pod.Name, ipv6Address)
	}

	services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, svc := range services.Items {
		framework.By("Verifying service %q IPv6 configuration", svc.Name)

		hasIPv6Family := false
		for _, family := range svc.Spec.IPFamilies {
			if family == corev1.IPv6Protocol {
				hasIPv6Family = true
				break
			}
		}
		o.Expect(hasIPv6Family).To(o.BeTrue(), "Service should have IPv6 in IPFamilies")

		if svc.Spec.ClusterIP != corev1.ClusterIPNone {
			clusterIP, err := helpers.ParseIP(svc.Spec.ClusterIP)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(helpers.IsIPv6(clusterIP)).To(o.BeTrue(), "Service ClusterIP should be IPv6: %s", svc.Spec.ClusterIP)
		}
	}
}

func validateDualStackIPv4Primary(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, svc := range services.Items {
		framework.By("Verifying service %q dual-stack configuration", svc.Name)

		o.Expect(svc.Spec.IPFamilies).To(o.HaveLen(2), "Service should have exactly 2 IP families for dual-stack")
		o.Expect(svc.Spec.IPFamilies[0]).To(o.Equal(corev1.IPv4Protocol), "Primary IP family should be IPv4")
		o.Expect(svc.Spec.IPFamilies[1]).To(o.Equal(corev1.IPv6Protocol), "Secondary IP family should be IPv6")
	}

	pods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		framework.By("Verifying pod %q dual-stack configuration", pod.Name)

		o.Expect(pod.Status.PodIP).ToNot(o.BeEmpty())
		primaryIP, err := helpers.ParseIP(pod.Status.PodIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(helpers.IsIPv4(primaryIP)).To(o.BeTrue(), "Primary pod IP should be IPv4: %s", pod.Status.PodIP)

		var hasIPv4, hasIPv6 bool
		for _, podIP := range pod.Status.PodIPs {
			if ip, err := helpers.ParseIP(podIP.IP); err == nil {
				if helpers.IsIPv4(ip) {
					hasIPv4 = true
				}
				if helpers.IsIPv6(ip) {
					hasIPv6 = true
				}
			}
		}
		o.Expect(hasIPv4).To(o.BeTrue(), "Pod should have at least one IPv4 address")
		o.Expect(hasIPv6).To(o.BeTrue(), "Pod should have at least one IPv6 address")
	}
}

func validateDualStackIPv6Primary(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, svc := range services.Items {
		framework.By("Verifying service %q dual-stack IPv6 primary configuration", svc.Name)

		o.Expect(svc.Spec.IPFamilies).To(o.HaveLen(2), "Service should have exactly 2 IP families for dual-stack")
		o.Expect(svc.Spec.IPFamilies[0]).To(o.Equal(corev1.IPv6Protocol), "Primary IP family should be IPv6")
		o.Expect(svc.Spec.IPFamilies[1]).To(o.Equal(corev1.IPv4Protocol), "Secondary IP family should be IPv4")
	}

	pods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		framework.By("Verifying pod %q dual-stack IPv6 primary configuration", pod.Name)

		o.Expect(pod.Status.PodIP).ToNot(o.BeEmpty())
		primaryIP, err := helpers.ParseIP(pod.Status.PodIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(helpers.IsIPv6(primaryIP)).To(o.BeTrue(), "Primary pod IP should be IPv6: %s", pod.Status.PodIP)

		var hasIPv4, hasIPv6 bool
		for _, podIP := range pod.Status.PodIPs {
			if ip, err := helpers.ParseIP(podIP.IP); err == nil {
				if helpers.IsIPv4(ip) {
					hasIPv4 = true
				}
				if helpers.IsIPv6(ip) {
					hasIPv6 = true
				}
			}
		}
		o.Expect(hasIPv4).To(o.BeTrue(), "Pod should have at least one IPv4 address")
		o.Expect(hasIPv6).To(o.BeTrue(), "Pod should have at least one IPv6 address")
	}
}

func validatePodIPBroadcastAddresses(ctx context.Context, configClient *scyllaclient.ConfigClient, pod *corev1.Pod, expectedFamily corev1.IPFamily) {
	broadcastAddress, err := configClient.BroadcastAddress(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Infof("Pod %q broadcast_address: %s", pod.Name, broadcastAddress)

	broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Infof("Pod %q broadcast_rpc_address: %s", pod.Name, broadcastRPCAddress)

	bAddr, err := helpers.ParseIP(broadcastAddress)
	o.Expect(err).NotTo(o.HaveOccurred())

	bRPCAddr, err := helpers.ParseIP(broadcastRPCAddress)
	o.Expect(err).NotTo(o.HaveOccurred())

	if expectedFamily == corev1.IPv6Protocol {
		o.Expect(helpers.IsIPv6(bAddr)).To(o.BeTrue(), "broadcast_address should be IPv6: %s", broadcastAddress)
		o.Expect(helpers.IsIPv6(bRPCAddr)).To(o.BeTrue(), "broadcast_rpc_address should be IPv6: %s", broadcastRPCAddress)
	} else {
		o.Expect(helpers.IsIPv4(bAddr)).To(o.BeTrue(), "broadcast_address should be IPv4: %s", broadcastAddress)
		o.Expect(helpers.IsIPv4(bRPCAddr)).To(o.BeTrue(), "broadcast_rpc_address should be IPv4: %s", broadcastRPCAddress)
	}

	var podIPs []string
	for _, podIP := range pod.Status.PodIPs {
		podIPs = append(podIPs, podIP.IP)
	}
	o.Expect(podIPs).To(o.ContainElement(broadcastAddress), "broadcast_address should be one of pod's IPs")
	o.Expect(podIPs).To(o.ContainElement(broadcastRPCAddress), "broadcast_rpc_address should be one of pod's IPs")

	framework.Infof("Verified broadcast addresses use %s and are from pod IPs", expectedFamily)
}

func validateClusterIPBroadcastAddresses(ctx context.Context, configClient *scyllaclient.ConfigClient, svc *corev1.Service) {
	broadcastAddress, err := configClient.BroadcastAddress(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(broadcastAddress).To(o.Equal(svc.Spec.ClusterIP), "broadcast_address should match service ClusterIP")

	broadcastRPCAddress, err := configClient.BroadcastRPCAddress(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(broadcastRPCAddress).To(o.Equal(svc.Spec.ClusterIP), "broadcast_rpc_address should match service ClusterIP")

	framework.Infof("Verified broadcast addresses match service ClusterIP: %s", svc.Spec.ClusterIP)
}
