// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/parallel"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaCluster", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sc, err := scc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KObj(sc))
		return nil
	}
	if err != nil {
		return err
	}

	// namespaces, err := scc.getNamespaces(sc)
	// if err != nil {
	// 	return err
	// }

	scyllaDatacenters, err := scc.getScyllaDatacenters(sc)
	if err != nil {
		return err
	}

	status := scc.calculateStatus(sc, scyllaDatacenters)

	if sc.DeletionTimestamp != nil {
		return scc.updateStatus(ctx, sc, status)
	}

	// TODO: orphan existing v1.ScyllaCluster resources

	var errs []error

	// err = scc.syncNamespaces(ctx, sc, namespaces, status)
	// if err != nil {
	// 	errs = append(errs, fmt.Errorf("can't sync remote namespaces: %w", err))
	// 	// TODO: Set degraded condition
	// }

	err = scc.syncScyllaDatacenters(ctx, sc, scyllaDatacenters, status)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote scyllaclusters: %w", err))
		// TODO: Set degraded condition
	}

	err = scc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

func (scc *Controller) getScyllaDatacenters(sc *scyllav2alpha1.ScyllaCluster) (map[string]map[string]*scyllav1alpha1.ScyllaDatacenter, error) {
	multiRegionScyllaDatacenterMap := make(map[string]map[string]*scyllav1alpha1.ScyllaDatacenter, len(sc.Spec.Datacenters))
	var mu sync.Mutex
	err := parallel.ForEach(len(sc.Spec.Datacenters), func(i int) error {
		dc := sc.Spec.Datacenters[i]

		// List all ScyllaClusters matching our selector.
		// Because objects lies in remote cluster, we cannot manage controllerRef and hence control adoption.
		selector := labels.SelectorFromSet(labels.Set{
			naming.ParentClusterNamespaceLabel:      sc.Namespace,
			naming.ParentClusterNameLabel:           sc.Name,
			naming.ParentClusterDatacenterNameLabel: dc.Name,
		})

		scyllaDatacenters, err := scc.scyllaDatacenterMultiregionLister.Datacenter(dc.Name).ScyllaDatacenters(sc.Namespace).List(selector)
		if err != nil {
			return err
		}

		sdMap := make(map[string]*scyllav1alpha1.ScyllaDatacenter, len(scyllaDatacenters))
		for _, sd := range scyllaDatacenters {
			sdMap[sd.Name] = sd
		}

		mu.Lock()
		defer mu.Unlock()
		multiRegionScyllaDatacenterMap[dc.Name] = sdMap

		return nil
	})
	if err != nil {
		return nil, err
	}

	return multiRegionScyllaDatacenterMap, nil
}

// func (scc *Controller) getNamespaces(sc *scyllav2alpha1.ScyllaCluster) (map[string]map[string]*corev1.Namespace, error) {
// 	multiRegionNamespaceMap := make(map[string]map[string]*corev1.Namespace, len(sc.Spec.Datacenters))
// 	var mu sync.Mutex
// 	err := parallel.ForEach(len(sc.Spec.Datacenters), func(i int) error {
// 		dc := sc.Spec.Datacenters[i]
//
// 		if dc.KubeConfigSecretRef != nil {
// 			// List all ScyllaClusters matching our selector. Because objects lies in remote cluster, we cannot manage controllerRef
// 			// TODO(zimnx): figure out solution for above.
// 			selector := labels.SelectorFromSet(labels.Set{
// 				naming.ParentClusterNamespaceLabel:      sc.Namespace,
// 				naming.ParentClusterNameLabel:           sc.Name,
// 				naming.ParentClusterDatacenterNameLabel: dc.Name,
// 			})
//
// 			dcInformer, err := scc.remoteNamespaceInformer.Informer(sc, dc.Name)
// 			if err != nil {
// 				return fmt.Errorf("can't create remote namespace for datacenter %q: %w", dc.Name, err)
// 			}
//
// 			namespaces, err := dcInformer.Lister().List(selector)
// 			if err != nil {
// 				return err
// 			}
//
// 			nsMap := make(map[string]*corev1.Namespace, len(namespaces))
// 			for _, ns := range namespaces {
// 				nsMap[ns.Name] = ns
// 			}
//
// 			mu.Lock()
// 			defer mu.Unlock()
// 			multiRegionNamespaceMap[dc.Name] = nsMap
// 		}
//
// 		return nil
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return multiRegionNamespaceMap, nil
// }
