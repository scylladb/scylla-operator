// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/util/cql"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaCluster", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should allow to build connection pool using shard aware ports", func() {
		const (
			nrShards = 4
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

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		waitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))

		clientPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "client",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "client",
						Image: configassests.Project.OperatorTests.NodeSetupImage,
						Command: []string{
							"sleep",
							"infinity",
						},
					},
				},
				TerminationGracePeriodSeconds: pointer.Int64(1),
				RestartPolicy:                 corev1.RestartPolicyNever,
			},
		}
		clientPod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})

		waitCtx2, waitCtx2Cancel := utils.ContextForPodStartup(ctx)
		defer waitCtx2Cancel()
		clientPod, err = controllerhelpers.WaitForPodState(waitCtx2, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)

		const (
			scyllaShardKey = "SCYLLA_SHARD"
			shardAwarePort = 19042

			connectionAttempts = 10
		)

		for shard := range nrShards {
			port := shardPort(shard, nrShards)

			for i := range connectionAttempts {
				framework.By("Establishing connection number %d to shard number %d", i, shard)
				stdout, stderr, err := utils.ExecWithOptions(f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
					Command: []string{
						"/usr/bin/bash",
						"-euEo",
						"pipefail",
						"-O",
						"inherit_errexit",
						"-c",
						fmt.Sprintf(`echo -e '%s' | nc -p %d %s %d`, cql.OptionsFrame, port, hosts[0], shardAwarePort)},
					Namespace:     clientPod.Namespace,
					PodName:       clientPod.Name,
					ContainerName: clientPod.Name,
					CaptureStdout: true,
					CaptureStderr: true,
				})
				o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
				o.Expect(stderr).To(o.BeEmpty())
				o.Expect(stdout).ToNot(o.BeEmpty())

				fp := cql.NewFrameParser(bytes.NewBuffer([]byte(stdout)))
				fp.SkipHeader()

				scyllaSupported := fp.ReadStringMultiMap()
				o.Expect(scyllaSupported[scyllaShardKey]).To(o.HaveLen(1))
				o.Expect(scyllaSupported[scyllaShardKey][0]).To(o.Equal(fmt.Sprintf("%d", shard)))
			}
		}
	})
})

// Ref: https://github.com/scylladb/scylla-rust-driver/blob/de7d8a5c78ea0702bf6da80197f7c495a145c188/scylla/src/routing.rs#L104-L110
func shardPort(shard, nrShards int) int {
	const (
		maxPort = 65535
		minPort = 49152
	)
	maxRange := maxPort - nrShards + 1
	minRange := minPort + nrShards - 1
	r := rand.Intn(maxRange-minRange+1) + minRange
	return r/nrShards*nrShards + shard
}
