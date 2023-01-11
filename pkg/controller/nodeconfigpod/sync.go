// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

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

	if !controllerhelpers.IsScyllaPod(pod) {
		klog.Warningf("Non-Scylla Pod %q enqueued for sync by NodeConfigPod controller", klog.KObj(pod))
		return nil
	}

	if pod.DeletionTimestamp != nil {
		return nil
	}

	podSelector := labels.SelectorFromSet(labels.Set{
		naming.OwnerUIDLabel:      string(pod.UID),
		naming.ConfigMapTypeLabel: string(naming.NodeConfigDataConfigMapType),
	})

	type CT = *corev1.Pod
	var objectErrs []error

	configMaps, err := controllerhelpers.GetObjects[CT, *corev1.ConfigMap](
		ctx,
		pod,
		podControllerGVK,
		podSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ConfigMap]{
			GetControllerUncachedFunc: ncpc.kubeClient.CoreV1().Pods(pod.Namespace).Get,
			ListObjectsFunc:           ncpc.configMapLister.ConfigMaps(pod.Namespace).List,
			PatchObjectFunc:           ncpc.kubeClient.CoreV1().ConfigMaps(pod.Namespace).Patch,
		},
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	objectErr := utilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return objectErr
	}

	var errs []error

	err = ncpc.syncConfigMaps(ctx, pod, configMaps)
	if err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
