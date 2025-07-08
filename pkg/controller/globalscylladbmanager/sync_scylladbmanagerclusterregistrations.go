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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (gsmc *Controller) syncScyllaDBManagerClusterRegistrations(
	ctx context.Context,
	scyllaDBDatacenters []*scyllav1alpha1.ScyllaDBDatacenter,
	scyllaDBClusters []*scyllav1alpha1.ScyllaDBCluster,
	scyllaDBManagerClusterRegistrations map[string]map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration,
) error {
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
		var requiredScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration
		requiredScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters, err = makeScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters(scyllaDBDatacenters)
		if err != nil {
			return fmt.Errorf("can't make required ScyllaDBManagerClusterRegistration objects for ScyllaDBDatacenter(s): %w", err)
		}

		maps.Copy(requiredScyllaDBManagerClusterRegistrations, requiredScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters)

		var requiredScyllaDBManagerClusterRegistrationsForScyllaDBClusters map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration
		requiredScyllaDBManagerClusterRegistrationsForScyllaDBClusters, err = makeScyllaDBManagerClusterRegistrationsForScyllaDBClusters(scyllaDBClusters)
		if err != nil {
			return fmt.Errorf("can't make required ScyllaDBManagerClusterRegistration objects for ScyllaDBClusters(s): %w", err)
		}

		maps.Copy(requiredScyllaDBManagerClusterRegistrations, requiredScyllaDBManagerClusterRegistrationsForScyllaDBClusters)
	}

	for ns, existing := range scyllaDBManagerClusterRegistrations {
		err = controllerhelpers.Prune(
			ctx,
			requiredScyllaDBManagerClusterRegistrations[ns],
			existing,
			&controllerhelpers.PruneControlFuncs{
				DeleteFunc: gsmc.scyllaClient.ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(ns).Delete,
			},
			gsmc.EventRecorder(),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't prune ScyllaDBManagerClusterRegistration(s) in Namespace %q: %w", ns, err))
		}
	}
	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return err
	}

	for _, smcr := range slices.Concat(slices.Collect(maps.Values(requiredScyllaDBManagerClusterRegistrations))...) {
		_, _, err = resourceapply.ApplyScyllaDBManagerClusterRegistration(ctx, gsmc.scyllaClient.ScyllaV1alpha1(), gsmc.scyllaDBManagerClusterRegistrationLister, gsmc.EventRecorder(), smcr, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply ScyllaDBManagerClusterRegistration: %w", err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func makeScyllaDBManagerClusterRegistrationsForScyllaDBDatacenters(sdcs []*scyllav1alpha1.ScyllaDBDatacenter) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	return makeScyllaDBManagerClusterRegistrationsForObjects(sdcs, makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter)
}

func makeScyllaDBManagerClusterRegistrationsForScyllaDBClusters(scs []*scyllav1alpha1.ScyllaDBCluster) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	return makeScyllaDBManagerClusterRegistrationsForObjects(scs, makeScyllaDBManagerClusterRegistrationForScyllaDBCluster)
}

func makeScyllaDBManagerClusterRegistrationsForObjects[S ~[]E, E metav1.Object](
	objs S,
	makeScyllaDBManagerClusterRegistrationForObject func(E) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error),
) (map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration, error) {
	var errs []error
	requiredScyllaDBManagerClusterRegistrations := map[string][]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{}
	for _, obj := range objs {
		required, err := makeScyllaDBManagerClusterRegistrationForObject(obj)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't make ScyllaDBManagerClusterRegistration for object %q: %w", naming.ObjRef(obj), err))
			continue
		}

		requiredScyllaDBManagerClusterRegistrations[obj.GetNamespace()] = append(requiredScyllaDBManagerClusterRegistrations[obj.GetNamespace()], required)
	}

	return requiredScyllaDBManagerClusterRegistrations, apimachineryutilerrors.NewAggregate(errs)
}
