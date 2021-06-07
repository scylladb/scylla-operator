// Copyright (C) 2021 ScyllaDB

package helpers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetCloudPlatform(t *testing.T) {
	ts := []struct {
		Name     string
		Node     *corev1.Node
		Expected CloudPlatform
	}{
		{
			Name:     "GKE",
			Expected: GKEPlatform,
			Node: &corev1.Node{
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KernelVersion: "5.4.0-1039-gke",
					},
				},
			},
		},
		{
			Name:     "EKS",
			Expected: EKSPlatform,
			Node: &corev1.Node{
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KernelVersion: "4.15.0-1018-aws",
					},
				},
			},
		},
		{
			Name:     "Azure",
			Expected: AKSPlatform,
			Node: &corev1.Node{
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KernelVersion: "4.15.0-1039-azure",
					},
				},
			},
		},
		{
			Name:     "Unknown",
			Expected: UnknownPlatform,
			Node: &corev1.Node{
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KernelVersion: "5.4.0-1039",
					},
				},
			},
		},
	}

	for i := range ts {
		test := ts[i]
		t.Run(test.Name, func(t *testing.T) {
			platform := GetCloudPlatform(test.Node)
			if platform != test.Expected {
				t.Errorf("Expected %s, got %s", test.Expected, platform)
			}
		})
	}
}
