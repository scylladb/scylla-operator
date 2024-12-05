// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"context"
	"net"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/tools"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	linuxnetutils "github.com/scylladb/scylla-operator/test/e2e/utils/linux/net"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("listens only on secure ports", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			BroadcastOptions: &scyllav1.NodeBroadcastOptions{
				Nodes: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
				Clients: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
				},
			},
		}
		sc.Spec.Alternator = &scyllav1.AlternatorSpec{}

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		serviceName := naming.MemberServiceName(sc.Spec.Datacenter.Racks[0], sc, 0)
		nodeService, err := f.KubeClient().CoreV1().Services(sc.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		nodeServiceIP := nodeService.Spec.ClusterIP
		o.Expect(nodeServiceIP).NotTo(o.BeEmpty())

		nodePod, err := f.KubeClient().CoreV1().Pods(sc.Namespace).Get(ctx, naming.PodNameFromService(nodeService), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		nodePodIP := nodePod.Status.PodIP
		o.Expect(nodePodIP).NotTo(o.BeEmpty())

		framework.By("Fetching raw network information from /proc/net/tcp{,6}")
		// Because the network stack is shared between containers in the same Pod, it doesn't matter which container we use.
		// We could also create an ephemeral Container in this Pod with an image we need, but the dependency here
		// is minimal (bash + tail) so, as long as one of the images in this Pod has those, using exec is fine and simplifies the test.
		stdout, stderr, err := tools.PodExec(
			f.KubeClient().CoreV1().RESTClient(),
			f.ClientConfig(),
			nodePod.Namespace,
			nodePod.Name,
			"scylla",
			[]string{
				"/usr/bin/bash",
				"-euEo",
				"pipefail",
				"-O",
				"inherit_errexit",
				"-c",
				strings.TrimPrefix(`
tail -n +2 /proc/net/tcp
tail -n +2 /proc/net/tcp6
`, "\n"),
			},
			nil,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stderr).To(o.BeEmpty())
		o.Expect(stdout).NotTo(o.BeEmpty())

		procNetEntries, err := linuxnetutils.ParseProcNetEntries(stdout)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(procNetEntries).NotTo(o.BeEmpty())

		listenProcNetEntries := procNetEntries.FilterListen()
		/*
		 * This is a list of allowed ports to be exposed using default configuration.
		 * All of these ports should use encryption and force AuthN/AuthZ, unless it is not appropriate
		 * or required for the type of data, e.g. /readyz probes.
		 * We have some techdebt in this area so this contains some historical ports that don't uphold
		 * to this standard. All such entries should have an issue link attached.
		 *
		 * Note: Some applications bind dedicated IPv4 and IPv6 socket while some reuse the IPv6 socket for IPv4 as well.
		 */
		o.Expect(listenProcNetEntries).To(o.ConsistOf([]linuxnetutils.AddressPort{
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    22, // ScyllaDB - ssh
				// FIXME: Remove
				//        https://github.com/scylladb/scylla-operator/issues/1763
			},
			{
				Address: net.ParseIP("::"),
				Port:    22, // ScyllaDB - ssh
				// FIXME: Remove
				//        https://github.com/scylladb/scylla-operator/issues/1763
			},
			{
				Address: net.ParseIP("::"),
				Port:    5090, // ScyllaDB Manager agent - metrics (insecure)
			},
			{
				Address: net.ParseIP("127.0.0.1"),
				Port:    5112, // ScyllaDB Manager agent - debug (insecure)
			},
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    7000, // ScyllaDB inter-node (insecure)
				// FIXME: Remove
				//        https://github.com/scylladb/scylla-operator/issues/1217
			},
			// {
			// 	Address: net.ParseIP("127.0.0.1"),
			// 	Port:    7199, // JMX monitoring
			// 	// FIXME: Remove
			// 	// FIXME: https://github.com/scylladb/scylla-operator/issues/1762
			// },
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    8043, // Alternator TLS
			},
			{
				Address: net.ParseIP("::"),
				Port:    8080, // Scylla Operator probes (insecure, non-sensitive data)
			},
			{
				Address: net.ParseIP("127.0.0.1"),
				Port:    9001, // supervisord (planned for removal with cont)
				// FIXME: Remove
				//        https://github.com/scylladb/scylla-operator/issues/1769
			},
			{
				Address: net.ParseIP("::"),
				Port:    9100, // Node exporter - metrics (insecure)
			},
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    9042, // CQL (insecure)
				// FIXME: Remove
				//        https://github.com/scylladb/scylla-operator/issues/1764
			},
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    9142, // CQL TLS (not AuthZ by default)
				// FIXME: Enforce AuthN+AuthZ by default
				//        https://github.com/scylladb/scylla-operator/issues/1770
			},
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    9180, // ScyllaDB - metrics (insecure)
			},
			{
				Address: net.ParseIP("127.0.0.1"),
				Port:    10000, // ScyllaDB API (insecure and unprotected)
			},
			{
				Address: net.ParseIP("::"),
				Port:    10001, // ScyllaDB Manager API (insecure but authorized)
				// FIXME: This needs to be replaced with TLS
				//        https://github.com/scylladb/scylla-operator/issues/1772
			},
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    19042, // Shard-aware CQL (insecure)
				// FIXME: Remove in favour of port 19142
				//        https://github.com/scylladb/scylla-operator/issues/1764
			},
			{
				Address: net.ParseIP("0.0.0.0"),
				Port:    19142, // Shard-aware CQL TLS
				// FIXME: Enforce AuthN+AuthZ by default
				//        https://github.com/scylladb/scylla-operator/issues/1770
			},
		}))
	})
})
