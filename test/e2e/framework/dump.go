package framework

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

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
		return err
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
		return err
	}
	defer func() {
		err := readCloser.Close()
		if err != nil {
			klog.ErrorS(err, "can't close log stream", "Path", destinationPath, "Pod", podName, "Container", logOptions.Container)
		}
	}()

	_, err = io.Copy(dest, readCloser)
	if err != nil {
		return err
	}

	return nil
}

func dumpPodLogs(ctx context.Context, coreClient corev1client.CoreV1Interface, pod *corev1.Pod, resourceDir string, namespace string) error {
	podDir := filepath.Join(resourceDir, pod.Name)
	err := os.Mkdir(podDir, 0777)
	if err != nil {
		return err
	}

	for _, c := range pod.Spec.Containers {
		logOptions := &corev1.PodLogOptions{
			Container:  c.Name,
			Follow:     false,
			Timestamps: true,
		}

		// Retrieve current logs.
		logOptions.Previous = false
		err = retrieveContainerLogs(ctx, coreClient.Pods(namespace), filepath.Join(podDir, fmt.Sprintf("%s.current", c.Name)), pod.Name, logOptions)
		if err != nil {
			return err
		}

		hasPrevious := false
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != c.Name {
				continue
			}

			if cs.RestartCount > 0 {
				hasPrevious = true
			}
		}
		if hasPrevious {
			logOptions.Previous = true
			err = retrieveContainerLogs(ctx, coreClient.Pods(namespace), filepath.Join(podDir, fmt.Sprintf("%s.previous", c.Name)), pod.Name, logOptions)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DumpResource(ctx context.Context, dynamicClient dynamic.Interface, coreClient corev1client.CoreV1Interface, gvr schema.GroupVersionResource, groupDir, namespace string) error {
	list, err := dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
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
		return err
	}

	listData, err := yaml.Marshal(list.Items)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(groupDir, fmt.Sprintf("%s.yaml", gvr.Resource)), listData, 0666)
	if err != nil {
		return err
	}

	for _, obj := range list.Items {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(filepath.Join(resourceDir, fmt.Sprintf("%s.yaml", obj.GetName())), data, 0666)
		if err != nil {
			return err
		}

		switch gvr.String() {
		case corev1.SchemeGroupVersion.WithResource("pods").String():
			pod := &corev1.Pod{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pod)
			if err != nil {
				return err
			}

			err = dumpPodLogs(ctx, coreClient, pod, resourceDir, namespace)
			if err != nil {
				return err
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
	err := os.Mkdir(namespaceDir, 0777)
	if err != nil {
		return err
	}

	resourceLists, err := discoveryClient.ServerPreferredNamespacedResources()
	if err != nil {
		return err
	}

	for _, list := range resourceLists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return err
		}

		var groupDir string
		if len(gv.Group) == 0 {
			groupDir = path.Join(namespaceDir, fmt.Sprintf("core_%s", gv.Version))
		} else {
			groupDir = path.Join(namespaceDir, fmt.Sprintf("%s_%s", gv.Group, gv.Version))
		}

		err = os.Mkdir(groupDir, 0777)
		if err != nil {
			return err
		}
		resources := getContainedListableResources(list.APIResources)
		for _, r := range resources {
			gvr := gv.WithResource(r)
			err = DumpResource(ctx, dynamicClient, podClient, gvr, groupDir, namespace)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
