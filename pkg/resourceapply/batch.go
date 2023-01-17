package resourceapply

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
	return ApplyGenericWithHandlers[*batchv1.Job](
		ctx,
		control,
		recorder,
		required,
		options,
		nil,
		func(required *batchv1.Job, existing *batchv1.Job) string {
			if !equality.Semantic.DeepEqual(existing.Spec.Completions, required.Spec.Completions) {
				return "spec.completions is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
				return "spec.selector is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.Template, required.Spec.Template) {
				return "spec.template is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.CompletionMode, required.Spec.CompletionMode) {
				return "spec.completionMode is immutable"
			}
			if !equality.Semantic.DeepEqual(existing.Spec.PodFailurePolicy, required.Spec.PodFailurePolicy) {
				return "spec.podFailurePolicy is immutable"
			}
			return ""
		},
	)
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
