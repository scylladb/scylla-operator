// Copyright (C) 2021 ScyllaDB

package resourceapply

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	policyv1client "k8s.io/client-go/kubernetes/typed/policy/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyPodDisruptionBudgetWithControl(
	ctx context.Context,
	control ApplyControlInterface[*policyv1.PodDisruptionBudget],
	recorder record.EventRecorder,
	required *policyv1.PodDisruptionBudget,
	options ApplyOptions,
) (*policyv1.PodDisruptionBudget, bool, error) {
	return ApplyGeneric[*policyv1.PodDisruptionBudget](ctx, control, recorder, required, options)
}

func ApplyPodDisruptionBudget(
	ctx context.Context,
	client policyv1client.PodDisruptionBudgetsGetter,
	lister policyv1listers.PodDisruptionBudgetLister,
	recorder record.EventRecorder,
	required *policyv1.PodDisruptionBudget,
	options ApplyOptions,
) (*policyv1.PodDisruptionBudget, bool, error) {
	return ApplyPodDisruptionBudgetWithControl(
		ctx,
		ApplyControlFuncs[*policyv1.PodDisruptionBudget]{
			GetCachedFunc: lister.PodDisruptionBudgets(required.Namespace).Get,
			CreateFunc:    client.PodDisruptionBudgets(required.Namespace).Create,
			UpdateFunc:    client.PodDisruptionBudgets(required.Namespace).Update,
			DeleteFunc:    client.PodDisruptionBudgets(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
