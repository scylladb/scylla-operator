// Copyright (c) 2024 ScyllaDB.

package resourceapply

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"k8s.io/client-go/tools/record"
)

func ApplyScyllaDBDatacenterWithControl(
	ctx context.Context,
	control ApplyControlInterface[*scyllav1alpha1.ScyllaDBDatacenter],
	recorder record.EventRecorder,
	required *scyllav1alpha1.ScyllaDBDatacenter,
	options ApplyOptions,
) (*scyllav1alpha1.ScyllaDBDatacenter, bool, error) {
	return ApplyGeneric[*scyllav1alpha1.ScyllaDBDatacenter](ctx, control, recorder, required, options)
}

func ApplyScyllaDBDatacenter(
	ctx context.Context,
	client scyllav1alpha1client.ScyllaDBDatacentersGetter,
	lister scyllav1alpha1listers.ScyllaDBDatacenterLister,
	recorder record.EventRecorder,
	required *scyllav1alpha1.ScyllaDBDatacenter,
	options ApplyOptions,
) (*scyllav1alpha1.ScyllaDBDatacenter, bool, error) {
	return ApplyScyllaDBDatacenterWithControl(
		ctx,
		ApplyControlFuncs[*scyllav1alpha1.ScyllaDBDatacenter]{
			GetCachedFunc: lister.ScyllaDBDatacenters(required.Namespace).Get,
			CreateFunc:    client.ScyllaDBDatacenters(required.Namespace).Create,
			UpdateFunc:    client.ScyllaDBDatacenters(required.Namespace).Update,
			DeleteFunc:    client.ScyllaDBDatacenters(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyRemoteOwnerWithControl(
	ctx context.Context,
	control ApplyControlInterface[*scyllav1alpha1.RemoteOwner],
	recorder record.EventRecorder,
	required *scyllav1alpha1.RemoteOwner,
	options ApplyOptions,
) (*scyllav1alpha1.RemoteOwner, bool, error) {
	return ApplyGeneric[*scyllav1alpha1.RemoteOwner](ctx, control, recorder, required, options)
}

func ApplyRemoteOwner(
	ctx context.Context,
	client scyllav1alpha1client.RemoteOwnersGetter,
	lister scyllav1alpha1listers.RemoteOwnerLister,
	recorder record.EventRecorder,
	required *scyllav1alpha1.RemoteOwner,
	options ApplyOptions,
) (*scyllav1alpha1.RemoteOwner, bool, error) {
	return ApplyRemoteOwnerWithControl(
		ctx,
		ApplyControlFuncs[*scyllav1alpha1.RemoteOwner]{
			GetCachedFunc: lister.RemoteOwners(required.Namespace).Get,
			CreateFunc:    client.RemoteOwners(required.Namespace).Create,
			UpdateFunc:    client.RemoteOwners(required.Namespace).Update,
			DeleteFunc:    client.RemoteOwners(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyScyllaDBManagerClusterRegistrationWithControl(
	ctx context.Context,
	control ApplyControlInterface[*scyllav1alpha1.ScyllaDBManagerClusterRegistration],
	recorder record.EventRecorder,
	required *scyllav1alpha1.ScyllaDBManagerClusterRegistration,
	options ApplyOptions,
) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, bool, error) {
	return ApplyGeneric[*scyllav1alpha1.ScyllaDBManagerClusterRegistration](ctx, control, recorder, required, options)
}

func ApplyScyllaDBManagerClusterRegistration(
	ctx context.Context,
	client scyllav1alpha1client.ScyllaDBManagerClusterRegistrationsGetter,
	lister scyllav1alpha1listers.ScyllaDBManagerClusterRegistrationLister,
	recorder record.EventRecorder,
	required *scyllav1alpha1.ScyllaDBManagerClusterRegistration,
	options ApplyOptions,
) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, bool, error) {
	return ApplyScyllaDBManagerClusterRegistrationWithControl(
		ctx,
		ApplyControlFuncs[*scyllav1alpha1.ScyllaDBManagerClusterRegistration]{
			GetCachedFunc: lister.ScyllaDBManagerClusterRegistrations(required.Namespace).Get,
			CreateFunc:    client.ScyllaDBManagerClusterRegistrations(required.Namespace).Create,
			UpdateFunc:    client.ScyllaDBManagerClusterRegistrations(required.Namespace).Update,
			DeleteFunc:    client.ScyllaDBManagerClusterRegistrations(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
