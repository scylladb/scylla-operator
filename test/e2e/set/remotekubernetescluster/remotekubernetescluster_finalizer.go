// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("RemoteKubernetesCluster finalizer", func() {
	f := framework.NewFramework("remotekubernetescluster")

	g.It("should block deletion when there are ScyllaDBClusters referencing the object", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		cluster := f.Cluster()
		userNs, _ := f.CreateUserNamespace(ctx)

		rkcClusterKey := "dc-1"
		rkcName := cluster.Name()

		framework.By("Creating RemoteKubernetesCluster %q with credentials to cluster %q", rkcName, cluster.Name())
		originalRKC, err := utils.GetRemoteKubernetesClusterWithOperatorClusterRole(ctx, f.KubeAdminClient(), f.AdminClientConfig(), rkcName, userNs.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		rc := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			remoteKubernetesClusterResourceInfo,
			originalRKC.Namespace,
			originalRKC.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleanerCollectors(rc)

		rkc, err := cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Create(ctx, originalRKC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the RemoteKubernetesCluster %q to roll out (RV=%s)", rkc.Name, rkc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
		defer waitCtx1Cancel()

		const expectedFinalizer = "scylla-operator.scylladb.com/remotekubernetescluster-protection"
		hasRKCFinalizer := func(rkc *scyllav1alpha1.RemoteKubernetesCluster) (bool, error) {
			return oslices.ContainsItem(rkc.Finalizers, expectedFinalizer), nil
		}

		rkc, err = controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx1, cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{},
			utils.IsRemoteKubernetesClusterRolledOut,
			hasRKCFinalizer,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Deleting RemoteKubernetesCluster %q", rkc.Name)
		err = cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Delete(ctx, rkcName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Awaiting RemoteKubernetesCluster %q deletion", rkc.Name)
		err = framework.WaitForObjectDeletion(ctx, f.DynamicAdminClient(), scyllav1alpha1.GroupVersion.WithResource("remotekubernetesclusters"), rkc.Namespace, rkc.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Recreating RemoteKubernetesCluster %q", rkcName)
		rkc = originalRKC.DeepCopy()
		rkc, err = cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Create(ctx, rkc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the RemoteKubernetesCluster %q to roll out (RV=%s)", rkc.Name, rkc.ResourceVersion)
		waitCtx2, waitCtx2Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
		defer waitCtx2Cancel()

		rkc, err = controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx2, cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{},
			utils.IsRemoteKubernetesClusterRolledOut,
			hasRKCFinalizer,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating ScyllaDBCluster using RemoteKubernetesCluster %q", rkcName)
		sc1 := f.GetDefaultScyllaDBCluster(map[string]*scyllav1alpha1.RemoteKubernetesCluster{rkcClusterKey: rkc})

		sc1, err = cluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()).Create(ctx, sc1, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBCluster %q to roll out (RV=%s)", sc1.Name, sc1.ResourceVersion)
		waitCtx3, waitCtx3Cancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc1)
		defer waitCtx3Cancel()
		sc1, err = controllerhelpers.WaitForScyllaDBClusterState(waitCtx3, cluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(f.Namespace()), sc1.Name, controllerhelpers.WaitForStateOptions{},
			utils.IsScyllaDBClusterRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Deleting RemoteKubernetesCluster %q", rkc.Name)
		err = cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Delete(ctx, rkcName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		rkc, err = cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters().Get(ctx, rkcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(rkc.DeletionTimestamp).NotTo(o.BeNil())

		hasIsBeingUsedCondition := func(rkc *scyllav1alpha1.RemoteKubernetesCluster) (bool, error) {
			cond := meta.FindStatusCondition(rkc.Status.Conditions, "RemoteKubernetesClusterFinalizerProgressing")
			return cond != nil &&
				cond.Reason == "IsBeingUsed" &&
				cond.Status == metav1.ConditionTrue &&
				cond.ObservedGeneration == rkc.Generation, nil
		}
		framework.By("Awaiting IsBeingUsed condition on RemoteKubernetesCluster %q", rkc.Name)
		waitCtx4, waitCtx4Cancel := utils.ContextForRemoteKubernetesClusterRollout(ctx, rkc)
		defer waitCtx4Cancel()
		rkc, err = controllerhelpers.WaitForRemoteKubernetesClusterState(waitCtx4, cluster.ScyllaAdminClient().ScyllaV1alpha1().RemoteKubernetesClusters(), rkc.Name, controllerhelpers.WaitForStateOptions{},
			hasRKCFinalizer,
			hasIsBeingUsedCondition,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Deleting ScyllaDBCluster %q", sc1.Name)
		err = cluster.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc1.Namespace).Delete(ctx, sc1.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Awaiting ScyllaDBCluster %q deletion", sc1.Name)
		err = framework.WaitForObjectDeletion(ctx, f.DynamicAdminClient(), scyllav1alpha1.GroupVersion.WithResource("scylladbclusters"), sc1.Namespace, sc1.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Awaiting RemoteKubernetesCluster %q deletion", rkc.Name)
		err = framework.WaitForObjectDeletion(ctx, f.DynamicAdminClient(), scyllav1alpha1.GroupVersion.WithResource("remotekubernetesclusters"), rkc.Namespace, rkc.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
