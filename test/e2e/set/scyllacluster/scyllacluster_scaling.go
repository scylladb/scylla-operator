// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controller/scylladbdatacenter"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should support scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating a ScyllaCluster with 3 members")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hostIDs).To(o.HaveLen(3))
		diRF3 := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer diRF3.Close()

		framework.By("Scaling the ScyllaCluster to 5 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 5}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(5))

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHosts := hosts
		oldHostIDs := hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHosts).To(o.HaveLen(3))
		o.Expect(oldHostIDs).To(o.HaveLen(3))
		o.Expect(hosts).To(o.HaveLen(5))
		o.Expect(hostIDs).To(o.HaveLen(5))
		o.Expect(hostIDs).To(o.ContainElements(oldHostIDs))

		verification.VerifyCQLData(ctx, diRF3)

		podName := naming.StatefulSetNameForRackForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc) + "-4"
		svcName := podName
		framework.By("Marking ScyllaCluster node #4 (%s) for maintenance", podName)
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					naming.NodeMaintenanceLabel: "",
				},
			},
		}
		patch, err := helpers.CreateTwoWayMergePatch(&corev1.Service{}, svc)
		o.Expect(err).NotTo(o.HaveOccurred())
		svc, err = f.KubeClient().CoreV1().Services(sc.Namespace).Patch(
			ctx,
			svcName,
			types.StrategicMergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Manually draining ScyllaCluster node #4 (%s)", podName)
		ec := &corev1.EphemeralContainer{
			TargetContainerName: naming.ScyllaContainerName,
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:            "e2e-drain-scylla",
				Image:           scylladbdatacenter.ImageForCluster(sc),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/usr/bin/nodetool", "drain"},
				Args:            []string{},
			},
		}
		pod, err := utils.RunEphemeralContainerAndWaitForCompletion(ctx, f.KubeClient().CoreV1().Pods(sc.Namespace), podName, ec)
		o.Expect(err).NotTo(o.HaveOccurred())
		ephemeralContainerState := controllerhelpers.FindContainerStatus(pod, ec.Name)
		o.Expect(ephemeralContainerState).NotTo(o.BeNil())
		o.Expect(ephemeralContainerState.State.Terminated).NotTo(o.BeNil())
		o.Expect(ephemeralContainerState.State.Terminated.ExitCode).To(o.BeEquivalentTo(0))

		framework.By("Scaling the ScyllaCluster down to 4 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 4}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(4))

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHosts = hosts
		oldHostIDs = hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHosts).To(o.HaveLen(5))
		o.Expect(oldHostIDs).To(o.HaveLen(5))
		o.Expect(hosts).To(o.HaveLen(4))
		o.Expect(hostIDs).To(o.HaveLen(4))
		o.Expect(oldHostIDs).To(o.ContainElements(hostIDs))

		verification.VerifyCQLData(ctx, diRF3)

		framework.By("Scaling the ScyllaCluster down to 3 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx5, waitCtx5Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx5Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHosts = hosts
		oldHostIDs = hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHosts).To(o.HaveLen(4))
		o.Expect(oldHostIDs).To(o.HaveLen(4))
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hostIDs).To(o.HaveLen(3))
		o.Expect(oldHostIDs).To(o.ContainElements(hostIDs))

		verification.VerifyCQLData(ctx, diRF3)

		framework.By("Scaling the ScyllaCluster back to 5 replicas to make sure there isn't an old (decommissioned) storage in place")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 5}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(5))

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx6, waitCtx6Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx6Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx6, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHosts = hosts
		oldHostIDs = hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHosts).To(o.HaveLen(3))
		o.Expect(oldHostIDs).To(o.HaveLen(3))
		o.Expect(hosts).To(o.HaveLen(5))
		o.Expect(hostIDs).To(o.HaveLen(5))
		o.Expect(hostIDs).To(o.ContainElements(oldHostIDs))

		verification.VerifyCQLData(ctx, diRF3)
	})
})

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should not lose more than one pod at time after smp change", func() {
		testCtx, testCtxCancel := context.WithTimeout(context.Background(), testTimeout)
		defer testCtxCancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating a ScyllaCluster with 3 members")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(testCtx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(testCtx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(testCtx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(testCtx, f.KubeClient().CoreV1(), sc)

		framework.By("Setting up Pod observer to monitor rolling restart")
		podSelector := labels.SelectorFromSet(naming.ScyllaDBNodePodsSelectorLabelsForScyllaCluster(sc))
		lw := &cache.ListWatch{
			ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = podSelector.String()
				return f.KubeClient().CoreV1().Pods(f.Namespace()).List(testCtx, options)
			}),
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = podSelector.String()
				return f.KubeClient().CoreV1().Pods(f.Namespace()).Watch(testCtx, options)
			},
		}
		podObserver := utils.ObserveObjects[*corev1.Pod](lw)
		err = podObserver.Start(testCtx)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Patching CPU limit from 1 to 2 to trigger SMP change and resharding")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			testCtx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources/limits/cpu", "value": "2"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		reshardingRolloutCtx, reshardingRolloutCtxCancel := utils.ContextForRollout(testCtx, sc)
		defer reshardingRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(reshardingRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Stopping Pod observer and verifying rolling restart constraint")
		events, err := podObserver.Stop()
		o.Expect(err).NotTo(o.HaveOccurred())

		podReady := map[string]bool{}
		for _, event := range events {
			pod := event.Obj
			switch event.Action {
			case watch.Deleted:
				podReady[pod.Name] = false
			default:
				podReady[pod.Name] = controllerhelpers.IsPodReady(pod)
			}

			notReadyCount := 0
			for _, ready := range podReady {
				if !ready {
					notReadyCount++
				}
			}
			o.Expect(notReadyCount).To(o.BeNumerically("<=", 1),
				fmt.Sprintf("more than one pod was not-ready simultaneously during rollout, pod states: %v", podReady),
			)
		}
	})
})
