// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
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

	scyllaDBDatacenters, err := gsmc.scyllaDBDatacenterLister.List(globalScyllaDBManagerSelector)
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBDatacenters: %w", err)
	}

	scyllaDBManagerClusterRegistrations, err := gsmc.getScyllaDBManagerClusterRegistrations()
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBManagerClusterRegistration objects: %w", err)
	}

	err = gsmc.syncScyllaDBManagerClusterRegistrations(
		ctx,
		scyllaDBDatacenters,
		scyllaDBManagerClusterRegistrations,
	)
	if err != nil {
		return fmt.Errorf("can't sync ScyllaDBManagerClusterRegistrations: %w", err)
	}

	return nil
}

func (gsmc *Controller) getScyllaDBManagerClusterRegistrations() (map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	smcrs, err := gsmc.scyllaDBManagerClusterRegistrationLister.List(labels.SelectorFromSet(labels.Set{
		naming.GlobalScyllaDBManagerLabel: naming.LabelValueTrue,
	}))
	if err != nil {
		return nil, fmt.Errorf("can't list ScyllaDBManagerClusterRegistrations: %w", err)
	}

	smcrMap := map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
	for i := range smcrs {
		smcrMap[smcrs[i].Name] = smcrs[i]
	}

	return smcrMap, nil
}
