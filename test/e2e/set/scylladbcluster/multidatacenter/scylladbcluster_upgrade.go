// Copyright (C) 2025 ScyllaDB

package multidatacenter

import (
	"fmt"
	"maps"
	"slices"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassets "github.com/scylladb/scylla-operator/assets/config"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scylladbclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaDBCluster", framework.MultiDatacenter, func() {
	f := framework.NewFramework("scylladbcluster")

	type entry struct {
		initialScyllaDBImage string
		targetScyllaDBImage  string
	}

	g.DescribeTable("should deploy when ScyllaDB version", func(ctx g.SpecContext, e *entry) {
		ns, _ := f.CreateUserNamespace(ctx)

		workerClusters := f.WorkerClusters()
		o.Expect(workerClusters).NotTo(o.BeEmpty(), "At least 1 worker cluster is required")

		rkcMap, rkcClusterMap, err := utils.SetUpRemoteKubernetesClusters(ctx, f, workerClusters)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By(`Creating a ScyllaDBCluster`)
		sc := f.GetDefaultScyllaDBCluster(rkcMap)
		sc.Spec.ScyllaDB.Image = e.initialScyllaDBImage

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

		initialHostIDsByDC, err := utilsv1alpha1.GetHostIDsForScyllaDBCluster(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		initialHostsByDC, err := utilsv1alpha1.GetBroadcastRPCAddressesForScyllaDBCluster(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		allInitialHosts := slices.Concat(slices.Collect(maps.Values(initialHostsByDC))...)
		o.Expect(allInitialHosts).To(o.HaveLen(int(controllerhelpers.GetScyllaDBClusterNodeCount(sc))))
		di := verification.InsertAndVerifyCQLDataByDC(ctx, initialHostsByDC)
		defer di.Close()

		var initialScyllaClientReportedVersions []string
		for _, dc := range sc.Spec.Datacenters {
			clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]
			remoteNamespaceName, err := naming.RemoteNamespaceName(sc, &dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(remoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			scyllaClient, _, err := utilsv1alpha1.GetScyllaClient(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc)
			o.Expect(err).NotTo(o.HaveOccurred())

			version, err := scyllaClient.ScyllaVersion(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())
			initialScyllaClientReportedVersions = append(initialScyllaClientReportedVersions, version)
		}

		slices.Sort(initialScyllaClientReportedVersions)
		initialScyllaClientReportedVersions = slices.Compact(initialScyllaClientReportedVersions)
		o.Expect(initialScyllaClientReportedVersions).To(o.HaveLen(1), "All ScyllaDBDatacenters should report the same initial ScyllaDB version")
		initialScyllaDBVersion := initialScyllaClientReportedVersions[0]

		framework.By("Updating ScyllaDB image")
		sc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(ns.Name).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op": "replace", "path": "/spec/scyllaDB/image", "value": %q}]`, e.targetScyllaDBImage)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.ScyllaDB.Image).To(o.Equal(e.targetScyllaDBImage))

		framework.By("Waiting for the ScyllaDBCluster %q to roll out (RV=%s)", sc.Name, sc.ResourceVersion)
		postUpdateRolloutCtx, postUpdateRolloutCtxCancel := utils.ContextForMultiDatacenterScyllaDBClusterRollout(ctx, sc)
		defer postUpdateRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaDBClusterState(postUpdateRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scylladbclusterverification.Verify(ctx, sc, rkcClusterMap)
		err = scylladbclusterverification.WaitForFullQuorum(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		targetHostIDsByDC, err := utilsv1alpha1.GetHostIDsForScyllaDBCluster(ctx, rkcClusterMap, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(maps.Keys(targetHostIDsByDC)).To(o.ConsistOf(maps.Keys(initialHostsByDC)))
		for dc, dcHostIDs := range targetHostIDsByDC {
			o.Expect(initialHostIDsByDC).To(o.HaveKeyWithValue(dc, o.ConsistOf(dcHostIDs)), "Host IDs in datacenter %q should not change after ScyllaDB image update", dc)
		}

		// Client shouldn't lose connection during the image update.
		verification.VerifyCQLData(ctx, di)

		var targetScyllaClientReportedVersions []string
		for _, dc := range sc.Spec.Datacenters {
			clusterClient := rkcClusterMap[dc.RemoteKubernetesClusterName]
			remoteNamespaceName, err := naming.RemoteNamespaceName(sc, &dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			sdc, err := clusterClient.ScyllaAdminClient().ScyllaV1alpha1().ScyllaDBDatacenters(remoteNamespaceName).Get(ctx, naming.ScyllaDBDatacenterName(sc, &dc), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Verifying that datacenter %q uses the target ScyllaDB image", dc.Name)
			o.Expect(sdc.Spec.ScyllaDB.Image).To(o.Equal(e.targetScyllaDBImage), "ScyllaDB image in ScyllaDBDatacenter should match the target image")

			scyllaClient, dcHosts, err := utilsv1alpha1.GetScyllaClient(ctx, clusterClient.KubeAdminClient().CoreV1(), sdc)
			o.Expect(err).NotTo(o.HaveOccurred())

			version, err := scyllaClient.ScyllaVersion(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())
			targetScyllaClientReportedVersions = append(targetScyllaClientReportedVersions, version)

			framework.By("Verifying that snapshots created by upgrade hooks are cleared from every node in datacenter %q", dc.Name)
			for _, host := range dcHosts {
				snapshots, err := scyllaClient.ListSnapshots(ctx, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(snapshots).To(o.BeEmpty(), "Snapshots should be cleared")
			}
		}

		slices.Sort(targetScyllaClientReportedVersions)
		targetScyllaClientReportedVersions = slices.Compact(targetScyllaClientReportedVersions)
		o.Expect(targetScyllaClientReportedVersions).To(o.HaveLen(1), "All ScyllaDBDatacenters should report the same target ScyllaDB version")
		targetScyllaDBVersion := targetScyllaClientReportedVersions[0]

		o.Expect(targetScyllaDBVersion).NotTo(o.Equal(initialScyllaDBVersion), "ScyllaDB version should change after the image has been updated")
	},
		g.Entry(
			"is updated",
			&entry{
				initialScyllaDBImage: fmt.Sprintf("%s:%s", configassets.ScyllaDBImageRepository, framework.TestContext.ScyllaDBUpdateFrom),
				targetScyllaDBImage:  fmt.Sprintf("%s:%s", configassets.ScyllaDBImageRepository, framework.TestContext.ScyllaDBVersion),
			},
		),
		g.Entry(
			"is upgraded",
			&entry{
				initialScyllaDBImage: fmt.Sprintf("%s:%s", configassets.ScyllaDBImageRepository, framework.TestContext.ScyllaDBUpgradeFrom),
				targetScyllaDBImage:  fmt.Sprintf("%s:%s", configassets.ScyllaDBImageRepository, framework.TestContext.ScyllaDBVersion),
			},
		),
	)
})
