package collect

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

func GetPodLogs(ctx context.Context, podClient corev1client.PodInterface, writer io.Writer, podName string, logOptions *corev1.PodLogOptions) error {
	logsReq := podClient.GetLogs(podName, logOptions)
	readCloser, err := logsReq.Stream(ctx)
	if err != nil {
		return fmt.Errorf("can't create a log stream: %w", err)
	}
	defer func() {
		err := readCloser.Close()
		if err != nil {
			klog.ErrorS(err, "can't close log stream", "Pod", podName, "Container", logOptions.Container)
		}
	}()

	_, err = io.Copy(writer, readCloser)
	if err != nil {
		return fmt.Errorf("can't read logs: %w", err)
	}

	return nil
}
