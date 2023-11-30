// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	discoveryv1client "k8s.io/client-go/kubernetes/typed/discovery/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyEndpointSliceWithControl(
	ctx context.Context,
	control ApplyControlInterface[*discoveryv1.EndpointSlice],
	recorder record.EventRecorder,
	required *discoveryv1.EndpointSlice,
	options ApplyOptions,
) (*discoveryv1.EndpointSlice, bool, error) {
	return ApplyGeneric[*discoveryv1.EndpointSlice](ctx, control, recorder, required, options)
}

func ApplyEndpointSlice(
	ctx context.Context,
	client discoveryv1client.EndpointSlicesGetter,
	lister discoveryv1listers.EndpointSliceLister,
	recorder record.EventRecorder,
	required *discoveryv1.EndpointSlice,
	options ApplyOptions,
) (*discoveryv1.EndpointSlice, bool, error) {
	return ApplyEndpointSliceWithControl(
		ctx,
		ApplyControlFuncs[*discoveryv1.EndpointSlice]{
			GetCachedFunc: lister.EndpointSlices(required.Namespace).Get,
			CreateFunc:    client.EndpointSlices(required.Namespace).Create,
			UpdateFunc:    client.EndpointSlices(required.Namespace).Update,
			DeleteFunc:    client.EndpointSlices(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
