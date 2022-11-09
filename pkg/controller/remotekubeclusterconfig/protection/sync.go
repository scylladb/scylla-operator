// Copyright (c) 2022 ScyllaDB.

package protection

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (rkccpc *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing RemoteKubeClusterConfig protection", "RemoteKubeClusterConfig", name, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing RemoteKubeClusterConfig protection", "RemoteKubeClusterConfig", name, "duration", time.Since(startTime))
	}()

	rkcc, err := rkccpc.remoteKubeClusterConfigLister.Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("RemoteKubeClusterConfig has been deleted, ignoring", "RemoteKubeClusterConfig", klog.KObj(rkcc))
		return nil
	}
	if err != nil {
		return err
	}

	if rkcc.GetDeletionTimestamp() != nil && slices.ContainsString(naming.RemoteKubeClusterConfigFinalizer, rkcc.GetFinalizers()) {
		isUsed, err := rkccpc.isBeingUsed(ctx, rkcc)
		if err != nil {
			return err
		}

		if !isUsed {
			return rkccpc.removeFinalizer(ctx, rkcc)
		}
		klog.V(2).InfoS("Keeping RemoteKubeClusterConfig because it's being used", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	}

	if rkcc.GetDeletionTimestamp() == nil && !slices.ContainsString(naming.RemoteKubeClusterConfigFinalizer, rkcc.GetFinalizers()) {
		return rkccpc.addFinalizer(ctx, rkcc)
	}

	klog.V(4).InfoS("Nothing to do", "RemoteKubeClusterConfig", klog.KObj(rkcc))

	return nil
}

func (rkccpc *Controller) addFinalizer(ctx context.Context, rkcc *scyllav1alpha1.RemoteKubeClusterConfig) error {
	patch, err := controllerhelpers.AddFinalizerPatch(rkcc, naming.RemoteKubeClusterConfigFinalizer)
	if err != nil {
		return fmt.Errorf("can't create add finalizer patch: %w", err)
	}

	_, err = rkccpc.scyllaClient.ScyllaV1alpha1().RemoteKubeClusterConfigs().Patch(ctx, rkcc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		klog.ErrorS(err, "Error adding protection finalizer to RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
		return err
	}

	klog.V(2).InfoS("Added protection finalizer to RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	return nil
}

func (rkccpc *Controller) removeFinalizer(ctx context.Context, rkcc *scyllav1alpha1.RemoteKubeClusterConfig) error {
	patch, err := controllerhelpers.RemoveFinalizerPatch(rkcc, naming.RemoteKubeClusterConfigFinalizer)
	if err != nil {
		return fmt.Errorf("can't create remove finalizer patch: %w", err)
	}

	_, err = rkccpc.scyllaClient.ScyllaV1alpha1().RemoteKubeClusterConfigs().Patch(ctx, rkcc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		klog.ErrorS(err, "Error removing protection finalizer from RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
		return err
	}

	klog.V(2).InfoS("Removed protection finalizer from RemoteKubeClusterConfig", "RemoteKubeClusterConfig", klog.KObj(rkcc))
	return nil
}

func (rkccpc *Controller) isBeingUsed(ctx context.Context, rkcc *scyllav1alpha1.RemoteKubeClusterConfig) (bool, error) {
	scs, err := rkccpc.scyllaClusterLister.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("list all ScyllaClusters using lister: %w", err)
	}

	var scyllaClusterReferents []string
	for _, sc := range scs {
		for _, dc := range sc.Spec.Datacenters {
			if dc.RemoteKubeClusterConfigRef != nil && dc.RemoteKubeClusterConfigRef.Name == rkcc.Name {
				scyllaClusterReferents = append(scyllaClusterReferents, naming.ObjRef(sc))
			}
		}
	}

	if len(scyllaClusterReferents) != 0 {
		klog.V(4).InfoS("Listed ScyllaClusters using Informer and found ScyllaCluster's referencing it", "RemoteKubeClusterConfig", klog.KObj(rkcc), "ScyllaClusters", scyllaClusterReferents)
		return true, nil
	}

	klog.V(4).InfoS("No ScyllaClusters referencing RemoteKubeClusterConfig found in the Informer cache", "RemoteKubeClusterConfig", klog.KObj(rkcc))

	// Live list ScyllaClusters to be 100% sure before we delete. Informer cache might not be updated yet.
	scList, err := rkccpc.scyllaClient.ScyllaV2alpha1().ScyllaClusters(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})
	if err != nil {
		return false, fmt.Errorf("list all ScyllaClusters using lister: %w", err)
	}

	scyllaClusterReferents = scyllaClusterReferents[:0]
	for _, sc := range scList.Items {
		for _, dc := range sc.Spec.Datacenters {
			if dc.RemoteKubeClusterConfigRef != nil && dc.RemoteKubeClusterConfigRef.Name == rkcc.Name {
				scyllaClusterReferents = append(scyllaClusterReferents, naming.ObjRef(&sc))
			}
		}
	}

	if len(scyllaClusterReferents) != 0 {
		klog.V(4).InfoS("Listed ScyllaClusters using live call and found ScyllaCluster's referencing it", "RemoteKubeClusterConfig", klog.KObj(rkcc), "ScyllaClusters", scyllaClusterReferents)
		return true, nil
	}

	klog.V(2).InfoS("RemoteKubeClusterConfig doesn't have any referents", "RemoteKubeClusterConfig", klog.KObj(rkcc))

	return false, nil
}
