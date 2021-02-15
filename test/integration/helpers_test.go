// Copyright (C) 2021 ScyllaDB

package integration

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func dumpUnstructuredObj(obj unstructuredv1.Unstructured) (string, error) {
	// Prevent managedFields from polluting the output
	obj.SetManagedFields(nil)

	bytes, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", fmt.Errorf("can't marshal obj to yaml: %w", err)
	}

	return string(bytes), nil
}

func dumpObjects(ctx context.Context, client dynamic.Interface, resource schema.GroupVersionResource, namespace string, options metav1.ListOptions) {
	objList, err := client.Resource(resource).Namespace(namespace).List(ctx, options)
	if err != nil {
		fmt.Printf("Can't dump objects for resource %q: %v\n", resource, err)
		return
	}

	for _, obj := range objList.Items {
		s, err := dumpUnstructuredObj(obj)
		if err != nil {
			fmt.Printf("Can't dump objects for resource %q: %v\n", resource, err)
			return
		}

		fmt.Println(s)
	}
}
