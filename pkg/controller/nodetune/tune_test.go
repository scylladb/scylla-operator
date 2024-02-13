// Copyright (C) 2021 ScyllaDB

package nodetune

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"github.com/scylladb/scylla-operator/pkg/util/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	testTimeout = 1 * time.Second
)

func TestStripContainerID(t *testing.T) {
	ts := []struct {
		name                string
		containerID         string
		expectedContainerID string
		expectedErr         error
	}{
		{
			name:                "Docker container",
			containerID:         "docker://5bdb9a25f236e703161902376f9377336c072a2c4fca54a5100e8e07fb7d4280",
			expectedContainerID: "5bdb9a25f236e703161902376f9377336c072a2c4fca54a5100e8e07fb7d4280",
			expectedErr:         nil,
		},
		{
			name:                "invalid",
			containerID:         "abc1234",
			expectedContainerID: "",
			expectedErr:         fmt.Errorf(`unsupported containerID format "abc1234"`),
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cid, err := stripContainerID(test.containerID)
			if !reflect.DeepEqual(err, test.expectedErr) {
				t.Errorf("expected error %s, got %s", test.expectedErr, err)
			}
			if cid != test.expectedContainerID {
				t.Errorf("expected container ID %q, got %q", test.containerID, cid)
			}
		})
	}
}

type fakeKubeletPodResources struct {
	expectations []*v1.PodResources
}

func (f *fakeKubeletPodResources) List(ctx context.Context) ([]*v1.PodResources, error) {
	if f.expectations != nil {
		return f.expectations, nil
	}
	return nil, nil
}

func (f *fakeKubeletPodResources) Close() error {
	return nil
}

func TestGetIRQCPUs(t *testing.T) {
	newScyllaPod := func(name string, qosClass corev1.PodQOSClass) *corev1.Pod {
		cid := fmt.Sprintf("docker://%s", strings.ReplaceAll(uuid.MustRandom().String(), "-", ""))
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					naming.ClusterNameLabel: "cluster",
				},
				Name:      name,
				Namespace: "default",
			},
			Status: corev1.PodStatus{
				QOSClass: qosClass,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:        naming.ScyllaContainerName,
						ContainerID: cid,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
						},
					},
				},
			},
		}
		return pod
	}

	guaranteedScylla1 := newScyllaPod("scylla-1", corev1.PodQOSGuaranteed)
	guaranteedScylla2 := newScyllaPod("scylla-2", corev1.PodQOSGuaranteed)
	burstableScylla := newScyllaPod("scylla-3", corev1.PodQOSBurstable)

	ts := []struct {
		name                string
		kubeletPodResources []*v1.PodResources
		scyllaPods          []*corev1.Pod
		hostCpuSet          string
		expectedCpuSet      string
		expectedErr         error
	}{
		{
			name: "single guaranteed Scylla",
			kubeletPodResources: []*v1.PodResources{
				{
					Name:      guaranteedScylla1.Name,
					Namespace: guaranteedScylla1.Namespace,
					Containers: []*v1.ContainerResources{
						{
							Name:   "scylla",
							CpuIds: []int64{0, 1},
						},
					},
				},
			},
			scyllaPods: []*corev1.Pod{
				guaranteedScylla1,
			},
			hostCpuSet:     "0-5",
			expectedCpuSet: "2-5",
		},
		{
			name: "multiple guaranteed Scylla",
			kubeletPodResources: []*v1.PodResources{
				{
					Name:      guaranteedScylla1.Name,
					Namespace: guaranteedScylla1.Namespace,
					Containers: []*v1.ContainerResources{
						{
							Name:   "scylla",
							CpuIds: []int64{0, 1},
						},
					},
				},
				{
					Name:      guaranteedScylla2.Name,
					Namespace: guaranteedScylla2.Namespace,
					Containers: []*v1.ContainerResources{
						{
							Name:   "scylla",
							CpuIds: []int64{5},
						},
					},
				},
			},
			scyllaPods: []*corev1.Pod{
				guaranteedScylla1,
				guaranteedScylla2,
			},
			hostCpuSet:     "0-5",
			expectedCpuSet: "2-4",
		},
		{
			name: "burstable Scylla",
			kubeletPodResources: []*v1.PodResources{
				{
					Name:      guaranteedScylla1.Name,
					Namespace: guaranteedScylla1.Namespace,
					Containers: []*v1.ContainerResources{
						{
							Name:   "scylla",
							CpuIds: []int64{0, 1, 2, 3, 4, 5},
						},
					},
				},
			},
			scyllaPods: []*corev1.Pod{
				burstableScylla,
			},
			hostCpuSet:     "0-5",
			expectedCpuSet: "0-5",
		},
	}
	for i := range ts {
		test := ts[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			hostFullCpuset, err := cpuset.Parse(test.hostCpuSet)
			if err != nil {
				t.Fatal(err)
			}

			kubeletPodResourcesClient := &fakeKubeletPodResources{
				expectations: test.kubeletPodResources,
			}

			irqCpuSet, err := getIRQCPUs(ctx, kubeletPodResourcesClient, test.scyllaPods, hostFullCpuset)
			if !reflect.DeepEqual(err, test.expectedErr) {
				t.Errorf("expected %v error, got %v", test.expectedErr, err)
			}

			if test.expectedCpuSet != irqCpuSet.String() {
				t.Errorf("expected %v IRQ cpuset, got %v", test.expectedCpuSet, irqCpuSet)
			}
		})
	}
}
