package framework

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

func retrieveContainerLogs(ctx context.Context, podClient corev1client.PodInterface, destinationPath string, podName string, logOptions *corev1.PodLogOptions) error {
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

func findContainerStatus(containerStatuses []corev1.ContainerStatus, containerName string) *corev1.ContainerStatus {
	var s *corev1.ContainerStatus
	for i := range containerStatuses {
		if containerStatuses[i].Name == containerName {
			s = &containerStatuses[i]
			break
		}
	}
	return s
}

func dumpPodLogs(ctx context.Context, coreClient corev1client.CoreV1Interface, pod *corev1.Pod, resourceDir string, namespace string) error {
	podDir := filepath.Join(resourceDir, pod.Name)
	err := os.Mkdir(podDir, 0777)
	if err != nil {
		return fmt.Errorf("can't make pod dir %q: %w", podDir, err)
	}

	for _, c := range pod.Spec.Containers {
		cs := findContainerStatus(pod.Status.ContainerStatuses, c.Name)
		if cs == nil {
			klog.InfoS("Container doesn't yet have a status", "Pod", naming.ObjRef(pod), "Container", c.Name)
			continue
		}

		logOptions := &corev1.PodLogOptions{
			Container:  c.Name,
			Follow:     false,
			Timestamps: true,
		}

		if cs.State.Running != nil {
			// Retrieve current logs.
			logOptions.Previous = false
			err = retrieveContainerLogs(ctx, coreClient.Pods(namespace), filepath.Join(podDir, fmt.Sprintf("%s.current", c.Name)), pod.Name, logOptions)
			if err != nil {
				return fmt.Errorf("can't retrieve pod logs for pod %q: %w", naming.ObjRef(pod), err)
			}
		}

		if cs.RestartCount > 0 {
			logOptions.Previous = true
			err = retrieveContainerLogs(ctx, coreClient.Pods(namespace), filepath.Join(podDir, fmt.Sprintf("%s.previous", c.Name)), pod.Name, logOptions)
			if err != nil {
				return fmt.Errorf("can't retrieve previous pod logs for pod %q: %w", naming.ObjRef(pod), err)
			}
		}
	}

	return nil
}

func DumpResource(ctx context.Context, dynamicClient dynamic.Interface, coreClient corev1client.CoreV1Interface, gvr schema.GroupVersionResource, groupDir, namespace string) error {
	list, err := dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("can't list %q: %w", gvr, err)
	}

	if len(list.Items) == 0 {
		return nil
	}

	for _, obj := range list.Items {
		obj.SetManagedFields(nil)
	}

	resourceDir := filepath.Join(groupDir, gvr.Resource)
	err = os.Mkdir(resourceDir, 0777)
	if err != nil {
		return fmt.Errorf("can't make resourceDir %q: %w", resourceDir, err)
	}

	listData, err := yaml.Marshal(list.Items)
	if err != nil {
		return fmt.Errorf("can't marshal items: %w", err)
	}

	resourceFile := filepath.Join(groupDir, fmt.Sprintf("%s.yaml", gvr.Resource))
	err = ioutil.WriteFile(resourceFile, listData, 0666)
	if err != nil {
		return fmt.Errorf("can't write resource file %q: %w", resourceFile, err)
	}

	for _, obj := range list.Items {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("can't marshal object: %w", err)
		}

		objFile := filepath.Join(resourceDir, fmt.Sprintf("%s.yaml", obj.GetName()))
		err = ioutil.WriteFile(objFile, data, 0666)
		if err != nil {
			return fmt.Errorf("can't write object file %q: %w", resourceFile, err)
		}

		switch gvr.String() {
		case corev1.SchemeGroupVersion.WithResource("pods").String():
			pod := &corev1.Pod{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pod)
			if err != nil {
				return fmt.Errorf("can't decode unstructured object: %w", err)
			}

			err = dumpPodLogs(ctx, coreClient, pod, resourceDir, namespace)
			if err != nil {
				return fmt.Errorf("can't dump pod logs: %w", err)
			}
		}
	}

	return nil
}

func getContainedListableResources(apiResources []metav1.APIResource) []string {
	var resources []string

	for _, apiResource := range apiResources {
		// Skip resources in different GV
		if len(apiResource.Group) != 0 || len(apiResource.Version) != 0 {
			continue
		}

		supportsList := false
		for _, verb := range apiResource.Verbs {
			if verb == "list" {
				supportsList = true
			}
		}

		if !supportsList {
			continue
		}

		resources = append(resources, apiResource.Name)
	}

	return resources
}

func DumpNamespace(ctx context.Context, discoveryClient discovery.DiscoveryInterface, dynamicClient dynamic.Interface, podClient corev1client.CoreV1Interface, artifactsDir, namespace string) error {
	namespaceDir := path.Join(artifactsDir, namespace)
	if err := os.Mkdir(namespaceDir, 0777); err != nil {
		return fmt.Errorf("can't make namesapce directory %q: %w", namespaceDir, err)
	}

	resourceLists, err := discoveryClient.ServerPreferredNamespacedResources()
	if err != nil {
		return fmt.Errorf("can't get server prefered resources: %w", err)
	}

	for _, list := range resourceLists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return fmt.Errorf("can't parse GroupVersion %q: %w", list.GroupVersion, err)
		}

		var groupDir string
		if len(gv.Group) == 0 {
			groupDir = path.Join(namespaceDir, fmt.Sprintf("core_%s", gv.Version))
		} else {
			groupDir = path.Join(namespaceDir, fmt.Sprintf("%s_%s", gv.Group, gv.Version))
		}

		err = os.Mkdir(groupDir, 0777)
		if err != nil {
			return fmt.Errorf("can't make group directory %q: %w", groupDir, err)
		}
		resources := getContainedListableResources(list.APIResources)
		for _, r := range resources {
			gvr := gv.WithResource(r)
			err = DumpResource(ctx, dynamicClient, podClient, gvr, groupDir, namespace)
			if err != nil {
				return fmt.Errorf("can't dump resource %q in namespace %q: %w", gvr, namespace, err)
			}
		}
	}

	return nil
}
