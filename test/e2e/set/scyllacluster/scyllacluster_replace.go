// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"sync"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should replace a node", func(ctx g.SpecContext) {
		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue(), "Can't get default namespace")

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 3

		framework.By("Creating a ScyllaCluster")
		sc, err := nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(ns.Name).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer rolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(rolloutCtx, nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sc)

		hosts, err := utils.GetBroadcastRPCAddresses(ctx, nsClient.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(int(utils.GetMemberCount(sc))))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		// Select a target node to replace.
		targetServiceName := utils.GetNodeName(sc, 0)
		targetService, err := nsClient.KubeClient().CoreV1().Services(sc.Namespace).Get(ctx, targetServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		targetPod, err := nsClient.KubeClient().CoreV1().Pods(sc.Namespace).Get(ctx, targetServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		preReplacementHostID, err := utils.GetHostID(ctx, nsClient.KubeClient().CoreV1(), sc, targetService, targetPod)
		o.Expect(err).NotTo(o.HaveOccurred(), "Can't get the host ID of the node being replaced")

		framework.By("Initiating the node replacement procedure")
		var scyllaClusterProgressingWG sync.WaitGroup
		// Ensure we wait for all background tasks on a test failure.
		defer scyllaClusterProgressingWG.Wait()

		// Wait for the controllers to pick up the Pod replacement.
		// Do this in the background to ensure we don't miss the progressing state if it happens too quickly.
		scyllaClusterProgressingWG.Add(1)
		go func() {
			defer scyllaClusterProgressingWG.Done()
			defer g.GinkgoRecover()

			replacementProgressingCtx, replacementProgressingCtxCancel := utils.ContextForRollout(ctx, sc)
			defer replacementProgressingCtxCancel()
			_, err = controllerhelpers.WaitForScyllaClusterState(replacementProgressingCtx, nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{},
				utils.IsScyllaClusterProgressing,
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		targetServiceCopy := targetService.DeepCopy()
		metav1.SetMetaDataLabel(&targetServiceCopy.ObjectMeta, naming.ReplaceLabel, "")

		patch, err := controllerhelpers.GenerateMergePatch(targetService, targetServiceCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = nsClient.KubeClient().CoreV1().Services(sc.Namespace).Patch(
			ctx,
			targetService.Name,
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for Pod to be replaced")
		podReplacementCtx, podReplacementCtxCancel := utils.ContextForRollout(ctx, sc)
		defer podReplacementCtxCancel()
		_, err = controllerhelpers.WaitForPodState(podReplacementCtx, nsClient.KubeClient().CoreV1().Pods(sc.Namespace), targetPod.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(p *corev1.Pod) (bool, error) {
			return p.UID != targetPod.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster %q to enter Progressing state (RV=%s)", sc.Name, sc.ResourceVersion)
		scyllaClusterProgressingWG.Wait()

		framework.By("Waiting for the ScyllaCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		postReplacementRolloutCtx, postReplacementRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer postReplacementRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(postReplacementRolloutCtx, nsClient.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sc)

		framework.By("Verifying the replacement node has the replace_node_first_boot config parameter set to the host ID of the node being replaced")
		broadcastRPCAddress, err := utils.GetBroadcastRPCAddress(ctx, nsClient.KubeClient().CoreV1(), sc, targetService)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaConfigClient, err := utils.GetScyllaConfigClient(ctx, nsClient.KubeClient().CoreV1(), sc, broadcastRPCAddress)
		o.Expect(err).NotTo(o.HaveOccurred())

		replaceNodeFirstBoot, err := scyllaConfigClient.ReplaceNodeFirstBoot(ctx)
		o.Expect(err).NotTo(o.HaveOccurred(), "Can't get replace_node_first_boot config parameter")
		o.Expect(replaceNodeFirstBoot).To(o.Equal(preReplacementHostID), "replace_node_first_boot config parameter doesn't match the pre-replacement host ID")

		framework.By("Verifying the replacement node has a different host ID than the replaced node")
		postReplacementPod, err := nsClient.KubeClient().CoreV1().Pods(sc.Namespace).Get(ctx, targetServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		postReplacementHostID, err := utils.GetHostID(ctx, nsClient.KubeClient().CoreV1(), sc, targetService, postReplacementPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(postReplacementHostID).NotTo(o.Equal(preReplacementHostID))

		framework.By("Verifying that the replacement node is part of the cluster and the replaced node is not")
		_, postReplacementHostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, nsClient.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(postReplacementHostIDs).To(o.HaveLen(int(utils.GetMemberCount(sc))))
		o.Expect(postReplacementHostIDs).NotTo(o.ContainElement(preReplacementHostID))
		o.Expect(postReplacementHostIDs).To(o.ContainElement(postReplacementHostID))
	})
})
