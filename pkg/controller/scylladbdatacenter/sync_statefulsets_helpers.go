package scylladbdatacenter

import (
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

// Helpers shared by the steps of the StatefulSet sync.

const (
	reasonWaitingForScyllaDBNodeExporterImage                     = "WaitingForScyllaDBNodeExporterImage"
	reasonWaitingForManagedConfig                                 = "WaitingForManagedConfig"
	reasonWaitingForScyllaDBDatacenterNodesStatusReportController = "WaitingForScyllaDBDatacenterNodesStatusReportController"
	reasonWaitingForStatefulSetRollout                            = "WaitingForStatefulSetRollout"
	reasonWaitingForRackServiceDecommission                       = "WaitingForRackServiceDecommission"
	reasonWaitingForMissingService                                = "WaitingForMissingService"
	reasonRunningUpgradeHooks                                     = "RunningUpgradeHooks"
	reasonUpgrading                                               = "Upgrading"
)

// newStatefulSetProgressingCondition makes a progressing condition of the StatefulSet sync. Returning one from a sync
// step means the sync has to stop and wait for a requeue.
func newStatefulSetProgressingCondition(sdc *scyllav1alpha1.ScyllaDBDatacenter, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               statefulSetControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: sdc.Generation,
	}
}

// getStatefulSetRolloutProgressingCondition returns a progressing condition when the StatefulSet hasn't rolled out yet,
// and nil when it has.
func getStatefulSetRolloutProgressingCondition(sdc *scyllav1alpha1.ScyllaDBDatacenter, sts *appsv1.StatefulSet) (*metav1.Condition, error) {
	rolledOut, err := controllerhelpers.IsStatefulSetRolledOut(sts)
	if err != nil {
		return nil, fmt.Errorf("can't verify statefulset %q rollout status: %w", naming.ObjRef(sts), err)
	}

	if rolledOut {
		return nil, nil
	}

	klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
	cond := newStatefulSetProgressingCondition(sdc, reasonWaitingForStatefulSetRollout, fmt.Sprintf("Waiting for StatefulSet %q to roll out.", naming.ObjRef(sts)))
	return &cond, nil
}

// updateRackStatus recomputes the status of the rack owned by the StatefulSet and records it in status, adding it when
// it's not there yet.
func updateRackStatus(
	podLister corev1listers.PodLister,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	sts *appsv1.StatefulSet,
	services map[string]*corev1.Service,
) error {
	rackName, ok := sts.Labels[naming.RackNameLabel]
	if !ok {
		return fmt.Errorf(
			"can't determine rack name: statefulset %s is missing label %q",
			naming.ObjRef(sts),
			naming.RackNameLabel,
		)
	}

	rackStatus := *calculateRackStatus(podLister, sdc, rackName, sts, services)
	_, idx, ok := oslices.Find(status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
		return rackStatus.Name == rackName
	})
	if ok {
		status.Racks[idx] = rackStatus
	} else {
		status.Racks = append(status.Racks, rackStatus)
	}

	return nil
}

// waitForStatefulSetCachePropagation gives the informers time to observe a StatefulSet we've just changed, so that
// the next sync doesn't act on a stale cache.
// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
func (sdcc *Controller) waitForStatefulSetCachePropagation() {
	time.Sleep(sdcc.statefulSetCachePropagationDelay)
}
