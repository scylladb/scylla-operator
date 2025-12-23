// Copyright (c) 2022 ScyllaDB.

package utils

import (
	"bytes"
	"context"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecOptions passed to ExecWithOptions
type ExecOptions struct {
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         io.Reader
	CaptureStdout bool
	CaptureStderr bool
}

// ExecWithOptions executes a command in the specified container,
// returning stdout, stderr and error. `options` allowed for
// additional parameters to be passed.
func ExecWithOptions(ctx context.Context, config *rest.Config, client corev1client.CoreV1Interface, options ExecOptions) (string, string, error) {
	const tty = false

	req := client.RESTClient().Post().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       tty,
	}, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer

	exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
	if err != nil {
		return "", "", err
	}
	err = exec.StreamWithContext(
		ctx,
		remotecommand.StreamOptions{
			Stdin:  options.Stdin,
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    tty,
		})

	return stdout.String(), stderr.String(), err
}

// ExecuteInPod executes a command in the specified container of a pod.
// If containerName is empty, it uses the first container in the pod.
func ExecuteInPod(ctx context.Context, config *rest.Config, client corev1client.CoreV1Interface, pod *corev1.Pod, containerName string, command string, args ...string) (string, string, error) {
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	return ExecWithOptions(ctx, config, client, ExecOptions{
		Command:       append([]string{command}, args...),
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: containerName,
		CaptureStdout: true,
		CaptureStderr: true,
	})
}
