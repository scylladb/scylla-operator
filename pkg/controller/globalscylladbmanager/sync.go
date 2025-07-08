// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

var (
	globalScyllaDBManagerSelector = labels.SelectorFromSet(labels.Set{
		naming.GlobalScyllaDBManagerRegistrationLabel: naming.LabelValueTrue,
	})
)

func (gsmc *Controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", gsmc.Observer.Name(), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", gsmc.Observer.Name(), "duration", time.Since(startTime))
	}()

	scyllaDBDatacenters, err := gsmc.scyllaDBDatacenterLister.ScyllaDBDatacenters(corev1.NamespaceAll).List(globalScyllaDBManagerSelector)
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBDatacenters: %w", err)
	}

	scyllaDBDatacenters = oslices.FilterOut(scyllaDBDatacenters, isObjectBeingDeleted)

	scyllaDBClusters, err := gsmc.scyllaDBClusterLister.ScyllaDBClusters(corev1.NamespaceAll).List(globalScyllaDBManagerSelector)
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBClusters: %w", err)
	}

	scyllaDBClusters = oslices.FilterOut(scyllaDBClusters, isObjectBeingDeleted)

	scyllaDBManagerClusterRegistrations, err := gsmc.getScyllaDBManagerClusterRegistrations()
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBManagerClusterRegistration objects: %w", err)
	}

	err = gsmc.syncScyllaDBManagerClusterRegistrations(
		ctx,
		scyllaDBDatacenters,
		scyllaDBClusters,
		scyllaDBManagerClusterRegistrations,
	)
	if err != nil {
		return fmt.Errorf("can't sync ScyllaDBManagerClusterRegistrations: %w", err)
	}

	return nil
}

func (gsmc *Controller) getScyllaDBManagerClusterRegistrations() (map[string]map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	smcrs, err := gsmc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(corev1.NamespaceAll).List(naming.GlobalScyllaDBManagerClusterRegistrationSelector())
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
