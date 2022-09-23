// Copyright (c) 2022 ScyllaDB.

package protection

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster/multiregion"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/parallel"
	"github.com/scylladb/scylla-operator/pkg/util/slices"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scpc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaCluster protection", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaCluster protection", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sc, err := scpc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted, ignoring", "ScyllaCluster", klog.KObj(sc))
		return nil
	}
	if err != nil {
		return err
	}

	if sc.GetDeletionTimestamp() != nil && slices.ContainsString(naming.ScyllaClusterFinalizer, sc.GetFinalizers()) {
		isUsed, err := scpc.isBeingUsed(ctx, sc)
		if err != nil {
			return err
		}

		if !isUsed {
			return scpc.removeFinalizer(ctx, sc)
		}
		klog.V(2).InfoS("Keeping ScyllaCluster because it's being used", "ScyllaCluster", klog.KObj(sc))
	}

	if sc.GetDeletionTimestamp() == nil && !slices.ContainsString(naming.ScyllaClusterFinalizer, sc.GetFinalizers()) {
		return scpc.addFinalizer(ctx, sc)
	}

	return nil
}

func (scpc *Controller) addFinalizer(ctx context.Context, sc *scyllav2alpha1.ScyllaCluster) error {
	scCopy := sc.DeepCopy()
	scCopy.ObjectMeta.Finalizers = append(scCopy.ObjectMeta.Finalizers, naming.ScyllaClusterFinalizer)

	_, err := scpc.scyllaClient.ScyllaClusters(scCopy.Namespace).Update(ctx, scCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.ErrorS(err, "Error adding protection finalizer to ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
		return err
	}

	klog.V(2).InfoS("Added protection finalizer to ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	return nil
}

func (scpc *Controller) removeFinalizer(ctx context.Context, sc *scyllav2alpha1.ScyllaCluster) error {
	scCopy := sc.DeepCopy()
	scCopy.ObjectMeta.Finalizers = slices.RemoveString(naming.ScyllaClusterFinalizer, scCopy.ObjectMeta.Finalizers)

	_, err := scpc.scyllaClient.ScyllaClusters(scCopy.Namespace).Update(ctx, scCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.ErrorS(err, "Error removing protection finalizer from ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
		return err
	}

	klog.V(2).InfoS("Removed protection finalizer from ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	return nil
}

func (scpc *Controller) isBeingUsed(ctx context.Context, sc *scyllav2alpha1.ScyllaCluster) (bool, error) {
	for _, dc := range sc.Spec.Datacenters {
		lister := scpc.scyllaDatacenterMultiregionInformer.Lister()

		remoteDCs, err := lister.Datacenter(multiregion.DatacenterKey(sc, dc.Name)).ScyllaDatacenters(sc.Namespace).List(naming.ParentClusterSelector(sc))
		if err != nil {
			return false, fmt.Errorf("list ScyllaDatacenter in %q ScyllaCluster %q datacenter: %w", naming.ObjRef(sc), dc.Name, err)
		}

		klog.V(4).InfoS("Listed ScyllaDatacenters using Informer", "ScyllaCluster", klog.KObj(sc), "datacenter", dc.Name, "results", len(remoteDCs), "selector", naming.ParentClusterSelector(sc).String())

		if len(remoteDCs) > 0 {
			return true, nil
		}
	}

	klog.V(4).InfoS("No remote ScyllaDatacenter found in the Informer cache", "ScyllaCluster", klog.KObj(sc))

	// Live list ScyllaDatacenter to be 100% sure before we delete the parent. Informer cache might not be updated yet.
	var used atomic.Value
	used.Store(false)

	err := parallel.ForEach(len(sc.Spec.Datacenters), func(i int) error {
		dc := sc.Spec.Datacenters[i]

		dcClient, err := scpc.scyllaMultiregionClient.Datacenter(multiregion.DatacenterKey(sc, dc.Name))
		if err != nil {
			return fmt.Errorf("get %q ScyllaCluster %q datacenter client: %w", naming.ObjRef(sc), dc.Name, err)
		}

		remoteDCs, err := dcClient.ScyllaV1alpha1().ScyllaDatacenters(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: naming.ParentClusterSelector(sc).String(),
		})
		if err != nil {
			return fmt.Errorf("list ScyllaDatacenter in %q ScyllaCluster %q datacenter: %w", naming.ObjRef(sc), dc.Name, err)
		}

		klog.V(4).InfoS("Listed ScyllaDatacenters live call", "ScyllaCluster", klog.KObj(sc), "datacenter", dc.Name, "results", len(remoteDCs.Items), "selector", naming.ParentClusterSelector(sc).String())

		if len(remoteDCs.Items) > 0 {
			used.Store(true)
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	klog.V(2).InfoS("ScyllaCluster doesn't have any remote child", "ScyllaCluster", klog.KObj(sc))

	return false, nil
}
