// Copyright (C) 2021 ScyllaDB

package actions

import (
	"context"
	"reflect"
	"testing"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controllers/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSidecarUpgradeAction(t *testing.T) {
	const (
		preUpdateImage  = "sidecar:old"
		postUpdateImage = "sidecar:new"
	)
	var (
		postUpdateCommand = []string{"true"}
	)

	ctx := context.Background()
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.DebugLevel),
	})

	cluster := unit.NewMultiRackCluster(1)
	rack := cluster.Spec.Datacenter.Racks[0]
	rackSts, err := resource.StatefulSetForRack(rack, cluster, preUpdateImage, helpers.UnknownPlatform)
	if err != nil {
		t.Fatal(err)
	}

	updatedContainer := corev1.Container{
		Name:    naming.SidecarInjectorContainerName,
		Image:   postUpdateImage,
		Command: postUpdateCommand,
	}

	objects := []runtime.Object{rackSts}
	kubeClient := fake.NewSimpleClientset(objects...)

	a := NewSidecarUpgrade(cluster, updatedContainer, logger)
	s := &State{kubeclient: kubeClient}

	if err := a.Execute(ctx, s); err != nil {
		t.Fatal(err)
	}

	sts, err := kubeClient.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, rackSts.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected non nil err, got %s", err)
	}

	idx, err := naming.FindSidecarInjectorContainer(sts.Spec.Template.Spec.InitContainers)
	if err != nil {
		t.Fatalf("Expected non nil err, got %s", err)
	}

	if !reflect.DeepEqual(sts.Spec.Template.Spec.InitContainers[idx], updatedContainer) {
		t.Fatalf("Expected containers to be equal, got %+v, expected %+v", sts.Spec.Template.Spec.InitContainers[idx], updatedContainer)
	}
}
