// Copyright (C) 2017 ScyllaDB

package cluster

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/cmd/scylla-operator/options"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestGetOperatorImage(t *testing.T) {
	tt := []struct {
		name            string
		containerImages []string
		expectedImage   string
		expectedError   error
	}{
		{
			name: "single correct",
			containerImages: []string{
				"scylladb/scylla-operator:0.3.0",
			},
			expectedImage: "scylladb/scylla-operator:0.3.0",
			expectedError: nil,
		},
		{
			name: "operator with sidecar",
			containerImages: []string{
				"scylladb/scylla-operator:0.3.0",
				"random/sidecar:latest",
			},
			expectedImage: "scylladb/scylla-operator:0.3.0",
			expectedError: nil,
		},
		{
			name: "custom operator",
			containerImages: []string{
				"random/my-scylla-operator:0.3.0",
			},
			expectedImage: "random/my-scylla-operator:0.3.0",
			expectedError: nil,
		},
		{
			name: "incorrect operator",
			containerImages: []string{
				"random/my-operator:0.3.0",
			},
			expectedImage: "",
			expectedError: errors.New("cannot find scylla operator container in pod spec"),
		},
	}

	for _, tc := range tt {
		t.Run(t.Name(), func(t *testing.T) {
			const (
				namespace = "ns"
				name      = "name"
			)
			opts := &options.OperatorOptions{
				CommonOptions: &options.CommonOptions{
					Name:      name,
					Namespace: namespace,
				},
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			for _, ci := range tc.containerImages {
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
					Image: ci,
				})
			}

			kubeClientFake := kubefake.NewSimpleClientset(pod)
			image, err := GetOperatorImage(context.Background(), kubeClientFake, opts)
			// Can't use DeepEqual because of different stacks in pkg/error
			if (err == nil && tc.expectedError != nil) || (err != nil && tc.expectedError == nil) || (err != tc.expectedError && err.Error() != tc.expectedError.Error()) {
				t.Errorf("expected error %#v, got %#v", tc.expectedError, err)
			}

			if image != tc.expectedImage {
				t.Errorf("expected image %q, got %q", tc.expectedImage, image)
			}
		})
	}
}
