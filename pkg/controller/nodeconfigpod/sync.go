// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (ncpc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	key := req.NamespacedName

	namespace, name := key.Namespace, key.Name

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing Pod", "Pod", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing Pod", "Pod", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	pod, err := ctrlclient.Get[corev1.Pod](ctx, ncpc.client, namespace, name)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("Pod has been deleted", "Pod", klog.KObj(pod))
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't list pods: %w", err)
	}

	if !controllerhelpers.IsScyllaPod(pod) {
		klog.Warningf("Non-Scylla Pod %q enqueued for sync by NodeConfigPod controller", klog.KObj(pod))
		return reconcile.Result{}, nil
	}

	if pod.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
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
		ctrlclient.GetObjectsControl[corev1.Pod, corev1.ConfigMap](ctx, ncpc.client, ncpc.apiReader, pod.Namespace),
	)
	if err != nil {
		objectErrs = append(objectErrs, err)
	}

	objectErr := apimachineryutilerrors.NewAggregate(objectErrs)
	if objectErr != nil {
		return reconcile.Result{}, objectErr
	}

	var errs []error

	err = ncpc.syncConfigMaps(ctx, pod, configMaps)
	if err != nil {
		errs = append(errs, err)
	}

	return reconcile.Result{}, apimachineryutilerrors.NewAggregate(errs)
}
