// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster upgrades", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	type entry struct {
		rackSize       int32
		rackCount      int32
		initialVersion string
		updatedVersion string
	}

	describeEntry := func(e *entry) string {
		return fmt.Sprintf("with %d member(s) and %d rack(s) from %s to %s", e.rackSize, e.rackCount, e.initialVersion, e.updatedVersion)
	}

	g.DescribeTable("should deploy and update",
		func(e *entry) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
			sc.Spec.Version = e.initialVersion

			o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
			rack := &sc.Spec.Datacenter.Racks[0]
			sc.Spec.Datacenter.Racks = make([]scyllav1.RackSpec, 0, e.rackCount)
			for i := int32(0); i < e.rackCount; i++ {
				r := rack.DeepCopy()
				r.Name = fmt.Sprintf("rack-%d", i)
				r.Members = e.rackSize
				sc.Spec.Datacenter.Racks = append(sc.Spec.Datacenter.Racks, *r)
			}

			framework.By("Creating a ScyllaCluster")
			sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sc.Spec.Version).To(o.Equal(e.initialVersion))

			framework.By("Waiting for the ScyllaCluster to deploy (RV=%s)", sc.ResourceVersion)
			waitCtx1, waitCtx1Cancel := v1utils.ContextForRollout(ctx, sc)
			defer waitCtx1Cancel()
			sc, err = v1utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
			hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
			o.Expect(hosts).To(o.HaveLen(int(e.rackCount * e.rackSize)))
			di := insertAndVerifyCQLData(ctx, hosts)
			defer di.Close()

			framework.By("triggering and update")
			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
				ctx,
				sc.Name,
				types.MergePatchType,
				[]byte(fmt.Sprintf(
					`{"metadata":{"uid":"%s"},"spec":{"version":"%s"}}`,
					sc.UID,
					e.updatedVersion,
				)),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sc.Spec.Version).To(o.Equal(e.updatedVersion))

			framework.By("Waiting for the ScyllaCluster to re-deploy (RV=%s)", sc.ResourceVersion)
			waitCtx2, waitCtx2Cancel := v1utils.ContextForRollout(ctx, sc)
			defer waitCtx2Cancel()
			sc, err = v1utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, v1utils.IsScyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), sc)
			o.Expect(hosts).To(o.ConsistOf(getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)))
			verifyCQLData(ctx, di)
		},
		// Test 1 and 3 member rack to cover e.g. handling PDBs correctly.
		g.Entry(describeEntry, &entry{
			rackCount:      1,
			rackSize:       1,
			initialVersion: updateFromScyllaVersion,
			updatedVersion: updateToScyllaVersion,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:      1,
			rackSize:       3,
			initialVersion: updateFromScyllaVersion,
			updatedVersion: updateToScyllaVersion,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:      1,
			rackSize:       1,
			initialVersion: upgradeFromScyllaVersion,
			updatedVersion: upgradeToScyllaVersion,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:      1,
			rackSize:       3,
			initialVersion: upgradeFromScyllaVersion,
			updatedVersion: upgradeToScyllaVersion,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:      2,
			rackSize:       3,
			initialVersion: upgradeFromScyllaVersion,
			updatedVersion: upgradeToScyllaVersion,
		}),
	)
})
