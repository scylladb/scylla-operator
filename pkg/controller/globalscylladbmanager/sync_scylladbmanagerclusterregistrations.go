// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"context"
	"fmt"
	"maps"
	"slices"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (gsmc *Controller) syncScyllaDBManagerClusterRegistrations(ctx context.Context, scyllaDBDatacenters []*scyllav1alpha1.ScyllaDBDatacenter, scyllaDBManagerClusterRegistrations map[string]map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration) error {
	var errs []error
	requiredScyllaDBManagerClusterRegistrations := map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}

	globalScyllaDBManagerNamespace, err := gsmc.namespaceLister.Get(naming.ScyllaManagerNamespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("can't get namespace %q: %w", naming.ScyllaManagerNamespace, err)
		}

		klog.V(4).InfoS("Global ScyllaDB Manager namespace does not exist, not creating any ScyllaDBManagerClusterRegistration objects for global ScyllaDB Manager instance.", "Namespace", naming.ScyllaManagerNamespace)
	} else if globalScyllaDBManagerNamespace.DeletionTimestamp != nil {
		klog.V(4).InfoS("Global ScyllaDB Manager namespace is being deleted, not creating any ScyllaDBManagerClusterRegistration objects for global ScyllaDB Manager instance.", "Namespace", naming.ScyllaManagerNamespace)
	} else {
		for _, sdc := range scyllaDBDatacenters {
			required, err := makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter(sdc)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't make ScyllaDBManagerClusterRegistration for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err))
				continue
			}

			requiredScyllaDBManagerClusterRegistrations[sdc.Namespace] = append(requiredScyllaDBManagerClusterRegistrations[sdc.Namespace], required)
		}

		err = utilerrors.NewAggregate(errs)
		if err != nil {
			return err
		}
	}

	for ns, existing := range scyllaDBManagerClusterRegistrations {
		err = controllerhelpers.Prune(
			ctx,
			requiredScyllaDBManagerClusterRegistrations[ns],
			existing,
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: gsmc.scyllaClient.ScyllaDBManagerClusterRegistrations(ns).Delete,
			},
			gsmc.EventRecorder(),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't prune ScyllaDBManagerClusterRegistration(s) in Namespace %q: %w", ns, err))
		}
	}
	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return err
	}

	for _, smcr := range slices.Concat(slices.Collect(maps.Values(requiredScyllaDBManagerClusterRegistrations))...) {
		_, _, err = resourceapply.ApplyScyllaDBManagerClusterRegistration(ctx, gsmc.scyllaClient, gsmc.scyllaDBManagerClusterRegistrationLister, gsmc.EventRecorder(), smcr, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply ScyllaDBManagerClusterRegistration: %w", err))
		}
	}

	return utilerrors.NewAggregate(errs)
}
