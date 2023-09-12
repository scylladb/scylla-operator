// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("nodes are cleaned up after horizontal scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		jobListWatcher := &cache.ListWatch{
			ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
				return f.KubeClient().BatchV1().Jobs(f.Namespace()).List(ctx, options)
			}),
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return f.KubeClient().BatchV1().Jobs(f.Namespace()).Watch(ctx, options)
			},
		}

		framework.By("Creating a single node ScyllaCluster")

		jobObserver := utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err := jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Validating no cleanup jobs were created")
		jobEvents, err := jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(jobEvents).To(o.BeEmpty())

		framework.By("Scaling the ScyllaCluster to 3 replicas")

		jobObserver = utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err = jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Validating cleanup jobs were created for all nodes except last one")
		jobEvents, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(jobEvents).NotTo(o.BeEmpty())

		nodeJobMatcher := func(nodeName string) func(utils.ObserverEvent[*batchv1.Job]) bool {
			return func(e utils.ObserverEvent[*batchv1.Job]) bool {
				return e.Obj.Labels[naming.NodeJobLabel] == nodeName
			}
		}

		cleanupJobsCreated := slices.Filter(jobEvents, func(e utils.ObserverEvent[*batchv1.Job]) bool {
			return e.Action == watch.Added && e.Obj.Labels[naming.NodeJobTypeLabel] == string(naming.JobTypeCleanup)
		})

		o.Expect(cleanupJobsCreated).To(o.HaveLen(2))
		o.Expect(cleanupJobsCreated).To(o.ConsistOf(
			o.Satisfy(nodeJobMatcher(naming.MemberServiceName(sc.Spec.Datacenter.Racks[0], sc, 0))),
			o.Satisfy(nodeJobMatcher(naming.MemberServiceName(sc.Spec.Datacenter.Racks[0], sc, 1))),
		))

		framework.By("Scaling down the ScyllaCluster to 2 replicas")

		jobObserver = utils.ObserveObjects[*batchv1.Job](jobListWatcher)
		err = jobObserver.Start(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 2}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(2))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Validating cleanup jobs were created for all nodes")
		jobEvents, err = jobObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(jobEvents).NotTo(o.BeEmpty())

		cleanupJobsCreated = slices.Filter(jobEvents, func(e utils.ObserverEvent[*batchv1.Job]) bool {
			return e.Action == watch.Added && e.Obj.Labels[naming.NodeJobTypeLabel] == string(naming.JobTypeCleanup)
		})

		o.Expect(cleanupJobsCreated).To(o.HaveLen(2))
		o.Expect(cleanupJobsCreated).To(o.ConsistOf(
			o.Satisfy(nodeJobMatcher(naming.MemberServiceName(sc.Spec.Datacenter.Racks[0], sc, 0))),
			o.Satisfy(nodeJobMatcher(naming.MemberServiceName(sc.Spec.Datacenter.Racks[0], sc, 1))),
		))
	})
})
