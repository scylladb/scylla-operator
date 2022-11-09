// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/util/parallel"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	sc, err := scc.scyllaClusterLister.ScyllaClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KObj(sc))
		return nil
	}
	if err != nil {
		return err
	}

	namespaces, err := scc.getNamespaces(sc)
	if err != nil {
		return err
	}

	scyllaDatacenters, err := scc.getScyllaDatacenters(sc)
	if err != nil {
		return err
	}

	status := scc.calculateStatus(sc, scyllaDatacenters)

	if sc.DeletionTimestamp != nil {
		return scc.updateStatus(ctx, sc, status)
	}

	var errs []error

	err = runSync(
		&status.Conditions,
		namespaceControllerProgressingCondition,
		namespaceControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncNamespaces(ctx, sc, namespaces)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote namespaces: %w", err))
	}

	err = runSync(
		&status.Conditions,
		scyllaDatacenterControllerProgressingCondition,
		scyllaDatacenterControllerDegradedCondition,
		sc.Generation,
		func() ([]metav1.Condition, error) {
			return scc.syncScyllaDatacenters(ctx, sc, scyllaDatacenters, status)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync remote scylladatacenters: %w", err))
	}

	// Aggregate conditions.

	availableCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav2alpha1.AvailableCondition),
		metav1.Condition{
			Type:               scyllav2alpha1.AvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}

	apimeta.SetStatusCondition(&status.Conditions, availableCondition)

	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav2alpha1.ProgressingCondition),
		metav1.Condition{
			Type:               scyllav2alpha1.ProgressingCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(&status.Conditions, progressingCondition)

	degradedCondition, err := controllerhelpers.AggregateStatusConditions(
		controllerhelpers.FindStatusConditionsWithSuffix(status.Conditions, scyllav2alpha1.DegradedCondition),
		metav1.Condition{
			Type:               scyllav2alpha1.DegradedCondition,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate status conditions: %w", err)
	}
	apimeta.SetStatusCondition(&status.Conditions, degradedCondition)

	err = scc.updateStatus(ctx, sc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

func runSync(conditions *[]metav1.Condition, progressingConditionType, degradedCondType string, observedGeneration int64, syncFn func() ([]metav1.Condition, error)) error {
	progressingConditions, err := syncFn()
	controllerhelpers.SetStatusConditionFromError(conditions, err, degradedCondType, observedGeneration)
	if err != nil {
		return err
	}

	progressingCondition, err := controllerhelpers.AggregateStatusConditions(
		progressingConditions,
		metav1.Condition{
			Type:               progressingConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: observedGeneration,
		},
	)
	if err != nil {
		return fmt.Errorf("can't aggregate progressing conditions %q: %w", progressingConditionType, err)
	}
	apimeta.SetStatusCondition(conditions, progressingCondition)

	return nil
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

		var remoteName string
		namespace := sc.Namespace
		var scyllaDatacenters []*scyllav1alpha1.ScyllaDatacenter

		if dc.RemoteKubeClusterConfigRef != nil {
			remoteName = dc.RemoteKubeClusterConfigRef.Name
			namespace = naming.RemoteNamespace(sc, dc)

			scyllaDatacentersRaw, err := scc.remoteScyllaDatacenterLister.Region(remoteName).ByNamespace(namespace).List(selector)
			if err != nil {
				return err
			}

			for _, obj := range scyllaDatacentersRaw {
				sd := &scyllav1alpha1.ScyllaDatacenter{}
				if err := scheme.Scheme.Convert(obj, sd, nil); err != nil {
					return fmt.Errorf("object returned by lister for scyllav1alpha1.ScyllaDatacenter (%T) cannot be converted to it", obj)
				}
				scyllaDatacenters = append(scyllaDatacenters, sd)
			}
		} else {
			var err error
			scyllaDatacenters, err = scc.scyllaDatacenterLister.ScyllaDatacenters(sc.Namespace).List(selector)
			if err != nil {
				return err
			}
		}

		sdMap := make(map[string]*scyllav1alpha1.ScyllaDatacenter, len(scyllaDatacenters))
		for _, sd := range scyllaDatacenters {
			sdMap[sd.Name] = sd
		}

		mu.Lock()
		defer mu.Unlock()
		multiRegionScyllaDatacenterMap[remoteName] = sdMap

		return nil
	})
	if err != nil {
		return nil, err
	}

	return multiRegionScyllaDatacenterMap, nil
}

func (scc *Controller) getNamespaces(sc *scyllav2alpha1.ScyllaCluster) (map[string]map[string]*corev1.Namespace, error) {
	multiRegionNamespaceMap := make(map[string]map[string]*corev1.Namespace, len(sc.Spec.Datacenters))
	var mu sync.Mutex
	err := parallel.ForEach(len(sc.Spec.Datacenters), func(i int) error {
		dc := sc.Spec.Datacenters[i]

		// Ignore local deployments
		if dc.RemoteKubeClusterConfigRef == nil {
			return nil
		}

		// List all ScyllaClusters matching our selector. Because objects lies in remote cluster, we cannot manage controllerRef
		// TODO(zimnx): figure out solution for above.
		selector := labels.SelectorFromSet(labels.Set{
			naming.ParentClusterNamespaceLabel:      sc.Namespace,
			naming.ParentClusterNameLabel:           sc.Name,
			naming.ParentClusterDatacenterNameLabel: dc.Name,
		})
		regionLister := scc.remoteNamespaceLister.Region(dc.RemoteKubeClusterConfigRef.Name)
		namespacesRaw, err := regionLister.List(selector)
		if err != nil {
			return err
		}

		nsMap := make(map[string]*corev1.Namespace, len(namespacesRaw))
		for _, nsRaw := range namespacesRaw {
			ns := &corev1.Namespace{}
			if err := scheme.Scheme.Convert(nsRaw, ns, nil); err != nil {
				return fmt.Errorf("object returned by lister for corev1.Namespace (%T) cannot be converrted to it", nsRaw)
			}
			nsMap[ns.Name] = ns
		}

		mu.Lock()
		defer mu.Unlock()
		multiRegionNamespaceMap[dc.RemoteKubeClusterConfigRef.Name] = nsMap

		return nil
	})
	if err != nil {
		return nil, err
	}

	return multiRegionNamespaceMap, nil
}
