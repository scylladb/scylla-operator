package scyllacluster

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func verifyStatefulset(ctx context.Context, s *appsv1.StatefulSet) {
	o.Expect(s.DeletionTimestamp).To(o.BeNil())
	o.Expect(s.Status.ObservedGeneration).To(o.Equal(s.Generation))
	o.Expect(s.Spec.Replicas).NotTo(o.BeNil())
	o.Expect(s.Status.ReadyReplicas).To(o.Equal(*s.Spec.Replicas))
	o.Expect(s.Status.CurrentRevision).To(o.Equal(s.Status.UpdateRevision))
}

func verifyPodDisruptionBudget(sc *scyllav1.ScyllaCluster, pdb *policyv1beta1.PodDisruptionBudget) {
	o.Expect(pdb.ObjectMeta.OwnerReferences).To(o.ConsistOf(util.NewControllerRef(sc)))
	o.Expect(pdb.Spec.MaxUnavailable.IntValue()).To(o.Equal(1))
	o.Expect(pdb.Spec.Selector).To(o.Equal(metav1.SetAsLabelSelector(naming.ClusterLabels(sc))))
}

func verifyScyllaCluster(ctx context.Context, kubeClient kubernetes.Interface, sc *scyllav1.ScyllaCluster) {
	framework.By("Verifying the ScyllaCluster")

	o.Expect(sc.Status.Racks).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	statefulsets, err := getStatefulSetsForScyllaCluster(ctx, kubeClient.AppsV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(statefulsets).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	memberCount := 0
	for _, r := range sc.Spec.Datacenter.Racks {
		memberCount += int(r.Members)

		s := statefulsets[r.Name]

		verifyStatefulset(ctx, s)

		o.Expect(sc.Status.Racks[r.Name].ReadyMembers).To(o.Equal(r.Members))
		o.Expect(sc.Status.Racks[r.Name].ReadyMembers).To(o.Equal(s.Status.ReadyReplicas))
	}

	if sc.Status.Upgrade != nil {
		o.Expect(sc.Status.Upgrade.FromVersion).To(o.Equal(sc.Status.Upgrade.ToVersion))
	}

	pdb, err := kubeClient.PolicyV1beta1().PodDisruptionBudgets(sc.Namespace).Get(ctx, naming.PodDisruptionBudgetName(sc), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	verifyPodDisruptionBudget(sc, pdb)

	// TODO: Use scylla client to check at least "UN"
	scyllaClient, hosts, err := getScyllaClient(ctx, kubeClient.CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hosts).To(o.HaveLen(memberCount))

	status, err := scyllaClient.Status(ctx, "")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(status.DownHosts()).To(o.BeEmpty())
	o.Expect(status.LiveHosts()).To(o.HaveLen(memberCount))
}
