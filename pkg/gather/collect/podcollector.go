package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

type PodRuntimeCollector struct {
	Filter  func(pod *corev1.Pod) bool
	Collect func(context.Context, *corev1.Pod, *ResourceInfo) error
}

type PodCollector struct {
	restConfig     *rest.Config
	corev1Client   corev1client.CoreV1Interface
	resourceWriter *ResourceWriter
	logsLimitBytes int64

	podRuntimeCollectors []PodRuntimeCollector
}

func NewPodCollector(restConfig *rest.Config, corev1Client corev1client.CoreV1Interface, resourceWriter *ResourceWriter, logsLimitBytes int64) *PodCollector {
	pc := &PodCollector{
		restConfig:     restConfig,
		corev1Client:   corev1Client,
		resourceWriter: resourceWriter,
		logsLimitBytes: logsLimitBytes,
	}

	pc.podRuntimeCollectors = []PodRuntimeCollector{
		{
			Collect: pc.collectPodLogs,
		},
		{
			Filter:  controllerhelpers.IsScyllaContainerRunning,
			Collect: pc.collectContainerCommandOutputFunc(naming.ScyllaContainerName, "nodetool-status.log", []string{"nodetool", "status"}),
		},
		{
			Filter:  controllerhelpers.IsScyllaContainerRunning,
			Collect: pc.collectContainerCommandOutputFunc(naming.ScyllaContainerName, "nodetool-gossipinfo.log", []string{"nodetool", "gossipinfo"}),
		},
		{
			Filter:  controllerhelpers.IsScyllaContainerRunning,
			Collect: pc.collectContainerCommandOutputFunc(naming.ScyllaContainerName, "df.log", []string{"df", "-h"}),
		},
		{
			Filter:  controllerhelpers.IsScyllaContainerRunning,
			Collect: pc.collectContainerCommandOutputFunc(naming.ScyllaContainerName, "io_properties.yaml", []string{"cat", "/etc/scylla.d/io_properties.yaml"}),
		},
		{
			Filter: controllerhelpers.IsScyllaContainerRunning,
			Collect: pc.collectContainerCommandOutputFunc(naming.ScyllaContainerName, "scylla-rlimits.log", []string{
				"bash",
				"-euEo",
				"pipefail",
				"-O",
				"inherit_errexit",
				"-c",
				`prlimit --pid=$(pidof scylla)`}),
		},
		{
			Filter:  controllerhelpers.IsNodeConfigPod,
			Collect: pc.collectContainerCommandOutputFunc(naming.NodeConfigAppName, "kubelet-cpu_manager_state.log", []string{"cat", "/host/var/lib/kubelet/cpu_manager_state"}),
		},
	}

	return pc
}

func (c *PodCollector) Collect(ctx context.Context, u *unstructured.Unstructured, resourceInfo *ResourceInfo) error {
	pod := &corev1.Pod{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pod)
	if err != nil {
		return fmt.Errorf("can't convert secret from unstructured: %w", err)
	}

	err = c.resourceWriter.WriteResource(ctx, pod, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't write resource %q: %w", resourceInfo, err)
	}

	err = c.collectPodRuntimeInformation(ctx, pod, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't collect runtime information: %w", err)
	}

	return nil
}

func writeContainerLogsToFile(ctx context.Context, podClient corev1client.PodInterface, destinationPath string, podName string, logOptions *corev1.PodLogOptions) error {
	dest, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("can't open file %q: %w", destinationPath, err)
	}
	defer func() {
		err := dest.Close()
		if err != nil {
			klog.ErrorS(err, "can't close file", "Path", destinationPath)
		}
	}()

	logsReq := podClient.GetLogs(podName, logOptions)
	readCloser, err := logsReq.Stream(ctx)
	if err != nil {
		return fmt.Errorf("can't create a log stream: %w", err)
	}
	defer func() {
		err := readCloser.Close()
		if err != nil {
			klog.ErrorS(err, "can't close log stream", "Path", destinationPath, "Pod", podName, "Container", logOptions.Container)
		}
	}()

	_, err = io.Copy(dest, readCloser)
	if err != nil {
		return fmt.Errorf("can't read logs: %w", err)
	}

	return nil
}

func (c *PodCollector) collectContainerLogs(ctx context.Context, logsDir string, podMeta *metav1.ObjectMeta, podCSs []corev1.ContainerStatus, containerName string) error {
	var err error

	cs, _, found := oslices.Find(podCSs, func(s corev1.ContainerStatus) bool {
		return s.Name == containerName
	})
	if !found {
		klog.InfoS("Container doesn't yet have a status", "Pod", naming.ObjRef(podMeta), "Container", containerName)
		return nil
	}

	var limitBytes *int64
	if c.logsLimitBytes > 0 {
		limitBytes = pointer.Ptr(c.logsLimitBytes)
	}

	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Timestamps: true,
		Follow:     false,
		LimitBytes: limitBytes,
	}

	// TODO: Tolerate errors in case state changes in the meantime (like when a pod is being restarted in backoff)
	//       It's error prone to just ignore it, maybe we should retry and refreshing the state and retrying instead.
	//       https://github.com/scylladb/scylla-operator/issues/1400

	if cs.State.Running != nil {
		// Retrieve current logs.
		logOptions.Previous = false
		err = writeContainerLogsToFile(ctx, c.corev1Client.Pods(podMeta.Namespace), filepath.Join(logsDir, containerName+".current"), podMeta.Name, logOptions)
		if err != nil {
			return fmt.Errorf("can't retrieve pod logs for container %q in pod %q: %w", containerName, naming.ObjRef(podMeta), err)
		}
	}

	if cs.LastTerminationState.Terminated != nil {
		logOptions.Previous = true
		err = writeContainerLogsToFile(ctx, c.corev1Client.Pods(podMeta.Namespace), filepath.Join(logsDir, containerName+".previous"), podMeta.Name, logOptions)
		if err != nil {
			return fmt.Errorf("can't retrieve previous pod logs for container %q in pod %q: %w", containerName, naming.ObjRef(podMeta), err)
		}
	}

	return nil
}

