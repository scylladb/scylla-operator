// Copyright (C) 2021 ScyllaDB

package integration

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
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

func destroyNamespace(ctx context.Context, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, namespace string) error {
	klog.Infof("Destroying namespace %q", namespace)
	defer klog.Infof("Destroyed namespace %q", namespace)

	zero := int64(0)
	orphanPolicy := metav1.DeletePropagationOrphan
	immediateOrphanDeletion := metav1.DeleteOptions{
		GracePeriodSeconds: &zero,
		PropagationPolicy:  &orphanPolicy,
	}

	resources, err := kubeClient.Discovery().ServerPreferredNamespacedResources()
	if err != nil {
		return err
	}
	deletableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"delete"}}, resources)
	groupVersionResources, err := discovery.GroupVersionResources(deletableResources)
	if err != nil {
		return err
	}

	var errors []error
forResource:
	for gvr := range groupVersionResources {
		objList, err := dynamicClient.Resource(gvr).Namespace(namespace).List(
			ctx,
			metav1.ListOptions{
				LabelSelector: labels.Everything().String(),
			},
		)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		for _, obj := range objList.Items {
			err = dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Delete(
				ctx,
				obj.GetName(),
				immediateOrphanDeletion,
			)
			if err != nil && !apierrors.IsNotFound(err) {
				errors = append(errors, err)
				continue forResource
			}
		}
	}

	aggregateError := kerrors.NewAggregate(errors)
	if aggregateError != nil {
		return aggregateError
	}

	err = kubeClient.CoreV1().Namespaces().Delete(ctx, namespace, immediateOrphanDeletion)
	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := kubeClient.CoreV1().Namespaces().Finalize(
			ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:       namespace,
					Finalizers: nil,
				},
			},
			metav1.UpdateOptions{},
		)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}
