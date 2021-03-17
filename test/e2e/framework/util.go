// Copyright (C) 2021 ScyllaDB

package framework

import (
	"context"
	"fmt"
	"sort"

	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

func WaitForServiceAccount(ctx context.Context, c corev1client.CoreV1Interface, namespace, name string) (*corev1.ServiceAccount, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return c.ServiceAccounts(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return c.ServiceAccounts(namespace).Watch(ctx, options)
		},
	}
	event, err := watchtools.UntilWithSync(
		ctx,
		lw,
		&corev1.ServiceAccount{},
		nil,
		func(e watch.Event) (bool, error) {
			switch t := e.Type; t {
			case watch.Added, watch.Modified:
				sa := e.Object.(*corev1.ServiceAccount)
				return len(sa.Secrets) > 0, nil
			case watch.Deleted, watch.Bookmark:
				return false, nil
			case watch.Error:
				return true, apierrors.FromObject(e.Object)
			default:
				return true, fmt.Errorf("unexpected event type %v", t)
			}
		},
	)
	if err != nil {
		return nil, err
	}

	return event.Object.(*corev1.ServiceAccount), nil
}

func WaitForObjectDeletion(ctx context.Context, dynamicClient dynamic.Interface, resource schema.GroupVersionResource, namespace, name string, uid types.UID) error {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return dynamicClient.Resource(resource).Namespace(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return dynamicClient.Resource(resource).Namespace(namespace).Watch(ctx, options)
		},
	}
	_, err := watchtools.UntilWithSync(
		ctx,
		lw,
		&unstructured.Unstructured{},
		func(store cache.Store) (bool, error) {
			_, exists, err := store.Get(&metav1.ObjectMeta{Namespace: namespace, Name: name})
			if err != nil {
				return true, err
			}
			if !exists {
				return true, nil
			}
			return false, nil
		},
		func(e watch.Event) (bool, error) {
			switch t := e.Type; t {
			case watch.Added, watch.Bookmark:
				return false, nil
			case watch.Modified:
				// DeltaFIFO can return modified event on re-list if the object is recreated in the meantime
				if e.Object.(metav1.Object).GetUID() != uid {
					return true, nil
				}
				return false, nil
			case watch.Deleted:
				return true, nil
			case watch.Error:
				return true, apierrors.FromObject(e.Object)
			default:
				return true, fmt.Errorf("unexpected event type %v", t)
			}
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// TODO: Replace when Scylla cluster can configure a SA or use the default one.
func SetupScyllaClusterSA(ctx context.Context, coreClient corev1client.CoreV1Interface, rbacClient rbacv1client.RbacV1Interface, namespace, scyllaClusterName string) error {
	sa, err := coreClient.ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-member", scyllaClusterName),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = rbacClient.RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: sa.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  corev1.GroupName,
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: sa.Namespace,
				Name:      sa.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "scyllacluster-member",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func DumpEventsInNamespace(ctx context.Context, c kubernetes.Interface, namespace string) {
	events, err := c.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	By(fmt.Sprintf("Found %d events.", len(events.Items)))
	// Sort events by their first timestamp
	sortedEvents := events.Items
	if len(sortedEvents) > 1 {
		sort.Slice(sortedEvents, func(i, j int) bool {
			if sortedEvents[i].FirstTimestamp.Equal(&sortedEvents[j].FirstTimestamp) {
				return sortedEvents[i].InvolvedObject.Name < sortedEvents[j].InvolvedObject.Name
			}
			return sortedEvents[i].FirstTimestamp.Before(&sortedEvents[j].FirstTimestamp)
		})
	}
	for _, e := range sortedEvents {
		Infof("At %v - event for %v: %v %v: %v", e.FirstTimestamp, e.InvolvedObject.Name, e.Source, e.Reason, e.Message)
	}
}
