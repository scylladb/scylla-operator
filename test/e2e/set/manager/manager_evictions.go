package manager

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	managerfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/manager"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = g.Describe("ScyllaManager eviction", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllamanager")

	g.It("should prevent from evicting", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sm := managerfixture.Manager.ReadOrFail()

		mc := scyllafixture.BasicScyllaCluster.ReadOrFail()

		framework.By("Creating a ScyllaManager Cluster")
		mc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, mc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the ScyllaManager Cluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, mc)
		defer waitCtx1Cancel()
		mc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), mc.Namespace, mc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaManager")
		sm.Spec.Database.Connection.Server = naming.CrossNamespaceServiceNameForCluster(mc)
		sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaManagers(f.Namespace()).Create(ctx, sm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.By("Waiting for the ScyllaManager to deploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForScyllaManagerRollout(ctx)
		defer waitCtx2Cancel()
		sm, err = utils.WaitForManagerState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1(), sm.Namespace, sm.Name, utils.WaitForStateOptions{}, utils.IsScyllaManagerRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Forbidding to evict a pod")
		pods, err := f.KubeAdminClient().CoreV1().Pods(f.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(naming.ManagerLabels(sm)).String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		l := len(pods.Items)
		for _, pod := range pods.Items {
			e := &policyv1.Eviction{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name,
					Namespace: f.Namespace(),
				},
			}
			err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).EvictV1(ctx, e)
			l = l - 1
			if l > 0 {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else { // Only one pod is left.
				o.Expect(err).Should(o.MatchError("Cannot evict pod as it would violate the pod's disruption budget."))
			}
		}
	})
})
