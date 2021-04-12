// Copyright (C) 2021 ScyllaDB

package actions

import (
	"context"
	"reflect"
	"testing"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

type subAction struct {
	updateState map[string]bool
	updates     []string
	updateFunc  func(sts *appsv1.StatefulSet) error
}

func (a *subAction) RackUpdated(sts *appsv1.StatefulSet) (bool, error) {
	return a.updateState[sts.Name], nil
}

func (a *subAction) Update(sts *appsv1.StatefulSet) error {
	a.updates = append(a.updates, sts.Name)
	if a.updateFunc != nil {
		return a.updateFunc(sts)
	}
	return nil
}

func (a subAction) Name() string {
	return "fake-sub-action"
}

func TestRackSynchronizedAction_SubActionUpdatesRack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.DebugLevel),
	})

	cluster := unit.NewMultiRackCluster(1)
	firstRack, err := resource.StatefulSetForRack(cluster.Spec.Datacenter.Racks[0], cluster, "image")
	if err != nil {
		t.Fatal(err)
	}

	objects := []runtime.Object{firstRack}
	kubeClient := fake.NewSimpleClientset(objects...)

	sa := &subAction{
		updateFunc: func(sts *appsv1.StatefulSet) error {
			sts.Generation += 1
			return nil
		},
		updateState: map[string]bool{
			firstRack.Name: false,
		},
	}
	a := rackSynchronizedAction{
		subAction: sa,
		cluster:   cluster,
		logger:    logger,
	}
	s := &State{kubeclient: kubeClient}

	if err := a.Execute(ctx, s); err != nil {
		t.Fatal(err)
	}
	expectedUpdates := []string{firstRack.Name}
	if !reflect.DeepEqual(sa.updates, expectedUpdates) {
		t.Errorf("Expected %s updates, got %s", expectedUpdates, sa.updates)
	}

	sts, err := kubeClient.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, firstRack.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("cannot get statefulset: %s", err)
	}

	if sts.Generation == firstRack.Generation {
		t.Error("Expected sts update")
	}
}

func TestRackSynchronizedAction_RackAreUpgradedInSequence(t *testing.T) {
	ctx := context.Background()
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.DebugLevel),
	})

	cluster := unit.NewMultiRackCluster(1, 1)
	firstRack, err := resource.StatefulSetForRack(cluster.Spec.Datacenter.Racks[0], cluster, "image")
	if err != nil {
		t.Fatal(err)
	}
	firstRackReady := firstRack.DeepCopy()
	firstRackReady.Status.ObservedGeneration = firstRackReady.Generation
	firstRackReady.Status.ReadyReplicas = cluster.Spec.Datacenter.Racks[0].Members

	secondRack, err := resource.StatefulSetForRack(cluster.Spec.Datacenter.Racks[1], cluster, "image")
	if err != nil {
		t.Fatal(err)
	}
	secondRackReady := secondRack.DeepCopy()
	secondRackReady.Status.ObservedGeneration = secondRackReady.Generation
	secondRackReady.Status.ReadyReplicas = cluster.Spec.Datacenter.Racks[1].Members

	ts := []struct {
		Name            string
		Objects         []runtime.Object
		State           map[string]bool
		ExpectedUpdates []string
	}{
		{
			Name:    "nothing updated",
			Objects: []runtime.Object{firstRack, secondRack},
			State: map[string]bool{
				firstRack.Name:  false,
				secondRack.Name: false,
			},
			ExpectedUpdates: []string{firstRack.Name},
		},
		{
			Name:    "first rack updated, not ready",
			Objects: []runtime.Object{firstRack, secondRack},
			State: map[string]bool{
				firstRack.Name:  true,
				secondRack.Name: false,
			},
			ExpectedUpdates: nil,
		},
		{
			Name:    "first rack updated and ready",
			Objects: []runtime.Object{firstRackReady, secondRack},
			State: map[string]bool{
				firstRackReady.Name: true,
				secondRack.Name:     false,
			},
			ExpectedUpdates: []string{secondRack.Name},
		},
		{
			Name:    "all racks updated",
			Objects: []runtime.Object{firstRack, secondRack},
			State: map[string]bool{
				firstRack.Name:  true,
				secondRack.Name: true,
			},
			ExpectedUpdates: nil,
		},
		{
			Name:    "second rack updated, first not",
			Objects: []runtime.Object{firstRack, secondRack},
			State: map[string]bool{
				firstRack.Name:  false,
				secondRack.Name: true,
			},
			ExpectedUpdates: []string{firstRack.Name},
		},
		{
			Name:    "second rack updated and ready, first not",
			Objects: []runtime.Object{firstRack, secondRackReady},
			State: map[string]bool{
				firstRack.Name:  false,
				secondRack.Name: true,
			},
			ExpectedUpdates: []string{firstRack.Name},
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			kubeClient := fake.NewSimpleClientset(test.Objects...)

			sa := &subAction{
				updateState: test.State,
			}
			a := rackSynchronizedAction{
				subAction: sa,
				cluster:   cluster,
				logger:    logger,
			}
			s := &State{kubeclient: kubeClient}

			if err := a.Execute(ctx, s); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(sa.updates, test.ExpectedUpdates) {
				t.Errorf("Expected %s updates, got %s", test.ExpectedUpdates, sa.updates)
			}
		})
	}
}
