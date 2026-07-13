// Copyright (C) 2026 ScyllaDB

package collect

import (
	"context"
	"reflect"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	kubetesting "k8s.io/client-go/testing"
)

type logRequestKey struct {
	container string
	previous  bool
}

func TestPodCollector_CollectAndFollowLogs_FollowOnlyCurrentLogs(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "my-pod",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "running-init-container"},
				{Name: "prev-init-container"},
				{Name: "terminated-init-container"},
			},
			Containers: []corev1.Container{
				{Name: "running-container"},
				{Name: "prev-container"},
				{Name: "terminated-container"},
			},
			EphemeralContainers: []corev1.EphemeralContainer{
				{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "running-ephemeral-container"}},
				{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "prev-ephemeral-container"}},
				{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "terminated-ephemeral-container"}},
			},
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "running-init-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "prev-init-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				},
				{
					Name:  "terminated-init-container",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "running-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "prev-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				},
				{
					Name:  "terminated-container",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}},
				},
			},
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "running-ephemeral-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "prev-ephemeral-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{},
					},
				},
				{
					Name:  "terminated-ephemeral-container",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}},
				},
			},
		},
	}

	fakeKubeClient := kubefakeclient.NewSimpleClientset(pod)

	var mu sync.Mutex
	capturedOpts := map[logRequestKey]*corev1.PodLogOptions{}
	fakeKubeClient.PrependReactor("get", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
		genericAction, ok := action.(kubetesting.GenericAction)
		if !ok || genericAction.GetSubresource() != "log" {
			return false, nil, nil
		}

		opts, ok := genericAction.GetValue().(*corev1.PodLogOptions)
		if !ok {
			return false, nil, nil
		}

		key := logRequestKey{container: opts.Container, previous: opts.Previous}
		mu.Lock()
		capturedOpts[key] = opts.DeepCopy()
		mu.Unlock()

		return false, nil, nil
	})

	tmpDir := t.TempDir()
	printers := []ResourcePrinterInterface{
		&OmitManagedFieldsPrinter{Delegate: &YAMLPrinter{}},
	}

	u := &unstructured.Unstructured{}
	err := kubernetesscheme.Scheme.Convert(pod, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

	fakeDynamicClient := dynamicfakeclient.NewSimpleDynamicClient(kubernetesscheme.Scheme, u)
	collector := NewCollector(tmpDir, printers, nil, nil, fakeKubeClient.CoreV1(), fakeDynamicClient, false, false, 0)

	resourceInfo := &ResourceInfo{
		Scope:    meta.RESTScopeNamespace,
		Resource: corev1.SchemeGroupVersion.WithResource("pods"),
	}

	err = collector.CollectResourceObjectWithOptionsAndFollowLogs(t.Context(), resourceInfo, pod.Namespace, pod.Name, CollectObjectOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedFollow := map[logRequestKey]bool{
		{container: "running-init-container", previous: false}:         true,
		{container: "prev-init-container", previous: false}:            true,
		{container: "prev-init-container", previous: true}:             false,
		{container: "terminated-init-container", previous: false}:      false,
		{container: "running-container", previous: false}:              true,
		{container: "prev-container", previous: false}:                 true,
		{container: "prev-container", previous: true}:                  false,
		{container: "terminated-container", previous: false}:           false,
		{container: "running-ephemeral-container", previous: false}:    true,
		{container: "prev-ephemeral-container", previous: false}:       true,
		{container: "prev-ephemeral-container", previous: true}:        false,
		{container: "terminated-ephemeral-container", previous: false}: false,
	}

	if len(capturedOpts) != len(expectedFollow) {
		t.Errorf("expected %d log requests, got %d: %v", len(expectedFollow), len(capturedOpts), capturedOpts)
	}

	for key, wantFollow := range expectedFollow {
		opts, ok := capturedOpts[key]
		if !ok {
			t.Errorf("no log request for container=%q previous=%v", key.container, key.previous)
			continue
		}

		if opts.Follow != wantFollow {
			t.Errorf("container=%q previous=%v: expected Follow=%v, got Follow=%v", key.container, key.previous, wantFollow, opts.Follow)
		}
	}
}

func TestPodCollector_CollectAndFollowLogs_CallsStreamOpenCallbackAfterAllCurrentStreamsOpen(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "my-pod",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "running-init-container"},
			},
			Containers: []corev1.Container{
				{Name: "running-container-1"},
				{Name: "running-container-2"},
			},
			EphemeralContainers: []corev1.EphemeralContainer{
				{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "running-ephemeral-container"}},
			},
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "running-init-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "running-container-1",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
				{
					Name:  "running-container-2",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "running-ephemeral-container",
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}

	fakeKubeClient := kubefakeclient.NewSimpleClientset(pod)

	var mu sync.Mutex
	openContainerStreams := map[string]bool{}
	fakeKubeClient.PrependReactor("get", "pods", func(action kubetesting.Action) (bool, runtime.Object, error) {
		genericAction, ok := action.(kubetesting.GenericAction)
		if !ok || genericAction.GetSubresource() != "log" {
			return false, nil, nil
		}

		opts, ok := genericAction.GetValue().(*corev1.PodLogOptions)
		if !ok || !opts.Follow {
			return false, nil, nil
		}

		mu.Lock()
		openContainerStreams[opts.Container] = true
		mu.Unlock()

		return false, nil, nil
	})

	tmpDir := t.TempDir()
	printers := []ResourcePrinterInterface{
		&OmitManagedFieldsPrinter{Delegate: &YAMLPrinter{}},
	}

	u := &unstructured.Unstructured{}
	err := kubernetesscheme.Scheme.Convert(pod, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

	fakeDynamicClient := dynamicfakeclient.NewSimpleDynamicClient(kubernetesscheme.Scheme, u)
	collector := NewCollector(tmpDir, printers, nil, nil, fakeKubeClient.CoreV1(), fakeDynamicClient, false, false, 0)

	resourceInfo := &ResourceInfo{
		Scope:    meta.RESTScopeNamespace,
		Resource: corev1.SchemeGroupVersion.WithResource("pods"),
	}

	expectedOpenedContainers := map[string]bool{
		"running-init-container":      true,
		"running-container-1":         true,
		"running-container-2":         true,
		"running-ephemeral-container": true,
	}

	err = collector.CollectResourceObjectWithOptionsAndFollowLogs(context.Background(), resourceInfo, pod.Namespace, pod.Name, CollectObjectOptions{}, func() {
		mu.Lock()
		defer mu.Unlock()

		if !reflect.DeepEqual(openContainerStreams, expectedOpenedContainers) {
			t.Fatalf("expected opened containers %v, got %v", expectedOpenedContainers, openContainerStreams)
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
