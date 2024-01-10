// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/gocqlx/v2"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should allow to build connection pool using shard aware ports", func() {
		const (
			nonShardAwarePort = 9042
			shardAwarePort    = 19042
			nrShards          = 4
		)

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			BroadcastOptions: &scyllav1.NodeBroadcastOptions{
				Clients: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
				Nodes: scyllav1.BroadcastOptions{
					Type: scyllav1.BroadcastAddressTypePodIP,
				},
			},
		}

		// Ensure number of shards.
		sc.Spec.Datacenter.Racks[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%d", nrShards))

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))

		connections := make(map[uint16]string)
		var connectionsMut sync.Mutex

		clusterConfig := gocql.NewCluster(hosts...)
		clusterConfig.Dialer = DialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
			sourcePort := gocql.ScyllaGetSourcePort(ctx)
			localAddr, err := net.ResolveTCPAddr(network, fmt.Sprintf(":%d", sourcePort))
			if err != nil {
				return nil, err
			}

			framework.Infof("Connecting to %s using %d source port", addr, sourcePort)
			connectionsMut.Lock()
			connections[sourcePort] = addr
			connectionsMut.Unlock()

			d := &net.Dialer{LocalAddr: localAddr}
			return d.DialContext(ctx, network, addr)
		})

		framework.By("Waiting for the driver to establish connection to shards")
		session, err := gocqlx.WrapSession(clusterConfig.CreateSession())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer session.Close()

		err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 5*time.Second, true, func(context.Context) (done bool, err error) {
			return len(connections) == nrShards, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		shardAwareAttempts := 0
		for sourcePort, addr := range connections {
			// Control connection is also put in pool, and it always uses the default port.
			if sourcePort == 0 {
				o.Expect(addr).To(o.HaveSuffix(fmt.Sprintf("%d", nonShardAwarePort)))
				continue
			}
			o.Expect(addr).To(o.HaveSuffix(fmt.Sprintf("%d", shardAwarePort)))
			shardAwareAttempts++
		}

		// Control connection used for shard number discovery, lands on some random shard.
		// This connection is also put in pool, and driver only establish connections to missing shards
		// using shard-aware-port.
		// Connections to shard-aware-port are guaranteed to land on shard driver wants.
		o.Expect(shardAwareAttempts).To(o.Equal(nrShards - 1))
	})
})

type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (f DialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}
