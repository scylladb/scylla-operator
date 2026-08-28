package scylladbdatacenter

import (
	"context"
	"errors"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

// pruneStatefulSets deletes the StatefulSets that aren't required anymore and drops their rack statuses.
func (sdcc *Controller) pruneStatefulSets(ctx context.Context, sc *statefulSetSyncContext) (stepResult, error) {
	var errs []error
	var progressingConditions []metav1.Condition
	for _, sts := range sc.existingStatefulSets {
		if sts.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range sc.requiredStatefulSets {
			if sts.Name == req.Name {
				isRequired = true
			}
		}
		if isRequired {
			continue
		}

		// A rack can only be removed once it has no members: admission rejects removing a rack with members, so there is
		// nothing to decommission here.
		propagationPolicy := metav1.DeletePropagationBackground
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, sts, "delete", sc.sdc.Generation)
		err := sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Delete(ctx, sts.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sts.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}

		rackName, found := sts.Labels[naming.RackNameLabel]
		if !found {
			klog.ErrorS(errors.New("statefulset is missing a rack label"),
				"Can't clean rack status for deleted StatefulSet",
				"StatefulSet", klog.KObj(sts))
			continue
		}

		sc.status.Racks = oslices.FilterOut(sc.status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
			return rackStatus.Name == rackName
		})
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return blockWith(progressingConditions...), fmt.Errorf("can't delete StatefulSet(s): %w", err)
	}

	return blockWith(progressingConditions...), nil
}
