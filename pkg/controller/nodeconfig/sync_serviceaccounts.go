// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) makeServiceAccounts() []*corev1.ServiceAccount {
	serviceAccounts := []*corev1.ServiceAccount{
		makeNodeConfigServiceAccount(),
		makePerftuneServiceAccount(),
	}

	return serviceAccounts
}

func (ncc *Controller) pruneServiceAccounts(ctx context.Context, requiredServiceAccounts []*corev1.ServiceAccount, serviceAccounts map[string]*corev1.ServiceAccount) error {
	var errs []error
	for _, sa := range serviceAccounts {
		if sa.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredServiceAccounts {
			if sa.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Delete(ctx, sa.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sa.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncServiceAccounts(ctx context.Context, nc *scyllav1alpha1.NodeConfig, serviceAccounts map[string]*corev1.ServiceAccount) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	requiredServiceAccounts := ncc.makeServiceAccounts()

	// Delete any excessive ServiceAccounts.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := ncc.pruneServiceAccounts(ctx, requiredServiceAccounts, serviceAccounts); err != nil {
		return progressingConditions, fmt.Errorf("can't delete ServiceAccount(s): %w", err)
	}

	var errs []error
	for _, sa := range requiredServiceAccounts {
		_, changed, err := resourceapply.ApplyServiceAccount(ctx, ncc.kubeClient.CoreV1(), ncc.serviceAccountLister, ncc.eventRecorder, sa, resourceapply.ApplyOptions{
			AllowMissingControllerRef: true,
		})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceAccountControllerProgressingCondition, sa, "apply", nc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing ServiceAccount: %w", err))
			continue
		}
	}

	return progressingConditions, utilerrors.NewAggregate(errs)
}
