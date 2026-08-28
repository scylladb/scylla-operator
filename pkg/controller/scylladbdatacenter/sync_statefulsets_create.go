package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

// createStatefulSets creates the missing StatefulSets and records the statuses of the racks they belong to.
func (sdcc *Controller) createStatefulSets(ctx context.Context, sc *statefulSetSyncContext) (stepResult, error) {
	createdStatefulSets, progressingConditions, err := createMissingStatefulSets(
		ctx,
		func(ctx context.Context, required *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
			return resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, required, resourceapply.ApplyOptions{})
		},
		sc.sdc,
		sc.requiredStatefulSets,
		sc.existingStatefulSets,
	)
	if len(createdStatefulSets) > 0 {
		defer sdcc.waitForStatefulSetCachePropagation()
	}

	var errs []error
	if err != nil {
		errs = append(errs, fmt.Errorf("can't create StatefulSet(s): %w", err))
	}

	// Record the statuses of the StatefulSets that were created, even if creating another one failed.
	err = ensureRackNamesInRackStatuses(sdcc.podLister, sc.sdc, sc.status, createdStatefulSets, sc.services)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status with rack statuses: %w", err))
	}

	return blockWith(progressingConditions...), apimachineryutilerrors.NewAggregate(errs)
}

// createMissingStatefulSets creates the missing StatefulSets from requiredStatefulSets.
// Existing StatefulSets are skipped. With parallel node operations disabled at most one missing StatefulSet is created
// so that racks bootstrap one by one, while with parallel node operations enabled all of them are created at once.
// It returns the StatefulSets created so far and their progressing conditions, together with an error if a creation
// failed.
func createMissingStatefulSets(
	ctx context.Context,
	applyStatefulSet func(context.Context, *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error),
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	requiredStatefulSets []*appsv1.StatefulSet,
	statefulSets map[string]*appsv1.StatefulSet,
) ([]*appsv1.StatefulSet, []metav1.Condition, error) {
	parallelNodeOperationsEnabled, err := effectiveParallelNodeOperationsEnabled(sdc)
	if err != nil {
		return nil, nil, fmt.Errorf("can't determine effective parallel node operations enablement: %w", err)
	}

	createdStatefulSets := make([]*appsv1.StatefulSet, 0)
	progressingConditions := make([]metav1.Condition, 0)
	for _, req := range requiredStatefulSets {
		sts, found := statefulSets[req.Name]
		if found {
			continue
		}

		klog.V(2).InfoS("Creating missing StatefulSet", "StatefulSet", klog.KObj(req))
		var changed bool
		var err error
		sts, changed, err = applyStatefulSet(ctx, req)
		if err != nil {
			return createdStatefulSets, progressingConditions, fmt.Errorf("can't create missing statefulset %q: %w", naming.ManualRef(sdc.Namespace, req.Name), err)
		}
		if !changed {
			continue
		}

		createdStatefulSets = append(createdStatefulSets, sts)
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, req, "apply", sdc.Generation)

		if !parallelNodeOperationsEnabled {
			// StatefulSets must be created sequentially. Return early.
			return createdStatefulSets, progressingConditions, nil
		}
	}

	return createdStatefulSets, progressingConditions, nil
}

// ensureRackNamesInRackStatuses records statuses for newly created racks before informer caches catch up.
func ensureRackNamesInRackStatuses(
	podLister corev1listers.PodLister,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	statefulSets []*appsv1.StatefulSet,
	services map[string]*corev1.Service,
) error {
	var errs []error

	for _, sts := range statefulSets {
		err := updateRackStatus(podLister, sdc, status, sts, services)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
