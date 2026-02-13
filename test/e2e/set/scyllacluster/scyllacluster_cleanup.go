// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var _ = g.Describe("ScyllaCluster", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("nodes are cleaned up after horizontal scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		jobListWatcher := createJobListWatcher(ctx, f)
		jobObserver := utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err := jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err := createClusterAndWaitForRollout(ctx, f, 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Validating no cleanup jobs were created")
		jobEvents, err := jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(jobEvents).To(o.BeEmpty())

		jobObserver = utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err = jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = scaleClusterAndWaitForRollout(ctx, f, sc, 3)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCleanupJobsCreatedEventually(ctx, f, sc, &jobObserver, []int32{0, 1})

		_, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())

		jobObserver = utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err = jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = scaleClusterAndWaitForRollout(ctx, f, sc, 2)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCleanupJobsCreatedEventually(ctx, f, sc, &jobObserver, []int32{0, 1})

		_, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("multi-node cluster nodes are cleaned up right after provisioning", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		jobListWatcher := createJobListWatcher(ctx, f)
		jobObserver := utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err := jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err := createClusterAndWaitForRollout(ctx, f, 3)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyCleanupJobsCreatedEventually(ctx, f, sc, &jobObserver, []int32{0, 1})

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

// nodeJobMatcher returns a matcher function that checks if a job event is for the given node name.
func nodeJobMatcher(nodeName string) func(utils.ObserverEvent[*batchv1.Job]) bool {
	return func(e utils.ObserverEvent[*batchv1.Job]) bool {
		return e.Obj.Labels[naming.NodeJobLabel] == nodeName
	}
}

// verifyCleanupJobsCreatedEventually verifies that cleanup jobs were created for the expected nodes.
// It polls the observer's events since cleanup jobs may be created asynchronously after the cluster
// is marked as rolled out - each node service is annotated with the token ring hash independently,
// which triggers cleanup job creation for that node.
func verifyCleanupJobsCreatedEventually(
	ctx context.Context,
	f *framework.Framework,
	sc *scyllav1.ScyllaCluster,
	jobObserver *utils.ObjectObserver[*batchv1.Job],
	expectedNodeIndices []int32,
) {
	tokenRingHash, err := utils.GetCurrentTokenRingHash(ctx, f.KubeClient().CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Infof("Current token ring hash of the cluster is %q", tokenRingHash)

	framework.Infof("Verifying cleanup jobs were created for nodes: %v", expectedNodeIndices)

	expectedMatchers := make([]interface{}, len(expectedNodeIndices))
	for i, nodeIndex := range expectedNodeIndices {
		nodeName := naming.MemberServiceNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, int(nodeIndex))
		expectedMatchers[i] = o.Satisfy(nodeJobMatcher(nodeName))
	}

	o.Eventually(func(g o.Gomega) {
		jobEvents := jobObserver.Events()
		cleanupJobsCreated := oslices.Filter(jobEvents, func(e utils.ObserverEvent[*batchv1.Job]) bool {
			return e.Action == watch.Added &&
				e.Obj.Labels[naming.NodeJobTypeLabel] == string(naming.JobTypeCleanup) &&
				e.Obj.Annotations[naming.CleanupJobTokenRingHashAnnotation] == tokenRingHash
		})

		g.Expect(cleanupJobsCreated).To(o.HaveLen(len(expectedNodeIndices)),
			"expected %d cleanup jobs with token ring hash %q, got %d",
			len(expectedNodeIndices), tokenRingHash, len(cleanupJobsCreated))
		g.Expect(cleanupJobsCreated).To(o.ConsistOf(expectedMatchers...))
	}).WithTimeout(30 * time.Second).WithPolling(1 * time.Second).Should(o.Succeed())
}

// createClusterAndWaitForRollout creates a ScyllaCluster with the specified number of members and waits for rollout.
func createClusterAndWaitForRollout(
	ctx context.Context,
	f *framework.Framework,
	members int32,
) (*scyllav1.ScyllaCluster, error) {
	sc := f.GetDefaultScyllaCluster()
	sc.Spec.Datacenter.Racks[0].Members = members

	framework.By("Creating a %d node ScyllaCluster", members)
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

	return sc, nil
}

// scaleClusterAndWaitForRollout scales the cluster to the given number of members and waits for rollout.
func scaleClusterAndWaitForRollout(
	ctx context.Context,
	f *framework.Framework,
	sc *scyllav1.ScyllaCluster,
	members int32,
) (*scyllav1.ScyllaCluster, error) {
	patchData := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`, members))

	framework.By("Scaling the ScyllaCluster to %d members", members)
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
		ctx,
		sc.Name,
		types.JSONPatchType,
		patchData,
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
	o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(members))

	framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

	return sc, nil
}
