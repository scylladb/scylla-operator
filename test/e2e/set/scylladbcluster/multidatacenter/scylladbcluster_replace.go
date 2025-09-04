// Copyright (C) 2025 ScyllaDB

package multidatacenter

import (
	"maps"
	"slices"
	"sync"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaDBCluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	g.It("should reconcile a node replacement performed directly in a datacenter", func(ctx g.SpecContext) {
		ns, _, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue(), "Can't get default namespace")

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By(`Creating a ScyllaDBCluster`)
		sc := f.GetDefaultScyllaDBCluster(rkcMap)

		sc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = utils.RegisterCollectionOfRemoteScyllaDBClusterNamespaces(ctx, sc, rkcClusterMap)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		rolloutCtx, rolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer rolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(rolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Sanity check.
		o.Expect(sc.Spec.Datacenters).NotTo(o.BeEmpty())
		targetDC := sc.Spec.Datacenters[0]
		targetClusterClient := rkcClusterMap[targetDC.RemoteKubernetesClusterName]
		targetRemoteNamespaceName, err := naming.RemoteNamespaceName(sc, &targetDC)
		o.Expect(err).NotTo(o.HaveOccurred())

		targetSDC, err := targetClusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(targetRemoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &targetDC), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Select a target node to replace.
		targetServiceName := naming.MemberServiceName(targetSDC.Spec.Racks[0], targetSDC, 0)
		targetService, err := targetClusterClient.KubeAdminClient().CoreV1().Services(targetRemoteNamespaceName).Get(ctx, targetServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		targetPod, err := targetClusterClient.KubeAdminClient().CoreV1().Pods(targetRemoteNamespaceName).Get(ctx, targetServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		preReplacementHostID, err := utilsv1alpha1.GetHostID(ctx, targetClusterClient.KubeAdminClient().CoreV1(), targetSDC, targetService, targetPod)
		o.Expect(err).NotTo(o.HaveOccurred(), "Can't get the host ID of the node being replaced")

		framework.By("Initiating the node replacement procedure")
		var scyllaDBClusterProgressingWG sync.WaitGroup
		// Ensure we wait for all background tasks on a test failure.
		defer scyllaDBClusterProgressingWG.Wait()

		// Wait for the controllers to pick up the Pod replacement.
		// Do this in the background to ensure we don't miss the progressing state if it happens too quickly.
		scyllaDBClusterProgressingWG.Add(1)
		go func() {
			defer scyllaDBClusterProgressingWG.Done()
			defer g.GinkgoRecover()

			replacementProgressingCtx, replacementProgressingCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
			defer replacementProgressingCtxCancel()
			_, err = controllerhelpers.WaitForScyllaDBClusterState(replacementProgressingCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{},
				utils.IsScyllaDBClusterProgressing,
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		targetServiceCopy := targetService.DeepCopy()
		metav1.SetMetaDataLabel(&targetServiceCopy.ObjectMeta, naming.ReplaceLabel, "")

		patch, err := controllerhelpers.GenerateMergePatch(targetService, targetServiceCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = targetClusterClient.KubeAdminClient().CoreV1().Services(targetRemoteNamespaceName).Patch(
			ctx,
			targetService.Name,
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for Pod to be replaced")
		podReplacementCtx, podReplacementCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer podReplacementCtxCancel()
		_, err = controllerhelpers.WaitForPodState(podReplacementCtx, targetClusterClient.KubeAdminClient().CoreV1().Pods(targetRemoteNamespaceName), targetPod.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: true}, func(p *corev1.Pod) (bool, error) {
			return p.UID != targetPod.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q to enter Progressing state (RV=%s)", sc.Name, sc.ResourceVersion)
		scyllaDBClusterProgressingWG.Wait()

		framework.By("Waiting for the ScyllaDBCluster %q roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		postReplacementRolloutCtx, postReplacementRolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer postReplacementRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(postReplacementRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying the replacement node has the replace_node_first_boot config parameter set to the host ID of the node being replaced")
		broadcastRPCAddress, err := utilsv1alpha1.GetBroadcastRPCAddress(ctx, targetClusterClient.KubeAdminClient().CoreV1(), targetSDC, targetService)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaConfigClient, err := utilsv1alpha1.GetScyllaConfigClient(ctx, targetClusterClient.KubeAdminClient().CoreV1(), targetSDC, broadcastRPCAddress)
		o.Expect(err).NotTo(o.HaveOccurred())

		replaceNodeFirstBoot, err := scyllaConfigClient.ReplaceNodeFirstBoot(ctx)
		o.Expect(err).NotTo(o.HaveOccurred(), "Can't get replace_node_first_boot config parameter")
		o.Expect(replaceNodeFirstBoot).To(o.Equal(preReplacementHostID), "replace_node_first_boot config parameter doesn't match the pre-replacement host ID")

		framework.By("Verifying the replacement node has a different host ID than the replaced node")
		postReplacementPod, err := targetClusterClient.KubeAdminClient().CoreV1().Pods(targetRemoteNamespaceName).Get(ctx, targetServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		postReplacementHostID, err := utilsv1alpha1.GetHostID(ctx, targetClusterClient.KubeAdminClient().CoreV1(), targetSDC, targetService, postReplacementPod)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(postReplacementHostID).NotTo(o.Equal(preReplacementHostID))

		framework.By("Verifying that the replacement node is part of the cluster and the replaced node is not")
		postReplacementHostIDsByDC, err := utilsv1alpha1.GetHostIDsForScyllaDBCluster(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		allPostReplacementHostIDS := slices.Concat(slices.Collect(maps.Values(postReplacementHostIDsByDC))...)
		o.Expect(allPostReplacementHostIDS).To(o.HaveLen(int(controllerhelpers.GetScyllaDBClusterNodeCount(sc))))
		o.Expect(allPostReplacementHostIDS).NotTo(o.ContainElement(preReplacementHostID))
		o.Expect(allPostReplacementHostIDS).To(o.ContainElement(postReplacementHostID))
	})
})
