// Copyright (C) 2021 ScyllaDB

package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controllers/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
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

func TestClusterStatus_SourceOfRackStatus(t *testing.T) {
	const (
		scyllaVersion = "1.2.3"
	)
	cluster := unit.NewSingleRackCluster(1)
	rack := cluster.Spec.Datacenter.Racks[0]

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	rackFirstPodName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(rack, cluster))
	rackFirstPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rackFirstPodName,
			Namespace: cluster.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  naming.ScyllaContainerName,
					Image: fmt.Sprintf("scylladb/scylla:%s", scyllaVersion),
				},
			},
		},
	}

	rackSts, err := resource.StatefulSetForRack(rack, cluster, "image", helpers.UnknownPlatform)
	if err != nil {
		t.Fatal(err)
	}
	rackSts.Spec.Replicas = pointer.Int32Ptr(123)
	rackSts.Status = appsv1.StatefulSetStatus{
		Replicas:      2,
		ReadyReplicas: 1,
	}
	cc := &ClusterReconciler{
		Client: fake.NewFakeClientWithScheme(scheme, []runtime.Object{rackSts, rackFirstPod}...),
	}

	if err := cc.updateStatus(context.Background(), cluster); err != nil {
		t.Errorf("expected nil, got %s", err)
	}

	rackStatus := cluster.Status.Racks[rack.Name]
	if rackStatus.Version != scyllaVersion {
		t.Errorf("expected rack version to be taken from container spec from first pod, got %s", rackStatus.Version)
	}
	if rackStatus.ReadyMembers != rackSts.Status.ReadyReplicas {
		t.Errorf("expected ready replicas to be taken from rack stateful status, got %d", rackStatus.ReadyMembers)
	}
	if rackStatus.Members != *rackSts.Spec.Replicas {
		t.Errorf("expected replicas to be taken from rack stateful spec, got %d", rackStatus.Members)
	}
}
