// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql"
	"github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/gocqlx/v2"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ScyllaClusterTrafficDuringUpgrade struct {
	scyllaCluster *scyllav1.ScyllaCluster
	dataInserter  *utils.DataInserter
}

func (t *ScyllaClusterTrafficDuringUpgrade) Name() string {
	return "ScyllaCluster traffic during upgrade"
}

func (t *ScyllaClusterTrafficDuringUpgrade) Setup(ctx context.Context, f *framework.Framework) {
	sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
	sc.Spec.Datacenter.Racks[0].Members = 3

	framework.By("Creating a ScyllaCluster")
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
	waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
	defer waitCtx1Cancel()
	sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Awaiting ScyllaCluster full availability")
	hosts, err := utils.GetScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Writing initial data")
	cluster := gocql.NewCluster(hosts...)
	cluster.Timeout *= 2

	session, err := gocqlx.WrapSession(gocql.NewSession(*cluster))
	o.Expect(err).NotTo(o.HaveOccurred())

	session.SetConsistency(gocql.Quorum)

	di, err := utils.NewDataInserter(hosts, utils.WithSession(&session))
	o.Expect(err).NotTo(o.HaveOccurred())

	err = di.Insert()
	o.Expect(err).NotTo(o.HaveOccurred())

	err = di.AwaitSchemaAgreement(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())

	data, err := di.Read()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(data).To(o.Equal(di.GetExpected()))

	t.scyllaCluster = sc
	t.dataInserter = di
}

func (t *ScyllaClusterTrafficDuringUpgrade) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}) {
	var (
		success, failures           atomic.Int64
		wg                          sync.WaitGroup
		postUpgradeRolloutCompleted = make(chan struct{})
	)

	framework.By("Continuously polling the cluster during upgrade.")
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ginkgo.GinkgoRecover()

		<-done

		sc := t.scyllaCluster

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		var err error
		t.scyllaCluster, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		close(postUpgradeRolloutCompleted)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer ginkgo.GinkgoRecover()

		wait.Until(func() {
			data, err := t.dataInserter.Read()
			if err != nil {
				framework.By("Could not read data: %v", err)
				failures.Add(1)
				return
			}
			if !equality.Semantic.DeepEqual(data, t.dataInserter.GetExpected()) {
				framework.By("Wrong data returned, expected %v, got %v", t.dataInserter.GetExpected(), data)
				failures.Add(1)
				return
			}

			success.Add(1)
		}, 500*time.Millisecond, postUpgradeRolloutCompleted)
	}()

	wg.Wait()

	totalRequests := success.Load() + failures.Load()
	ratio := float64(success.Load()) / float64(totalRequests)

	framework.By("Read requests: %d", totalRequests)
	framework.By("Read failures: %d", failures.Load())
	framework.By("Read success ratio: %.2f", ratio)
	// TODO(zimnx): Increase to 100% when #1077 is fixed.
	o.Expect(ratio).To(o.BeNumerically(">=", 0.50))
}

func (t *ScyllaClusterTrafficDuringUpgrade) Teardown(ctx context.Context, f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
