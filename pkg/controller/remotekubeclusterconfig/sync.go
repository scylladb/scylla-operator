// Copyright (c) 2022 ScyllaDB.

package remotekubeclusterconfig

import (
	"context"
	"time"

	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scc *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing multiregion client", "RemoteKubeClusterConfig", name, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing multiregion client", "RemoteKubeClusterConfig", name, "duration", time.Since(startTime))
	}()

	rkcc, err := scc.remoteKubeClusterConfigLister.Get(name)
	if apierrors.IsNotFound(err) {
		var errs []error

		for _, syncHandler := range scc.syncHandlers {
			syncHandler.Delete(name)
		}

		return errors.NewAggregate(errs)
	}
	if err != nil {
		return err
	}

	if rkcc.DeletionTimestamp != nil {
		return nil
	}

	var errs []error

	for _, syncHandler := range scc.syncHandlers {
		var config []byte
		if rkcc.Spec.KubeConfigSecretRef == nil {
			continue
		}
		kubeConfigSecret, err := scc.secretLister.Secrets(rkcc.Spec.KubeConfigSecretRef.Namespace).Get(rkcc.Spec.KubeConfigSecretRef.Name)
		if err != nil {
			klog.ErrorS(err, "Failed to fetch secret", "RemoteKubeClusterConfig", klog.KObj(rkcc))
			errs = append(errs, err)
			continue
		}
		config = kubeConfigSecret.Data[naming.KubeConfigSecretKey]

		if err := syncHandler.Update(rkcc.Name, config); err != nil {
			klog.ErrorS(err, "Failed to sync multiregion client about RemoteKubeClusterConfig update", "RemoteKubeClusterConfig", klog.KObj(rkcc))
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}
