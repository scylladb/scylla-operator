// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (ncpc *Controller) getConfigMaps(ctx context.Context, pod *corev1.Pod) (map[string]*corev1.ConfigMap, error) {
	// List all ConfigMaps to find even those that no longer match our selector.
	// They will be orphaned in ClaimConfigMaps().
	configMaps, err := ncpc.configMapLister.ConfigMaps(pod.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.OwnerUIDLabel:      string(pod.UID),
		naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pods.
	canAdoptFunc := func() error {
		fresh, err := ncpc.kubeClient.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != pod.UID {
			return fmt.Errorf("original Pod %v/%v is gone: got uid %v, wanted %v", pod.Namespace, pod.Name, fresh.UID, pod.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", pod.Namespace, pod.Name, pod.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewConfigMapControllerRefManager(
		ctx,
		pod,
		podControllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealConfigMapControl{
			KubeClient: ncpc.kubeClient,
			Recorder:   ncpc.eventRecorder,
		},
	)
	return cm.ClaimConfigMaps(configMaps)
}

func (ncpc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing Pod", "Pod", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing Pod", "Pod", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	pod, err := ncpc.podLister.Pods(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("Pod has been deleted", "Pod", klog.KObj(pod))
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't list pods: %w", err)
	}

	if pod.DeletionTimestamp != nil {
		return nil
	}

	configMaps, err := ncpc.getConfigMaps(ctx, pod)
	if err != nil {
		return err
	}

	var errs []error

	err = ncpc.syncConfigMaps(ctx, pod, configMaps)
	if err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