func (c *PodCollector) collectPodRuntimeInformation(ctx context.Context, pod *corev1.Pod, resourceInfo *ResourceInfo) error {
	var errs []error
	for _, fc := range c.podRuntimeCollectors {
		if fc.Filter != nil && !fc.Filter(pod) {
			continue
		}

		err := fc.Collect(ctx, pod, resourceInfo)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't collect Pod %q runtime information: %w", naming.ObjRef(pod), err))
		}
	}

	return nil
}

func (c *PodCollector) collectPodLogs(ctx context.Context, pod *corev1.Pod, resourceInfo *ResourceInfo) error {
	podDir, err := c.createPodDirectory(pod, resourceInfo)
	if err != nil {
		return fmt.Errorf("can't create pod directory: %w", err)
	}

	for _, container := range pod.Spec.InitContainers {
		err := c.collectContainerLogs(ctx, podDir, &pod.ObjectMeta, pod.Status.InitContainerStatuses, container.Name)
		if err != nil {
			return fmt.Errorf("can't collect logs for init container %q in pod %q: %w", container.Name, naming.ObjRef(pod), err)
		}
	}

	for _, container := range pod.Spec.Containers {
		err := c.collectContainerLogs(ctx, podDir, &pod.ObjectMeta, pod.Status.ContainerStatuses, container.Name)
		if err != nil {
			return fmt.Errorf("can't collect logs for container %q in pod %q: %w", container.Name, naming.ObjRef(pod), err)
		}
	}

	for _, container := range pod.Spec.EphemeralContainers {
		err := c.collectContainerLogs(ctx, podDir, &pod.ObjectMeta, pod.Status.EphemeralContainerStatuses, container.Name)
		if err != nil {
			return fmt.Errorf("can't collect logs for ephemeral container %q in pod %q: %w", container.Name, naming.ObjRef(pod), err)
		}
	}

	return nil
}

func (c *PodCollector) createPodDirectory(pod *corev1.Pod, resourceInfo *ResourceInfo) (string, error) {
	resourceDir, err := c.resourceWriter.GetResourceDir(pod, resourceInfo)
	if err != nil {
		return "", fmt.Errorf("can't get resourceDir: %q", err)
	}
	podDir := filepath.Join(resourceDir, pod.GetName())

	err = os.MkdirAll(podDir, 0770)
	if err != nil {
		return "", fmt.Errorf("can't create pod dir %q: %w", podDir, err)
	}

	return podDir, nil
}

func (c *PodCollector) executeRemoteCommand(ctx context.Context, pod *corev1.Pod, containerName string, command []string) (stdout, stderr bytes.Buffer, err error) {
	execReq := c.corev1Client.RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, runtime.NewParameterCodec(scheme.Scheme))

	executor, err := remotecommand.NewWebSocketExecutor(c.restConfig, http.MethodPost, execReq.URL().String())
	if err != nil {
		return stdout, stderr, fmt.Errorf("can't create websocket executor for pod %q: %w", naming.ObjRef(pod), err)
	}

	err = executor.StreamWithContext(
		ctx,
		remotecommand.StreamOptions{
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		})
	if err != nil {
		return stdout, stderr, fmt.Errorf("can't execute command %q on Pod %q: %w", command, naming.ObjRef(pod), err)
	}

	return stdout, stderr, nil
}

func (c *PodCollector) collectContainerCommandOutputFunc(containerName string, filename string, command []string) func(context.Context, *corev1.Pod, *ResourceInfo) error {
	return func(ctx context.Context, pod *corev1.Pod, resourceInfo *ResourceInfo) error {
		klog.V(4).InfoS("Collecting container command output", "Namespace", pod.Namespace, "Pod", pod.Name, "Container", containerName, "Command", command)

		podDir, err := c.createPodDirectory(pod, resourceInfo)
		if err != nil {
			return fmt.Errorf("can't create pod directory: %w", err)
		}

		stdout, stderr, err := c.executeRemoteCommand(ctx, pod, containerName, command)
		if err != nil {
			return fmt.Errorf("can't execute remote command %q on Pod %q: %w", command, naming.ObjRef(pod), err)
		}

		filePath := filepath.Join(podDir, filename)
		err = saveNonEmptyBufferToFile(&stdout, filePath)
		if err != nil {
			return fmt.Errorf("can't save stdout: %w", err)
		}

		err = saveNonEmptyBufferToFile(&stderr, fmt.Sprintf("%s.stderr", filePath))
		if err != nil {
			return fmt.Errorf("can't save stderr: %w", err)
		}

		return nil
	}
}

func saveNonEmptyBufferToFile(buffer *bytes.Buffer, filePath string) error {
	if buffer.Len() == 0 {
		return nil
	}

	err := os.WriteFile(filePath, buffer.Bytes(), 0666)
	if err != nil {
		return fmt.Errorf("can't write to file %q: %w", filePath, err)
	}

	return nil
}
