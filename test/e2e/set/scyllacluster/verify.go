package scyllacluster

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

func verifyStatefulset(ctx context.Context, s *appsv1.StatefulSet) {
	o.Expect(s.DeletionTimestamp).To(o.BeNil())
	o.Expect(s.Status.ObservedGeneration).To(o.Equal(s.Generation))
	o.Expect(s.Spec.Replicas).NotTo(o.BeNil())
	o.Expect(s.Status.ReadyReplicas).To(o.Equal(*s.Spec.Replicas))
	o.Expect(s.Status.CurrentRevision).To(o.Equal(s.Status.UpdateRevision))
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

	// TODO: Use scylla client to check at least "UN"
	scyllaClient, _, err := getScyllaClient(ctx, kubeClient.CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())

	status, err := scyllaClient.Status(ctx, "")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(status.DownHosts()).To(o.BeEmpty())
	o.Expect(status.LiveHosts()).To(o.HaveLen(memberCount))
}
