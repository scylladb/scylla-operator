package resourceapply

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// ApplyJob will apply the Job to match the required object.
// forceOwnership allows to apply objects without an ownerReference. Normally such objects
// would be adopted but the old objects may not have correct labels that we need to fix in the new version.
func ApplyJob(
	ctx context.Context,
	client batchv1client.JobsGetter,
	lister batchv1listers.JobLister,
	recorder record.EventRecorder,
	required *batchv1.Job,
) (*batchv1.Job, bool, error) {
	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return nil, false, fmt.Errorf("job %q is missing controllerRef", naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopy()
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return nil, false, err
	}

	requiredHash := requiredCopy.Annotations[naming.ManagedHash]
	existing, err := lister.Jobs(requiredCopy.Namespace).Get(requiredCopy.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}

		actual, err := client.Jobs(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Job", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, true, err
	}

	// TODO: handle failed jobs.

	existingHash := existing.Annotations[naming.ManagedHash]

	// If they are the same do nothing.
	if existingHash == requiredHash {
		return existing, false, nil
	}

	if !equality.Semantic.DeepEqual(existing.Spec.Completions, requiredCopy.Spec.Completions) ||
		!equality.Semantic.DeepEqual(existing.Spec.Template, requiredCopy.Spec.Template) ||
		(requiredCopy.Spec.ManualSelector != nil && *requiredCopy.Spec.ManualSelector && !equality.Semantic.DeepEqual(existing.Spec.Selector, requiredCopy.Spec.Selector)) {
		klog.V(2).InfoS(
			"Apply needs to change immutable field(s) and will recreate the object",
			"Job", naming.ObjRefWithUID(existing),
		)

		propagationPolicy := metav1.DeletePropagationBackground
		err := client.Jobs(existing.Namespace).Delete(ctx, existing.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		})
		ReportDeleteEvent(recorder, existing, err)
		if err != nil {
			return nil, false, err
		}

		created, err := client.Jobs(requiredCopy.Namespace).Create(ctx, requiredCopy, metav1.CreateOptions{})
		ReportCreateEvent(recorder, requiredCopy, err)
		if err != nil {
			return nil, false, err
		}

		return created, true, nil
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.ResourceVersion) == 0 {
		requiredCopy.ResourceVersion = existing.ResourceVersion
	}

	actual, err := client.Jobs(requiredCopy.Namespace).Update(ctx, requiredCopy, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Job", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return nil, false, fmt.Errorf("can't update job: %w", err)
	}
	return actual, true, nil
}
