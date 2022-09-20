// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/image"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should allow to build connection pool using shard aware ports", func() {
		const (
			shardAwarePort = 19042
			nrShards       = 2
		)

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllaclusterfixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		// Ensure number of shards.
		sc.Spec.Datacenter.Racks[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%d", nrShards))

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		hosts, err := utils.GetHosts(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))

		clientPod := makePodSpec("client", image.GetE2EImage(image.BusyBox))
		clientPod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx2, waitCtx2Cancel := utils.ContextForPodStartup(ctx)
		defer waitCtx2Cancel()
		clientPod, err = utils.WaitForPodState(waitCtx2, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, utils.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		const (
			optionsFrame   = `\x04\x00\x00\x00\x05\x00\x00\x00\x00`
			scyllaShardKey = "SCYLLA_SHARD"
		)

		for i := 0; i < 10; i++ {
			for shard := 0; shard < nrShards; shard++ {
				framework.By("Establishing connection to %d shard", shard)

				port := shardPort(shard, nrShards)
				stdout, stderr, err := f.ExecWithOptions(framework.ExecOptions{
					Command:       []string{"/bin/sh", "-c", fmt.Sprintf(`echo -e '%s' | nc -p %d %s %d`, optionsFrame, port, hosts[0], shardAwarePort)},
					Namespace:     clientPod.Namespace,
					PodName:       clientPod.Name,
					ContainerName: clientPod.Name,
					CaptureStdout: true,
					CaptureStderr: true,
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(stderr).To(o.BeEmpty())
				o.Expect(stdout).ToNot(o.BeEmpty())

				fp := &frameParser{buf: bytes.NewBuffer([]byte(stdout))}
				fp.SkipHeader()

				scyllaSupported := fp.ReadStringMultiMap()
				o.Expect(scyllaSupported[scyllaShardKey]).To(o.HaveLen(1))
				o.Expect(scyllaSupported[scyllaShardKey][0]).To(o.Equal(fmt.Sprintf("%d", shard)))
			}
		}
	})
})

func makePodSpec(name, image string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  name,
					Image: image,
					Command: []string{
						"/bin/sh",
						"-c",
						"sleep 3600",
					},
				},
			},
			TerminationGracePeriodSeconds: pointer.Int64(1),
			RestartPolicy:                 corev1.RestartPolicyNever,
		},
	}

	return pod
}

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

type frameParser struct {
	buf *bytes.Buffer
}

func (fp *frameParser) SkipHeader() {
	const (
		headerLen = 9
	)
	_ = fp.readBytes(headerLen)
}

func (fp *frameParser) readByte() byte {
	p, err := fp.buf.ReadByte()
	if err != nil {
		panic(fmt.Errorf("buffer readByte error: %w", err))
	}
	return p
}

func (fp *frameParser) ReadShort() uint16 {
	return uint16(fp.readByte())<<8 | uint16(fp.readByte())
}

func (fp *frameParser) ReadStringMultiMap() map[string][]string {
	n := fp.ReadShort()
	m := make(map[string][]string, n)
	for i := uint16(0); i < n; i++ {
		k := fp.ReadString()
		v := fp.ReadStringList()
		m[k] = v
	}
	return m
}

func (fp *frameParser) readBytes(n int) []byte {
	p := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		p = append(p, fp.readByte())
	}

	return p
}

func (fp *frameParser) ReadString() string {
	return string(fp.readBytes(int(fp.ReadShort())))
}

func (fp *frameParser) ReadStringList() []string {
	n := fp.ReadShort()
	l := make([]string, 0, n)
	for i := uint16(0); i < n; i++ {
		l = append(l, fp.ReadString())
	}
	return l
}
