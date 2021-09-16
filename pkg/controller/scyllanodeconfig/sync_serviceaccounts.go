// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sncc *Controller) makeServiceAccounts() []*corev1.ServiceAccount {
	serviceAccounts := []*corev1.ServiceAccount{
		resource.ScyllaNodeConfigServiceAccount(),
	}

	return serviceAccounts
}

func (sncc *Controller) pruneServiceAccounts(ctx context.Context, requiredServiceAccounts []*corev1.ServiceAccount, serviceAccounts map[string]*corev1.ServiceAccount) error {
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
		err := sncc.kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Delete(ctx, sa.Name, metav1.DeleteOptions{
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

func (sncc *Controller) syncServiceAccounts(
	ctx context.Context,
	serviceAccounts map[string]*corev1.ServiceAccount,
) error {
	requiredServiceAccounts := sncc.makeServiceAccounts()

	// Delete any excessive ServiceAccounts.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := sncc.pruneServiceAccounts(ctx, requiredServiceAccounts, serviceAccounts); err != nil {
		return fmt.Errorf("can't delete ServiceAccount(s): %w", err)
	}

	var errs []error
	for _, sa := range requiredServiceAccounts {
		_, _, err := resourceapply.ApplyServiceAccount(ctx, sncc.kubeClient.CoreV1(), sncc.serviceAccountLister, sncc.eventRecorder, sa, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing ServiceAccount: %w", err))
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}
