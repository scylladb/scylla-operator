// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func verifyScyllaDatacenter(sd *scyllav1alpha1.ScyllaDatacenter) {
	for _, r := range sd.Spec.Racks {
		o.Expect(sd.Status.Racks[r.Name].Stale).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].Stale).To(o.BeFalse())
		o.Expect(sd.Status.Racks[r.Name].ReadyNodes).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].ReadyNodes).To(o.Equal(*r.Nodes))
		o.Expect(sd.Status.Racks[r.Name].UpdatedNodes).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].UpdatedNodes).To(o.Equal(*r.Nodes))
		o.Expect(sd.Status.Racks[r.Name].Nodes).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].Nodes).To(o.Equal(*r.Nodes))
	}
}

func verifyScyllaCluster(ctx context.Context, scyllaclient scyllaclient.Interface, sc *scyllav2alpha1.ScyllaCluster) {
	framework.By("Verifying the ScyllaCluster")

	o.Expect(sc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(sc.Status.Datacenters).To(o.HaveLen(len(sc.Spec.Datacenters)))

	var totalMembers, totalUpdatedMembers, totalReadyMembers int32
	for i, dc := range sc.Spec.Datacenters {
		namespace := sc.Namespace
		if dc.RemoteKubeClusterConfigRef != nil {
			namespace = naming.RemoteNamespace(sc, dc)
		}
		scyllaDatacenter, err := scyllaclient.ScyllaV1alpha1().ScyllaDatacenters(namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(scyllaDatacenter)

		o.Expect(sc.Status.Datacenters[i].Nodes).ToNot(o.BeNil())
		totalMembers += *sc.Status.Datacenters[i].Nodes

		o.Expect(sc.Status.Datacenters[i].UpdatedNodes).ToNot(o.BeNil())
		totalUpdatedMembers += *sc.Status.Datacenters[i].UpdatedNodes

		o.Expect(sc.Status.Datacenters[i].ReadyNodes).ToNot(o.BeNil())
		totalReadyMembers += *sc.Status.Datacenters[i].ReadyNodes
	}

	o.Expect(sc.Status.Nodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.Nodes).To(o.Equal(totalMembers))
	o.Expect(sc.Status.UpdatedNodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.UpdatedNodes).To(o.Equal(totalUpdatedMembers))
	o.Expect(sc.Status.ReadyNodes).ToNot(o.BeNil())
	o.Expect(*sc.Status.ReadyNodes).To(o.Equal(totalReadyMembers))
}
