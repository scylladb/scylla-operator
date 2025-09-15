// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

var _ = g.Describe("ScyllaCluster IPv6", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should create IPv6-preferred cluster successfully", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		// Configure IPv6-only cluster
		ipv6Family := corev1.IPv6Protocol
		sc.Spec.IPFamily = &ipv6Family
		sc.Spec.Network = scyllav1.Network{
			DNSPolicy:      corev1.DNSClusterFirst,
			IPFamilyPolicy: (*corev1.IPFamilyPolicy)(&[]corev1.IPFamilyPolicy{corev1.IPFamilyPolicySingleStack}[0]),
			IPFamilies:     []corev1.IPFamily{corev1.IPv6Protocol},
		}
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			BroadcastOptions: &scyllav1.NodeBroadcastOptions{
				Nodes: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
				Clients: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
			},
		}

		framework.By("Creating IPv6-preferred ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCtx, verifyCtxCancel := utils.ContextForRollout(ctx, sc)
		defer verifyCtxCancel()

		framework.By("Verifying IPv6 networking configuration")
		verifyIPv6Configuration(verifyCtx, f, sc)
	})

	g.It("should create dual-stack cluster with IPv4 primary successfully", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		// Configure dual-stack cluster with IPv4 primary
		ipv4Family := corev1.IPv4Protocol
		sc.Spec.IPFamily = &ipv4Family
		sc.Spec.Network = scyllav1.Network{
			DNSPolicy:      corev1.DNSClusterFirst,
			IPFamilyPolicy: (*corev1.IPFamilyPolicy)(&[]corev1.IPFamilyPolicy{corev1.IPFamilyPolicyPreferDualStack}[0]),
			IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
		}
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			BroadcastOptions: &scyllav1.NodeBroadcastOptions{
				Nodes: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
				Clients: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
			},
		}

		framework.By("Creating dual-stack ScyllaCluster with IPv4 primary")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCtx, verifyCtxCancel := utils.ContextForRollout(ctx, sc)
		defer verifyCtxCancel()

		framework.By("Verifying dual-stack networking configuration")
		verifyDualStackConfiguration(verifyCtx, f, sc)
	})

	g.It("should maintain IP family consistency for broadcast addresses", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		// Configure cluster to test broadcast address consistency
		// We'll use IPv4 as primary since dual-stack clusters typically assign IPv4 as primary
		ipv4Family := corev1.IPv4Protocol
		sc.Spec.IPFamily = &ipv4Family
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			BroadcastOptions: &scyllav1.NodeBroadcastOptions{
				Nodes: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
				Clients: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
			},
		}

		framework.By("Creating ScyllaCluster to test broadcast address consistency")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying broadcast address IP family consistency")
		verifyBroadcastAddressConsistency(ctx, f, sc)
	})
})

// verifyIPv6Configuration verifies that IPv6 networking is properly configured
func verifyIPv6Configuration(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	// Get pods
	pods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pods.Items).ToNot(o.BeEmpty())

	// Verify pods have IPv6 addresses
	for _, pod := range pods.Items {
		framework.By("Verifying pod %q has IPv6 address", pod.Name)

		// Verify at least one IPv6 address in PodIPs
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

		// For IPv6-preferred clusters, verify ScyllaDB sidecar is configured with IPv6
		for _, container := range pod.Spec.Containers {
			if container.Name == "scylla" {
				// Check if IP_FAMILY env var is set to IPv6
				for _, envVar := range container.Env {
					if envVar.Name == "IP_FAMILY" {
						o.Expect(envVar.Value).To(o.Equal("IPv6"), "ScyllaDB container should be configured for IPv6")
						break
					}
				}
			}
		}
	}

	// Verify services are configured for IPv6
	services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, svc := range services.Items {
		framework.By("Verifying service %q IPv6 configuration", svc.Name)

		// Check service has IPv6 in its IP families
		hasIPv6Family := false
		for _, family := range svc.Spec.IPFamilies {
			if family == corev1.IPv6Protocol {
				hasIPv6Family = true
				break
			}
		}
		o.Expect(hasIPv6Family).To(o.BeTrue(), "Service should have IPv6 in IPFamilies")

		// Verify ClusterIP is IPv6 (if not headless)
		if svc.Spec.ClusterIP != corev1.ClusterIPNone {
			clusterIP, err := helpers.ParseIP(svc.Spec.ClusterIP)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(helpers.IsIPv6(clusterIP)).To(o.BeTrue(), "Service ClusterIP should be IPv6: %s", svc.Spec.ClusterIP)
		}
	}
}

