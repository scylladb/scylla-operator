// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	globalScyllaDBManagerSelector = labels.SelectorFromSet(labels.Set{
		naming.GlobalScyllaDBManagerRegistrationLabel: naming.LabelValueTrue,
	})
)

func (gsmc *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", ControllerName, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", ControllerName, "duration", time.Since(startTime))
	}()

	scyllaDBDatacenters, err := ctrlclient.List[scyllav1alpha1.ScyllaDBDatacenter](ctx, gsmc.client, corev1.NamespaceAll, globalScyllaDBManagerSelector)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't list ScyllaDBDatacenters: %w", err)
	}

	scyllaDBDatacenters = oslices.FilterOut(scyllaDBDatacenters, isObjectBeingDeleted)

	scyllaDBClusters, err := ctrlclient.List[scyllav1alpha1.ScyllaDBCluster](ctx, gsmc.client, corev1.NamespaceAll, globalScyllaDBManagerSelector)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't list ScyllaDBClusters: %w", err)
	}

	scyllaDBClusters = oslices.FilterOut(scyllaDBClusters, isObjectBeingDeleted)

	scyllaDBManagerClusterRegistrations, err := gsmc.getScyllaDBManagerClusterRegistrations(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't list ScyllaDBManagerClusterRegistration objects: %w", err)
	}

	err = gsmc.syncScyllaDBManagerClusterRegistrations(
		ctx,
		scyllaDBDatacenters,
		scyllaDBClusters,
		scyllaDBManagerClusterRegistrations,
	)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't sync ScyllaDBManagerClusterRegistrations: %w", err)
	}

	return reconcile.Result{}, nil
}

func (gsmc *Controller) getScyllaDBManagerClusterRegistrations(ctx context.Context) (map[string]map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	smcrs, err := ctrlclient.List[scyllav1alpha1.ScyllaDBManagerClusterRegistration](ctx, gsmc.client, corev1.NamespaceAll, naming.GlobalScyllaDBManagerClusterRegistrationSelector())
	if err != nil {
		return nil, fmt.Errorf("can't list ScyllaDBManagerClusterRegistrations: %w", err)
	}

	smcrMap := map[string]map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
	for i := range smcrs {
		if _, ok := smcrMap[smcrs[i].Namespace]; !ok {
			smcrMap[smcrs[i].Namespace] = map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
		}

		smcrMap[smcrs[i].Namespace][smcrs[i].Name] = smcrs[i]
	}

	return smcrMap, nil
}

func isObjectBeingDeleted[T metav1.Object](obj T) bool {
	return obj.GetDeletionTimestamp() != nil
}
