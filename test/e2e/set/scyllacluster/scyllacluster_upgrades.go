// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	gt "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster upgrades", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	type entry struct {
		rackSize       int32
		initialVersion string
		updatedVersion string
	}

	describeEntry := func(e *entry) string {
		return fmt.Sprintf("with %d member(s) from %s to %s", e.rackSize, e.initialVersion, e.updatedVersion)
	}

	gt.DescribeTable("should deploy and update",
		func(e *entry) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimout)
			defer cancel()

			sc := scyllaclusterfixture.BasicScyllaCluster.ReadOrFail()
			sc.Spec.Version = e.initialVersion
			sc.Spec.Datacenter.Racks[0].Members = e.rackSize

			framework.By("Creating a ScyllaCluster")
			err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sc.Spec.Version).To(o.Equal(e.initialVersion))

			framework.By("Waiting for the ScyllaCluster to deploy")
			waitCtx1, waitCtx1Cancel := contextForRollout(ctx, sc)
			defer waitCtx1Cancel()
			sc, err = waitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, getMemberCount(sc))
			o.Expect(err).NotTo(o.HaveOccurred())
			defer di.Close()

			err = di.Insert()
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

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

			// FIXME: Remove when we have observedGeneration. Until then this is racy.
			time.Sleep(10 * time.Second)

			framework.By("Waiting for the ScyllaCluster to re-deploy")
			waitCtx2, waitCtx2Cancel := contextForRollout(ctx, sc)
			defer waitCtx2Cancel()
			sc, err = waitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = di.UpdateClientEndpoints(ctx, sc)
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaCluster(ctx, f.KubeClient(), sc, di)
		},
		// Test 1 and 3 member rack to cover e.g. handling PDBs correctly.
		gt.Entry(describeEntry, &entry{
			rackSize:       1,
			initialVersion: updateFromScyllaVersion,
			updatedVersion: updateToScyllaVersion,
		}),
		gt.Entry(describeEntry, &entry{
			rackSize:       3,
			initialVersion: updateFromScyllaVersion,
			updatedVersion: updateToScyllaVersion,
		}),
		gt.Entry(describeEntry, &entry{
			rackSize:       1,
			initialVersion: upgradeFromScyllaVersion,
			updatedVersion: upgradeToScyllaVersion,
		}),
		gt.Entry(describeEntry, &entry{
			rackSize:       3,
			initialVersion: upgradeFromScyllaVersion,
			updatedVersion: upgradeToScyllaVersion,
		}),
	)
})
