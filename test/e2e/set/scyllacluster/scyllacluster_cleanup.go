// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	type horizontalScalingEntry struct {
		initialRackLayout rackLayout
		targetRackLayout  rackLayout
	}
	g.DescribeTable("nodes are cleaned up", framework.SuiteKindClusterTopology, func(ctx g.SpecContext, e *horizontalScalingEntry) {
		jobListWatcher := createJobListWatcher(ctx, f)
		jobObserver := utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err := jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := createClusterAndWaitForRollout(ctx, f, e.initialRackLayout)

		verifyCleanupJobsCreatedEventually(ctx, f, sc, &jobObserver)

		_, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())

		jobObserver = utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err = jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc = scaleClusterAndWaitForRollout(ctx, f, sc, e.targetRackLayout)

		verifyCleanupJobsCreatedEventually(ctx, f, sc, &jobObserver)

		_, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())
	},
		g.Entry("after scaling the cluster out", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 1},
			targetRackLayout:  rackLayout{rackCount: 1, membersPerRack: 3},
		}),
		g.Entry("after scaling the cluster in", &horizontalScalingEntry{
			initialRackLayout: rackLayout{rackCount: 1, membersPerRack: 3},
			targetRackLayout:  rackLayout{rackCount: 1, membersPerRack: 1},
		}),
	)

	g.It("multi-node cluster nodes are cleaned up right after provisioning", func(ctx g.SpecContext) {
		jobListWatcher := createJobListWatcher(ctx, f)
		jobObserver := utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err := jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := createClusterAndWaitForRollout(ctx, f, rackLayout{rackCount: 1, membersPerRack: 3})

		verifyCleanupJobsCreatedEventually(ctx, f, sc, &jobObserver)

		_, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

// createJobListWatcher creates a ListWatch for observing Jobs in the framework's namespace.
func createJobListWatcher(ctx context.Context, f *framework.Framework) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			return f.KubeClient().BatchV1().Jobs(f.Namespace()).List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return f.KubeClient().BatchV1().Jobs(f.Namespace()).Watch(ctx, options)
		},
	}
}

// verifyCleanupJobsCreatedEventually verifies that a cleanup job was created for every node of the cluster.
// It polls the observer's events since cleanup jobs may be created asynchronously after the cluster is marked as rolled
// out - each node service is annotated with the token ring hash independently, which triggers cleanup job creation for
// that node.
func verifyCleanupJobsCreatedEventually(
	ctx context.Context,
	f *framework.Framework,
	sc *scyllav1.ScyllaCluster,
	jobObserver *utils.ObjectObserver[*batchv1.Job],
) {
	tokenRingHash, err := utils.GetCurrentTokenRingHash(ctx, f.KubeClient().CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Infof("Current token ring hash of the cluster is %q", tokenRingHash)

	var memberServiceNames []string
	for _, r := range sc.Spec.Datacenter.Racks {
		for i := range int(r.Members) {
			memberServiceNames = append(memberServiceNames, naming.MemberServiceNameForScyllaCluster(r, sc, i))
		}
	}

	framework.Infof("Verifying cleanup jobs were created for nodes: %v", memberServiceNames)
	o.Eventually(func(eo o.Gomega) {
		var cleanedUpNodes []string
		for _, e := range jobObserver.Events() {
			if e.Action == watch.Added &&
				e.Obj.Labels[naming.NodeJobTypeLabel] == string(naming.JobTypeCleanup) &&
				e.Obj.Annotations[naming.CleanupJobTokenRingHashAnnotation] == tokenRingHash {
				cleanedUpNodes = append(cleanedUpNodes, e.Obj.Labels[naming.NodeJobLabel])
			}
		}

		eo.Expect(cleanedUpNodes).To(o.ConsistOf(memberServiceNames))
	}).WithTimeout(30 * time.Second).WithPolling(1 * time.Second).Should(o.Succeed())
}