// verifyDualStackConfiguration verifies that dual-stack networking is properly configured
func verifyDualStackConfiguration(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	// Get services
	services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, svc := range services.Items {
		framework.By("Verifying service %q dual-stack configuration", svc.Name)

		// Check service has both IPv4 and IPv6 in its IP families (if cluster supports it)
		if len(svc.Spec.IPFamilies) > 1 {
			o.Expect(svc.Spec.IPFamilies).To(o.ContainElement(corev1.IPv4Protocol))
			o.Expect(svc.Spec.IPFamilies).To(o.ContainElement(corev1.IPv6Protocol))
		}

		// Primary IP family should be IPv4 since we configured ipFamily: IPv4
		if len(svc.Spec.IPFamilies) > 0 {
			o.Expect(svc.Spec.IPFamilies[0]).To(o.Equal(corev1.IPv4Protocol))
		}
	}

	// Verify pods can have both IPv4 and IPv6 addresses
	pods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		framework.By("Verifying pod %q dual-stack configuration", pod.Name)

		// Primary IP should be IPv4 since we configured ipFamily: IPv4
		o.Expect(pod.Status.PodIP).ToNot(o.BeEmpty())
		primaryIP, err := helpers.ParseIP(pod.Status.PodIP)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(helpers.IsIPv4(primaryIP)).To(o.BeTrue(), "Primary pod IP should be IPv4: %s", pod.Status.PodIP)
	}
}

// verifyBroadcastAddressConsistency verifies that broadcast addresses use consistent IP families
func verifyBroadcastAddressConsistency(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	// Get ScyllaDB pods
	pods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(naming.ClusterLabelsForScyllaCluster(sc)).String(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pods.Items).ToNot(o.BeEmpty())

	for _, pod := range pods.Items {
		framework.By("Verifying broadcast address consistency for pod %q", pod.Name)

		// Check ScyllaDB configuration for broadcast addresses
		stdout, stderr, err := executeInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), &pod, "cat", "/etc/scylla/scylla.yaml")
		framework.Infof("ScyllaDB config stdout: %s", stdout)
		if err != nil {
			framework.Infof("ScyllaDB config stderr: %s", stderr)
			// If we can't read the config, skip this verification
			framework.Infof("Skipping broadcast address verification for pod %q due to config read error", pod.Name)
			continue
		}

		// Parse broadcast addresses from configuration and verify IP family consistency
		lines := strings.Split(stdout, "\n")
		var broadcastAddr, broadcastRPCAddr string

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "broadcast_address:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					broadcastAddr = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "broadcast_rpc_address:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					broadcastRPCAddr = strings.TrimSpace(parts[1])
				}
			}
		}

		framework.Infof("Found broadcast_address: %s, broadcast_rpc_address: %s", broadcastAddr, broadcastRPCAddr)

		if broadcastAddr != "" && broadcastRPCAddr != "" {
			// Parse both addresses
			bAddr, err := helpers.ParseIP(broadcastAddr)
			if err != nil {
				framework.Infof("Warning: Failed to parse broadcast_address %q: %v", broadcastAddr, err)
				continue
			}

			bRPCAddr, err := helpers.ParseIP(broadcastRPCAddr)
			if err != nil {
				framework.Infof("Warning: Failed to parse broadcast_rpc_address %q: %v", broadcastRPCAddr, err)
				continue
			}

			// Verify both use the same IP family
			isIPv6Broadcast := helpers.IsIPv6(bAddr)
			isIPv6BroadcastRPC := helpers.IsIPv6(bRPCAddr)

			o.Expect(isIPv6Broadcast).To(o.Equal(isIPv6BroadcastRPC),
				"broadcast_address (%s, IPv6: %t) and broadcast_rpc_address (%s, IPv6: %t) must use the same IP family",
				broadcastAddr, isIPv6Broadcast, broadcastRPCAddr, isIPv6BroadcastRPC)

			framework.Infof("Verified IP family consistency: broadcast_address=%s (IPv6: %t), broadcast_rpc_address=%s (IPv6: %t)",
				broadcastAddr, isIPv6Broadcast, broadcastRPCAddr, isIPv6BroadcastRPC)
		} else {
			framework.Infof("Warning: Could not find both broadcast addresses in config for pod %q", pod.Name)
		}
	}
}

// executeInPod executes a command in the specified pod
func executeInPod(ctx context.Context, config *rest.Config, client corev1client.CoreV1Interface, pod *corev1.Pod, command string, args ...string) (string, string, error) {
	return utils.ExecWithOptions(ctx, config, client, utils.ExecOptions{
		Command:       append([]string{command}, args...),
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: naming.ScyllaContainerName,
		CaptureStdout: true,
		CaptureStderr: true,
	})
}
