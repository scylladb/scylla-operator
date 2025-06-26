// Copyright (c) 2023 ScyllaDB.

package scylladbdatacenter

import (
	"context"
	"fmt"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sdcc *Controller) syncJobs(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	services map[string]*corev1.Service,
	jobs map[string]*batchv1.Job,
) ([]metav1.Condition, error) {
	requiredJobs, progressingConditions, err := MakeJobs(sdc, services, sdcc.podLister, sdcc.operatorImage)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make jobs: %w", err)
	}
	if len(progressingConditions) != 0 {
		return progressingConditions, nil
	}

	var progressingMessages []string
	var errs []error

	err = controllerhelpers.Prune(ctx, requiredJobs, jobs,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: sdcc.kubeClient.BatchV1().Jobs(sdc.Namespace).Delete,
		},
		sdcc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune job(s): %w", err)
	}

	if sdc.Status.ObservedGeneration == nil || (sdc.Generation != *sdc.Status.ObservedGeneration) {
		progressingMessages = append(progressingMessages, fmt.Sprintf("Waiting for status update of ScyllaCluster %q", naming.ObjRef(sdc)))
	}

	if apimeta.IsStatusConditionTrue(sdc.Status.Conditions, statefulSetControllerProgressingCondition) {
		progressingMessages = append(progressingMessages, fmt.Sprintf("Waiting for StatefulSet controller to finish progressing with ScyllaCluster %q", naming.ObjRef(sdc)))
	}

	if !apimeta.IsStatusConditionTrue(sdc.Status.Conditions, scyllav1.AvailableCondition) || !apimeta.IsStatusConditionFalse(sdc.Status.Conditions, scyllav1.DegradedCondition) {
		progressingMessages = append(progressingMessages, fmt.Sprintf("Waiting for ScyllaCluster %q nodes to be ready", naming.ObjRef(sdc)))
	}

	if len(progressingMessages) != 0 {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               jobControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.ProgressingReason,
			Message:            strings.Join(progressingMessages, "\n"),
			ObservedGeneration: sdc.Generation,
		})

		return progressingConditions, nil
	}

	for _, job := range requiredJobs {
		fresh, changed, err := resourceapply.ApplyJob(ctx, sdcc.kubeClient.BatchV1(), sdcc.jobLister, sdcc.eventRecorder, job, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, jobControllerProgressingCondition, job, "apply", sdc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply Job: %w", err))
			continue
		}

		if fresh.Status.CompletionTime == nil {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               jobControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForJobCompletion",
				Message:            fmt.Sprintf("Waiting for Job %q to complete.", naming.ObjRef(fresh)),
				ObservedGeneration: sdc.Generation,
			})
		}
	}
	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, err
	}

	return progressingConditions, nil
}
