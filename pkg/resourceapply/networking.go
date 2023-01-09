// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	networkingv1client "k8s.io/client-go/kubernetes/typed/networking/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyIngressWithControl(
	ctx context.Context,
	control ApplyControlInterface[*networkingv1.Ingress],
	recorder record.EventRecorder,
	required *networkingv1.Ingress,
	options ApplyOptions,
) (*networkingv1.Ingress, bool, error) {
	return ApplyGeneric[*networkingv1.Ingress](ctx, control, recorder, required, options)
}

func ApplyIngress(
	ctx context.Context,
	client networkingv1client.IngressesGetter,
	lister networkingv1listers.IngressLister,
	recorder record.EventRecorder,
	required *networkingv1.Ingress,
	options ApplyOptions,
) (*networkingv1.Ingress, bool, error) {
	return ApplyIngressWithControl(
		ctx,
		ApplyControlFuncs[*networkingv1.Ingress]{
			GetCachedFunc: lister.Ingresses(required.Namespace).Get,
			CreateFunc:    client.Ingresses(required.Namespace).Create,
			UpdateFunc:    client.Ingresses(required.Namespace).Update,
			DeleteFunc:    client.Ingresses(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
