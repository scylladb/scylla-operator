// Copyright (c) 2022 ScyllaDB.

package multiregion

import (
	"context"
	"fmt"
	"time"

	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func DatacenterKey(sc *scyllav2alpha1.ScyllaCluster, datacenter string) string {
	return fmt.Sprintf("%s/%s/%s", sc.Namespace, sc.Name, datacenter)
}

func (scc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing multiregion ScyllaCluster client", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing multiregion ScyllaCluster client", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sc, err := scc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		var errs []error

		for _, dc := range sc.Spec.Datacenters {
			for _, syncHandler := range scc.syncHandlers {
				if err := syncHandler.OnDelete(DatacenterKey(sc, dc.Name)); err != nil {
					klog.ErrorS(err, "Failed to sync multiregion client about ScyllaCluster deletion", "ScyllaCluster", klog.KObj(sc))
					errs = append(errs, err)
				}
			}
		}

		return errors.NewAggregate(errs)
	}
	if err != nil {
		return err
	}

	var errs []error
	for _, dc := range sc.Spec.Datacenters {
		for _, syncHandler := range scc.syncHandlers {
			var config []byte
			if dc.KubeConfigSecretRef != nil {
				kubeConfigSecret, err := scc.secretLister.Secrets(sc.Namespace).Get(dc.KubeConfigSecretRef.Name)
				if err != nil {
					klog.ErrorS(err, "Failed to sync multiregion client about ScyllaCluster update", "ScyllaCluster", klog.KObj(sc), "datacenter", dc.Name)
					errs = append(errs, err)
				}
				config = kubeConfigSecret.Data[naming.KubeConfigSecretKey]
			}

			if err := syncHandler.OnUpdate(DatacenterKey(sc, dc.Name), config); err != nil {
				klog.ErrorS(err, "Failed to sync multiregion client about ScyllaCluster update", "ScyllaCluster", klog.KObj(sc), "datacenter", dc.Name)
				errs = append(errs, err)
			}
		}
	}

	return errors.NewAggregate(errs)
}
