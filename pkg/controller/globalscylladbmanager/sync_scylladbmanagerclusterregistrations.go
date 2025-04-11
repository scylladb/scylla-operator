// Copyright (C) 2025 ScyllaDB

package globalscylladbmanager

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (gsmc *Controller) pruneScyllaDBManagerClusterRegistrations(ctx context.Context, requiredScyllaDBManagerClusterRegistrations []*scyllav1alpha1.ScyllaDBManagerClusterRegistration, scyllaDBManagerClusterRegistrations map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration) error {
	var errs []error

	for _, smcr := range scyllaDBManagerClusterRegistrations {
		if smcr.GetDeletionTimestamp() != nil {
			continue
		}

		isRequired := false
		for _, required := range requiredScyllaDBManagerClusterRegistrations {
			if smcr.GetName() == required.GetName() {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		uid := smcr.GetUID()
		propagationPolicy := metav1.DeletePropagationBackground
		klog.V(2).InfoS("Pruning resource", "GVK", resource.GetObjectGVKOrUnknown(smcr), "Ref", klog.KObj(smcr))
		err := gsmc.scyllaClient.ScyllaDBManagerClusterRegistrations(smcr.Namespace).Delete(ctx, smcr.GetName(), metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &uid,
			},
			PropagationPolicy: &propagationPolicy,
		})
		resourceapply.ReportDeleteEvent(gsmc.EventRecorder(), smcr, err)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (gsmc *Controller) syncScyllaDBManagerClusterRegistrations(ctx context.Context, scyllaDBDatacenters []*scyllav1alpha1.ScyllaDBDatacenter, scyllaDBManagerClusterRegistrations map[string]*scyllav1alpha1.ScyllaDBManagerClusterRegistration) error {
	var requiredScyllaDBManagerClusterRegistrations []*scyllav1alpha1.ScyllaDBManagerClusterRegistration

	globalScyllaDBManagerNamespace, err := gsmc.namespaceLister.Get(naming.ScyllaManagerNamespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("can't get namespace %q: %w", naming.ScyllaManagerNamespace, err)
		}

		klog.V(4).InfoS("Global ScyllaDB Manager namespace does not exist, not creating any ScyllaDBManagerClusterRegistration objects for global ScyllaDB Manager instance.", "Namespace", naming.ScyllaManagerNamespace)
	} else if globalScyllaDBManagerNamespace.DeletionTimestamp != nil {
		klog.V(4).InfoS("Global ScyllaDB Manager namespace is being deleted, not creating any ScyllaDBManagerClusterRegistration objects for global ScyllaDB Manager instance.", "Namespace", naming.ScyllaManagerNamespace)
	} else {
		var errs []error

		for _, sdc := range scyllaDBDatacenters {
			required, err := makeScyllaDBManagerClusterRegistrationForScyllaDBDatacenter(sdc)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't make ScyllaDBManagerClusterRegistration for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err))
				continue
			}
			requiredScyllaDBManagerClusterRegistrations = append(requiredScyllaDBManagerClusterRegistrations, required)
		}

		err = utilerrors.NewAggregate(errs)
		if err != nil {
			return err
		}
	}

	err = gsmc.pruneScyllaDBManagerClusterRegistrations(
		ctx,
		requiredScyllaDBManagerClusterRegistrations,
		scyllaDBManagerClusterRegistrations,
	)
	if err != nil {
		return fmt.Errorf("can't prune ScyllaDBManagerClusterRegistration(s): %w", err)
	}

	var errs []error
	for _, smcr := range requiredScyllaDBManagerClusterRegistrations {
		_, _, err = resourceapply.ApplyScyllaDBManagerClusterRegistration(ctx, gsmc.scyllaClient, gsmc.scyllaDBManagerClusterRegistrationLister, gsmc.EventRecorder(), smcr, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create ScyllaDBManagerClusterRegistration: %w", err))
		}
	}

	return utilerrors.NewAggregate(errs)
}
