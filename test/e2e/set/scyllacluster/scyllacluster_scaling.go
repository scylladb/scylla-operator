// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"sync/atomic"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

var _ = g.Describe("ScyllaCluster", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should react on scaling up in the middle of scaling down", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating a ScyllaCluster with 3 nodes")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		scyllaClient, _, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		firstNodeIP, err := utils.NodeIndexToIP(ctx, f.KubeClient().CoreV1(), sc, sc.Spec.Datacenter.Racks[0], 0)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scaling the ScyllaCluster to 1 node")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		framework.By("Waiting for the ScyllaCluster to start decommissioning 3rd node (RV=%s)", sc.ResourceVersion)

		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()

		o.Eventually(func(g o.Gomega, ctx context.Context) {
			nss, err := scyllaClient.Status(ctx, firstNodeIP)
			g.Expect(err).NotTo(o.HaveOccurred())

			thirdNodeID, err := utils.NodeIndexToHostID(ctx, f.KubeClient().CoreV1(), sc, sc.Spec.Datacenter.Racks[0], 2)
			g.Expect(err).NotTo(o.HaveOccurred())
			g.Expect(thirdNodeID).NotTo(o.BeEmpty())

			for _, ns := range nss {
				if ns.HostID == thirdNodeID {
					klog.Infof("Node %q status %s%s", thirdNodeID, ns.Status, ns.State)

					g.Expect(ns.Status).To(o.Equal(scyllaclient.NodeStatusUp))
					g.Expect(ns.State).To(o.Equal(scyllaclient.NodeStateLeaving))
				}
			}
		}).WithContext(waitCtx2).WithPolling(time.Second).Should(o.Succeed())

		var secondNodeWasLeaving atomic.Bool
		observerCtx, observerCancel := context.WithCancel(ctx)
		defer observerCancel()
		go func() {
			defer g.GinkgoRecover()

			wait.Until(func() {
				statuses, err := scyllaClient.Status(ctx, firstNodeIP)
				o.Expect(err).NotTo(o.HaveOccurred())

				nodeHostID, err := utils.NodeIndexToHostID(ctx, f.KubeClient().CoreV1(), sc, sc.Spec.Datacenter.Racks[0], 1)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(nodeHostID).NotTo(o.BeEmpty())

				for _, s := range statuses {
					if s.HostID == nodeHostID {
						if s.State == scyllaclient.NodeStateLeaving {
							secondNodeWasLeaving.CompareAndSwap(false, true)
						}
						return
					}
				}
			}, time.Second, observerCtx.Done())
		}()

		framework.By("Scaling up the ScyllaCluster to 4 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 4}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(4))

		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		o.Expect(secondNodeWasLeaving.Load()).To(o.BeFalse())
	})

	g.It("should react on scaling down in the middle of scaling up", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Limiting number of Pods to 5")
		_, err := f.KubeAdminClient().CoreV1().ResourceQuotas(f.Namespace()).Create(
			ctx,
			&corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name: "limit",
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						corev1.ResourcePods: resource.MustParse("5"),
					},
				},
			},
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Scaling the ScyllaCluster to 10 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 10}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(10))

		framework.By("Waiting for the ScyllaCluster to start progressing with 3rd node(RV=%s)", sc.ResourceVersion)

		scyllaClient, _, err := utils.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		firstNodeIP, err := utils.NodeIndexToIP(ctx, f.KubeClient().CoreV1(), sc, sc.Spec.Datacenter.Racks[0], 0)
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, func() *scyllav1.ScyllaCluster {
			// Make a copy of sc with 3 members to properly calculate timeout - ContextForRollout takes number of replicas into account
			scCopy := sc.DeepCopy()
			scCopy.Spec.Datacenter.Racks[0].Members = 3
			return scCopy
		}())
		defer waitCtx2Cancel()
		o.Eventually(func(g o.Gomega, ctx context.Context) {
			nss, err := scyllaClient.Status(ctx, firstNodeIP)
			g.Expect(err).NotTo(o.HaveOccurred())

			thirdNodeID, err := utils.NodeIndexToHostID(ctx, f.KubeClient().CoreV1(), sc, sc.Spec.Datacenter.Racks[0], 2)
			g.Expect(err).NotTo(o.HaveOccurred())
			g.Expect(thirdNodeID).NotTo(o.BeEmpty())

			for _, ns := range nss {
				if ns.HostID == thirdNodeID {
					klog.Infof("Node %q status %s%s", thirdNodeID, ns.Status, ns.State)

					g.Expect(ns.Status).To(o.Equal(scyllaclient.NodeStatusUp))
					g.Expect(ns.State).To(o.Equal(scyllaclient.NodeStateJoining))
				}
			}
		}).WithContext(waitCtx2).WithPolling(time.Second).Should(o.Succeed())

		framework.By("Scaling down the ScyllaCluster to 2 replicas")
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

		framework.By("Awaiting UP/LEAVING status from 3rd node")

		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()

		o.Eventually(func(g o.Gomega, ctx context.Context) {
			nss, err := scyllaClient.Status(ctx, firstNodeIP)
			g.Expect(err).NotTo(o.HaveOccurred())

			thirdNodeID, err := utils.NodeIndexToHostID(ctx, f.KubeClient().CoreV1(), sc, sc.Spec.Datacenter.Racks[0], 2)
			g.Expect(err).NotTo(o.HaveOccurred())
			g.Expect(thirdNodeID).NotTo(o.BeEmpty())

			for _, ns := range nss {
				if ns.HostID == thirdNodeID {
					klog.Infof("Node %q status %s%s", thirdNodeID, ns.Status, ns.State)

					g.Expect(ns.Status).To(o.Equal(scyllaclient.NodeStatusUp))
					g.Expect(ns.State).To(o.Equal(scyllaclient.NodeStateLeaving))
				}
			}
		}).WithContext(waitCtx3).WithPolling(time.Second).Should(o.Succeed())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
	})

	g.It("should support scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(1))
		diRF1 := insertAndVerifyCQLData(ctx, hosts)
		defer diRF1.Close()

		framework.By("Scaling the ScyllaCluster to 3 replicas")
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

		oldHosts := hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(1))
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.ContainElements(oldHosts))

		verifyCQLData(ctx, diRF1)

		// Statistically, some data should land on the 3rd node that will give us a chance to ensure
		// it was stream correctly when downscaling.
		diRF2 := insertAndVerifyCQLData(ctx, hosts[0:2])
		defer diRF2.Close()

		diRF3 := insertAndVerifyCQLData(ctx, hosts)
		defer diRF3.Close()

		framework.By("Scaling the ScyllaCluster down to 2 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 2}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(2))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.HaveLen(2))
		o.Expect(oldHosts).To(o.ContainElements(hosts))

		verifyCQLData(ctx, diRF1)

		// The 2 nodes out of 3 we used earlier may not be the ones that got left. Although discovery will still
		// make sure the missing one is picked up, let's avoid having a down node in the pool and refresh it.
		err = diRF2.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verifyCQLData(ctx, diRF2)

		podName := naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc) + "-1"
		svcName := podName
		framework.By("Marking ScyllaCluster node #2 (%s) for maintenance", podName)
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

		framework.By("Manually draining ScyllaCluster node #2 (%s)", podName)
		ec := &corev1.EphemeralContainer{
			TargetContainerName: naming.ScyllaContainerName,
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:            "e2e-drain-scylla",
				Image:           scyllacluster.ImageForCluster(sc),
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

		framework.By("Scaling the ScyllaCluster down to 1 replicas")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx5, waitCtx5Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx5Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx5, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(2))
		o.Expect(hosts).To(o.HaveLen(1))
		o.Expect(oldHosts).To(o.ContainElements(hosts))

		verifyCQLData(ctx, diRF1)

		framework.By("Scaling the ScyllaCluster back to 3 replicas to make sure there isn't an old (decommissioned) storage in place")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx6, waitCtx6Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx6Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx6, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		oldHosts = hosts
		hosts = getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(oldHosts).To(o.HaveLen(1))
		o.Expect(hosts).To(o.HaveLen(3))
		o.Expect(hosts).To(o.ContainElements(oldHosts))

		verifyCQLData(ctx, diRF1)
		verifyCQLData(ctx, diRF2)
		verifyCQLData(ctx, diRF3)
	})
})
