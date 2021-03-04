// Copyright (C) 2021 ScyllaDB

package cluster

import (
	"context"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterStatus_RackStatusIsRemovedWhenStatefulSetNoLongerExists(t *testing.T) {
	cluster := unit.NewSingleRackCluster(1)
	rack := cluster.Spec.Datacenter.Racks[0]

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cc := &ClusterReconciler{
		Client: fake.NewFakeClientWithScheme(scheme),
	}

	if err := cc.updateStatus(context.Background(), cluster); err != nil {
		t.Errorf("expected nil, got %s", err)
	}

	if v, ok := cluster.Status.Racks[rack.Name]; ok {
		t.Errorf("expected rack status to be cleared, got %v", v)
	}
}
