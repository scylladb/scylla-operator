package scylladbdatacenter

import (
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func Test_updateRackStatus(t *testing.T) {
	t.Parallel()

	newRackStatefulSet := func(rackName string, replicas int32) *appsv1.StatefulSet {
		sts := newStatefulSet("sts-" + rackName)
		sts.Labels = map[string]string{naming.RackNameLabel: rackName}
		sts.Spec.Replicas = new(replicas)
		sts.Status.ReadyReplicas = replicas
		return sts
	}

	emptyPodLister := corev1listers.NewPodLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}))

	t.Run("replaces an existing rack status", func(t *testing.T) {
		t.Parallel()

		status := &scyllav1alpha1.ScyllaDBDatacenterStatus{
			Racks: []scyllav1alpha1.RackStatus{{Name: "a"}, {Name: "b"}},
		}

		err := updateRackStatus(emptyPodLister, newScyllaDBDatacenter(), status, newRackStatefulSet("b", 3), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(status.Racks) != 2 || status.Racks[1].Name != "b" || *status.Racks[1].Nodes != 3 || *status.Racks[1].ReadyNodes != 3 {
			t.Errorf("unexpected rack statuses: %+v", status.Racks)
		}
	})

	t.Run("appends a missing rack status", func(t *testing.T) {
		t.Parallel()

		status := &scyllav1alpha1.ScyllaDBDatacenterStatus{
			Racks: []scyllav1alpha1.RackStatus{{Name: "a"}},
		}

		err := updateRackStatus(emptyPodLister, newScyllaDBDatacenter(), status, newRackStatefulSet("b", 1), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(status.Racks) != 2 || status.Racks[1].Name != "b" || *status.Racks[1].Nodes != 1 {
			t.Errorf("unexpected rack statuses: %+v", status.Racks)
		}
	})

	t.Run("fails without a rack label", func(t *testing.T) {
		t.Parallel()

		status := &scyllav1alpha1.ScyllaDBDatacenterStatus{}
		err := updateRackStatus(emptyPodLister, newScyllaDBDatacenter(), status, newStatefulSet("unlabeled"), nil)
		if err == nil {
			t.Fatal("expected an error")
		}
		if len(status.Racks) != 0 {
			t.Errorf("expected no rack statuses, got %+v", status.Racks)
		}
	})
}
