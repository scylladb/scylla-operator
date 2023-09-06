// Copyright (C) 2021 ScyllaDB

package identity

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMember_GetSeeds(t *testing.T) {
	t.Parallel()

	createPodAndSvc := func(name, ip string, creationTimestamp time.Time) (*corev1.Pod, *corev1.Service) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "namespace",
				Labels: map[string]string{
					"scylla/cluster":               "my-cluster",
					"app":                          "scylla",
					"app.kubernetes.io/name":       "scylla",
					"app.kubernetes.io/managed-by": "scylla-operator",
				},
				CreationTimestamp: metav1.NewTime(creationTimestamp),
			},
		}
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "namespace",
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: ip,
			},
		}
		return pod, svc
	}

	now := time.Now()
	firstPod, firstService := createPodAndSvc("pod-0", "1.1.1.1", now)
	secondPod, secondService := createPodAndSvc("pod-1", "2.2.2.2", now.Add(time.Second))
	thirdPod, thirdService := createPodAndSvc("pod-2", "3.3.3.3", now.Add(2*time.Second))

	ts := []struct {
		name          string
		memberName    string
		memberIP      string
		objects       []runtime.Object
		externalSeeds []string
		expectSeeds   []string
		expectError   error
	}{
		{
			name:        "error when no pods are found",
			memberName:  firstPod.Name,
			memberIP:    firstService.Spec.ClusterIP,
			objects:     []runtime.Object{},
			expectError: fmt.Errorf("internal error: can't find any pod for this cluster, including itself"),
		},
		{
			name:        "bootstraps with itself when cluster is empty",
			memberName:  firstPod.Name,
			memberIP:    firstService.Spec.ClusterIP,
			objects:     []runtime.Object{firstPod, firstService},
			expectSeeds: []string{firstService.Spec.ClusterIP},
		},
		{
			name:        "bootstrap with first created UN node",
			memberName:  firstPod.Name,
			memberIP:    firstService.Spec.ClusterIP,
			objects:     []runtime.Object{firstPod, firstService, markPodReady(secondPod), secondService, markPodReady(thirdPod), thirdService},
			expectSeeds: []string{secondService.Spec.ClusterIP},
		},
		{
			name:        "bootstrap only with UN node",
			memberName:  firstPod.Name,
			memberIP:    firstService.Spec.ClusterIP,
			objects:     []runtime.Object{firstPod, firstService, secondPod, secondService, markPodReady(thirdPod), thirdService},
			expectSeeds: []string{thirdService.Spec.ClusterIP},
		},
		{
			name:        "bootstrap with first created Pod when all are down",
			memberName:  firstPod.Name,
			memberIP:    firstService.Spec.ClusterIP,
			objects:     []runtime.Object{firstPod, firstService, secondPod, secondService, thirdPod, thirdService},
			expectSeeds: []string{secondService.Spec.ClusterIP},
		},
		{
			name:          "bootstrap with external seeds only when cluster is empty and external seeds are provided",
			memberName:    firstPod.Name,
			memberIP:      firstService.Spec.ClusterIP,
			objects:       []runtime.Object{firstPod, firstService},
			externalSeeds: []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:   []string{"10.0.1.1", "10.0.1.2"},
		},
		{
			name:          "bootstrap with external seeds and first created UN node when external seeds are provided",
			memberName:    firstPod.Name,
			memberIP:      firstService.Spec.ClusterIP,
			objects:       []runtime.Object{firstPod, firstService, markPodReady(secondPod), secondService, markPodReady(thirdPod), thirdService},
			externalSeeds: []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:   []string{"10.0.1.1", "10.0.1.2", secondService.Spec.ClusterIP},
		},
		{
			name:          "bootstrap with external seeds and first created Pod when all Pods from DC are down and external seeds are provided",
			memberName:    firstPod.Name,
			memberIP:      firstService.Spec.ClusterIP,
			objects:       []runtime.Object{firstPod, firstService, secondPod, secondService, thirdPod, thirdService},
			externalSeeds: []string{"10.0.1.1", "10.0.1.2"},
			expectSeeds:   []string{"10.0.1.1", "10.0.1.2", secondService.Spec.ClusterIP},
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			member := Member{
				Cluster:   "my-cluster",
				Namespace: "namespace",
				Name:      test.memberName,
				StaticIP:  test.memberIP,
			}

			fakeClient := fake.NewSimpleClientset(test.objects...)
			seeds, err := member.GetSeeds(ctx, fakeClient.CoreV1(), test.externalSeeds)
			if !reflect.DeepEqual(err, test.expectError) {
				t.Errorf("expected error %v, got %v", test.expectError, err)
			}
			if !reflect.DeepEqual(seeds, test.expectSeeds) {
				t.Errorf("expected seeds %v, got %v", test.expectSeeds, seeds)
			}
		})
	}
}

func markPodReady(pod *corev1.Pod) *corev1.Pod {
	p := pod.DeepCopy()
	cond := controllerhelpers.GetPodCondition(p.Status.Conditions, corev1.PodReady)
	if cond != nil {
		cond.Status = corev1.ConditionTrue
		return p
	}

	p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})

	return p
}
