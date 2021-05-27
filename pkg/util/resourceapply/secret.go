// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"context"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplySecret creates or updates Secret. If desired object is equal (based on calculated hash)
// to existing one update is not called.
func ApplySecret(ctx context.Context, recorder record.EventRecorder, client client.Client, secret *corev1.Secret) error {
	required := secret.DeepCopy()
	if err := setHash(required); err != nil {
		return errors.Wrap(err, "set hash on secret")
	}

	existing := &corev1.Secret{}
	err := client.Get(ctx, naming.NamespacedName(required.Name, required.Namespace), existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "get secret")
		}

		err := client.Create(ctx, required)
		if err != nil {
			ReportCreateEvent(recorder, required, err)
			return errors.Wrap(err, "create secret")
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
		ReportUpdateEvent(recorder, secret, err)
		return errors.Wrap(err, "update secret")
	}

	ReportUpdateEvent(recorder, secret, nil)
	return nil
}
