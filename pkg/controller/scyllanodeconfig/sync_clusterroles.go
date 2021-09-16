// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sncc *Controller) makeClusterRoles() []*rbacv1.ClusterRole {
	clusterRoles := []*rbacv1.ClusterRole{
		resource.ScyllaNodeConfigClusterRole(),
	}

	return clusterRoles
}

func (sncc *Controller) pruneClusterRoles(ctx context.Context, requiredClusterRoles []*rbacv1.ClusterRole, clusterRoles map[string]*rbacv1.ClusterRole) error {
	var errs []error
	for _, cr := range clusterRoles {
		if cr.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredClusterRoles {
			if cr.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := sncc.kubeClient.RbacV1().ClusterRoles().Delete(ctx, cr.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &cr.UID,
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

func (sncc *Controller) syncClusterRoles(ctx context.Context, clusterRoles map[string]*rbacv1.ClusterRole) error {
	requiredClusterRoles := sncc.makeClusterRoles()

	// Delete any excessive ClusterRoles.
	// Delete has to be the first action to avoid getting stuck on quota.
	if err := sncc.pruneClusterRoles(ctx, requiredClusterRoles, clusterRoles); err != nil {
		return fmt.Errorf("can't delete ClusterRole(s): %w", err)
	}

	var errs []error
	for _, cr := range requiredClusterRoles {
		_, _, err := resourceapply.ApplyClusterRole(ctx, sncc.kubeClient.RbacV1(), sncc.clusterRoleLister, sncc.eventRecorder, cr, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing clusterrole: %w", err))
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}
