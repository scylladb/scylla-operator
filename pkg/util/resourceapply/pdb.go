// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"context"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyPodDisruptionBudget creates or updates PDB. If desired object is equal (based on calculated hash)
// to existing one update is not called.
func ApplyPodDisruptionBudget(ctx context.Context, recorder record.EventRecorder, client client.Client, pdb *v1beta1.PodDisruptionBudget) error {
	required := pdb.DeepCopy()
	if err := setHash(required); err != nil {
		return errors.Wrap(err, "set hash on pod disruption budget")
	}

	existing := &v1beta1.PodDisruptionBudget{}
	err := client.Get(ctx, naming.NamespacedName(required.Name, required.Namespace), existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "get pod disruption budget")
		}

		err := client.Create(ctx, required)
		if err != nil {
			ReportCreateEvent(recorder, required, err)
			return errors.Wrap(err, "create pod disruption budget")
		}

		ReportCreateEvent(recorder, required, nil)
		return nil
	}

	// If they are the same do nothing.
	if existing.Annotations[naming.ManagedHash] == required.Annotations[naming.ManagedHash] {
		return nil
	}

	required.ResourceVersion = existing.ResourceVersion
	if err := client.Update(ctx, required); err != nil {
		ReportUpdateEvent(recorder, pdb, err)
		return errors.Wrap(err, "update pod disruption budget")
	}

	ReportUpdateEvent(recorder, pdb, nil)
	return nil
}
