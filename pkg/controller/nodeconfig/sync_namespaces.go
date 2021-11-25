// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) makeNamespaces() []*corev1.Namespace {
	namespaces := []*corev1.Namespace{
		makeScyllaOperatorNodeTuningNamespace(),
	}

	return namespaces
}

func (ncc *Controller) pruneNamespaces(ctx context.Context, requiredNamespaces []*corev1.Namespace, namespaces map[string]*corev1.Namespace) error {
	var errs []error
	for _, ns := range namespaces {
		if ns.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredNamespaces {
			if ns.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &ns.UID,
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

func (ncc *Controller) syncNamespaces(ctx context.Context, namespaces map[string]*corev1.Namespace) error {
	requiredNamespaces := ncc.makeNamespaces()

	// Delete any excessive Namespaces.
	// Delete has to be the first action to avoid getting stuck on quota.
	err := ncc.pruneNamespaces(ctx, requiredNamespaces, namespaces)
	if err != nil {
		return fmt.Errorf("can't delete Namespace(s): %w", err)
	}

	var errs []error
	for _, ns := range requiredNamespaces {
		_, _, err := resourceapply.ApplyNamespace(ctx, ncc.kubeClient.CoreV1(), ncc.namespaceLister, ncc.eventRecorder, ns, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't create missing Namespace: %w", err))
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}
