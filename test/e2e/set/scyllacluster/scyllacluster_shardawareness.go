// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"net"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scylladrivertransport "github.com/scylladb/scylla-go-driver/transport"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.FIt("should allow to build connection pool using shard aware ports", func() {
		const (
			nonShardAwarePort = 9042
			shardAwarePort    = 19042
			nrShards          = 2
		)

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllaclusterfixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		// Ensure 2 shards.
		sc.Spec.Datacenter.Racks[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%d", nrShards))

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		hosts, err := utils.GetHosts(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// connections := make(map[uint16]string)
		// var connectionsMut sync.Mutex

		for i := 0; i < 100; i++ {
			for s := 0; s < nrShards; s++ {
				cfg := scylladrivertransport.DefaultConnConfig("system")
				cfg.DefaultPort = fmt.Sprintf("%d", shardAwarePort)
				c, err := scylladrivertransport.OpenShardConn(ctx, hosts[0], scylladrivertransport.ShardInfo{
					Shard:    uint16(s),
					NrShards: nrShards,
				}, cfg)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(c.Shard()).To(o.Equal(s), fmt.Sprintf("wanted to connect to %d, got %d", s, c.Shard()))
				c.Close()
			}
		}
		//
		// clusterConfig := gocql.NewCluster(hosts...)
		// clusterConfig.Dialer = DialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 	sourcePort := gocql.ScyllaGetSourcePort(ctx)
		// 	localAddr, err := net.ResolveTCPAddr(network, fmt.Sprintf(":%d", sourcePort))
		// 	if err != nil {
		// 		return nil, err
		// 	}
		//
		// 	framework.Infof("Connecting to %s using %d source port", addr, sourcePort)
		// 	connectionsMut.Lock()
		// 	connections[sourcePort] = addr
		// 	connectionsMut.Unlock()
		//
		// 	d := &net.Dialer{LocalAddr: localAddr}
		// 	return d.DialContext(ctx, network, addr)
		// })
		//
		// framework.By("Waiting for the driver to establish connection to shards")
		// session, err := gocqlx.WrapSession(clusterConfig.CreateSession())
		// o.Expect(err).NotTo(o.HaveOccurred())
		// defer session.Close()
		//
		// err = wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (done bool, err error) {
		// 	connectionsMut.Lock()
		// 	defer connectionsMut.Unlock()
		//
		// 	shardAwareAttempts := 0
		// 	for sourcePort := range connections {
		// 		if sourcePort != 0 {
		// 			shardAwareAttempts++
		// 		}
		// 	}
		// 	return shardAwareAttempts == nrShards, nil
		// })
		// o.Expect(err).NotTo(o.HaveOccurred())
		//
		// connectionsMut.Lock()
		// defer connectionsMut.Unlock()
		// for sourcePort, addr := range connections {
		// 	// Control connection is also put in pool, and it always uses the default port.
		// 	if sourcePort == 0 {
		// 		o.Expect(addr).To(o.HaveSuffix(fmt.Sprintf("%d", nonShardAwarePort)))
		// 		continue
		// 	}
		// 	o.Expect(addr).To(o.HaveSuffix(fmt.Sprintf("%d", shardAwarePort)))
		// }
		//
		// //
		// // // Control connection used for shard number discovery, lands on some random shard.
		// // // This connection is also put in pool, and driver only establish connections to missing shards
		// // // using shard-aware-port.
		// // // Connections to shard-aware-port are guaranteed to land on shard driver wants.
		// // o.Expect(shardAwareAttempts).To(o.Equal(nrShards - 1))
	})
})

type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (f DialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}
