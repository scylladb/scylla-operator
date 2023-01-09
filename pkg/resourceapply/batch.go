package resourceapply

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyJobWithControl(
	ctx context.Context,
	control ApplyControlInterface[*batchv1.Job],
	recorder record.EventRecorder,
	required *batchv1.Job,
	options ApplyOptions,
) (*batchv1.Job, bool, error) {
	return ApplyGeneric[*batchv1.Job](ctx, control, recorder, required, options)
}

func ApplyJob(
	ctx context.Context,
	client batchv1client.JobsGetter,
	lister batchv1listers.JobLister,
	recorder record.EventRecorder,
	required *batchv1.Job,
	options ApplyOptions,
) (*batchv1.Job, bool, error) {
	return ApplyJobWithControl(
		ctx,
		ApplyControlFuncs[*batchv1.Job]{
			GetCachedFunc: lister.Jobs(required.Namespace).Get,
			CreateFunc:    client.Jobs(required.Namespace).Create,
			UpdateFunc:    client.Jobs(required.Namespace).Update,
			DeleteFunc:    client.Jobs(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
