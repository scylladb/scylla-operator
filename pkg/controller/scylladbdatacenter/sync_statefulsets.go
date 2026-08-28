package scylladbdatacenter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blang/semver"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func (sdcc *Controller) makeRacks(sdc *scyllav1alpha1.ScyllaDBDatacenter, statefulSets map[string]*appsv1.StatefulSet, nodeExporterImage string, inputsHash string) ([]*appsv1.StatefulSet, error) {
	sets := make([]*appsv1.StatefulSet, 0, len(sdc.Spec.Racks))
	for i, rack := range sdc.Spec.Racks {
		oldSts := statefulSets[naming.StatefulSetNameForRack(rack, sdc)]
		sts, err := StatefulSetForRack(rack, sdc, oldSts, sdcc.operatorImage, nodeExporterImage, i, inputsHash)
		if err != nil {
			return nil, err
		}

		sets = append(sets, sts)
	}
	return sets, nil
}

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

func (sdcc *Controller) pruneStatefulSets(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	requiredStatefulSets []*appsv1.StatefulSet,
	statefulSets map[string]*appsv1.StatefulSet,
) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition
	for _, sts := range statefulSets {
		if sts.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredStatefulSets {
			if sts.Name == req.Name {
				isRequired = true
			}
		}
		if isRequired {
			continue
		}

		// TODO: Decommission the rack before removal.

		propagationPolicy := metav1.DeletePropagationBackground
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, sts, "delete", sdc.Generation)
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

		status.Racks = oslices.FilterOut(status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
			return rackStatus.Name == rackName
		})
	}
	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}

// checkExistingStatefulSetsRolloutStatus returns progressing conditions for existing StatefulSets that haven't rolled out yet.
func (sdcc *Controller) checkExistingStatefulSetsRolloutStatus(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	requiredStatefulSets []*appsv1.StatefulSet,
	statefulSets map[string]*appsv1.StatefulSet,
) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	for _, req := range requiredStatefulSets {
		sts, ok := statefulSets[req.Name]
		if !ok {
			continue
		}

		// When we decommission a member there is a pod left that's not ready until we scale.
		if req.Spec.Replicas != nil && sts.Spec.Replicas != nil &&
			*req.Spec.Replicas != *sts.Spec.Replicas {
			continue
		}

		cond, err := getStatefulSetRolloutProgressingCondition(sdc, sts)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if cond != nil {
			progressingConditions = append(progressingConditions, *cond)
		}
	}

	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
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

func (sdcc *Controller) syncStatefulSets(
	ctx context.Context,
	key string,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	statefulSets map[string]*appsv1.StatefulSet,
	services map[string]*corev1.Service,
	configMaps map[string]*corev1.ConfigMap,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	if soc.Status.ScyllaDBNodeExporterImage == nil {
		progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(
			sdc,
			reasonWaitingForScyllaDBNodeExporterImage,
			"Waiting for ScyllaOperatorConfig to have scylladb-node-exporter image available in the status.",
		))
		return progressingConditions, nil
	}
	nodeExporterImage := *soc.Status.ScyllaDBNodeExporterImage

	managedScyllaDBConfigCMName := naming.GetScyllaDBManagedConfigCMName(sdc.Name)
	managedScyllaDBConfigCM, found := configMaps[managedScyllaDBConfigCMName]
	if !found {
		klog.V(2).InfoS("Waiting for managed config map", "ScyllaDBDatacenter", klog.KObj(sdc), "ConfigMapName", managedScyllaDBConfigCMName)
		progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(
			sdc,
			reasonWaitingForManagedConfig,
			fmt.Sprintf("Waiting for ConfigMap %q to be created.", managedScyllaDBConfigCMName),
		))
		return progressingConditions, nil
	}

	inputsHash, err := hash.HashObjects(managedScyllaDBConfigCM.Data)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't hash inputs: %w", err)
	}

	requiredStatefulSets, err := sdcc.makeRacks(sdc, statefulSets, nodeExporterImage, inputsHash)
	if err != nil {
		sdcc.eventRecorder.Eventf(
			sdc,
			corev1.EventTypeWarning,
			"InvalidRack",
			"Failed to make rack: %v", err,
		)
		return progressingConditions, err
	}

	// Delete any excessive StatefulSets.
	// Delete has to be the first action to avoid getting stuck on quota.
	pruneProgressingConditions, err := sdcc.pruneStatefulSets(ctx, sdc, status, requiredStatefulSets, statefulSets)
	progressingConditions = append(progressingConditions, pruneProgressingConditions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete StatefulSet(s): %w", err)
	}

	// Wait for the ScyllaDBDatacenterNodesStatusReport controller to settle before proceeding.
	// This ensures that the status report is up to date before we start making changes,
	// which lowers the chance of a new node being bootstrapped while the cluster is unhealthy.
	isScyllaDBDatacenterNodesStatusReportControllerProgressing := apimeta.IsStatusConditionTrue(status.Conditions, scyllaDBDatacenterNodesStatusReportControllerProgressingCondition)
	isScyllaDBDatacenterNodesStatusReportControllerDegraded := apimeta.IsStatusConditionTrue(status.Conditions, scyllaDBDatacenterNodesStatusReportControllerDegradedCondition)
	if isScyllaDBDatacenterNodesStatusReportControllerProgressing || isScyllaDBDatacenterNodesStatusReportControllerDegraded {
		klog.V(4).InfoS("Waiting for ScyllaDBDatacenterNodesStatusReport controller to settle", "ScyllaDBDatacenter", klog.KObj(sdc), "Progressing", isScyllaDBDatacenterNodesStatusReportControllerProgressing, "Degraded", isScyllaDBDatacenterNodesStatusReportControllerDegraded)
		progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(
			sdc,
			reasonWaitingForScyllaDBDatacenterNodesStatusReportController,
			"Waiting for ScyllaDBDatacenterNodesStatusReport controller to settle.",
		))
	}
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	progressingConditions, err = sdcc.checkExistingStatefulSetsRolloutStatus(ctx, sdc, requiredStatefulSets, statefulSets)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't check existing statefulset(s) rollout status: %w", err)
	}
	// Wait for existing StatefulSets to roll out. Racks can only bootstrap one by one.
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Before any update, make sure all StatefulSets are present.
	// Create any that are missing.
	createdStatefulSets, createProgressingConditions, err := createMissingStatefulSets(
		ctx,
		func(ctx context.Context, required *appsv1.StatefulSet) (*appsv1.StatefulSet, bool, error) {
			return resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, required, resourceapply.ApplyOptions{})
		},
		sdc,
		requiredStatefulSets,
		statefulSets,
	)
	progressingConditions = append(progressingConditions, createProgressingConditions...)
	defer func() {
		if len(createProgressingConditions) > 0 {
			// Wait for the informers to catch up.
			// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
			time.Sleep(sdcc.statefulSetCachePropagationDelay)
		}
	}()
	var createErrs []error
	if err != nil {
		createErrs = append(createErrs, fmt.Errorf("can't create StatefulSet(s): %w", err))
	}

	// Record the statuses of the StatefulSets that were created, even if creating another one failed.
	err = ensureRackNamesInRackStatuses(sdcc.podLister, sdc, status, createdStatefulSets, services)
	if err != nil {
		createErrs = append(createErrs, fmt.Errorf("can't update status with rack statuses: %w", err))
	}

	if len(createErrs) > 0 {
		return progressingConditions, apimachineryutilerrors.NewAggregate(createErrs)
	}

	// Return to wait for created StatefulSets to roll out before proceeding.
	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	// Scale before the update.
	for _, req := range requiredStatefulSets {
		sts := statefulSets[req.Name]

		scale := &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name:            sts.Name,
				Namespace:       sts.Namespace,
				ResourceVersion: sts.ResourceVersion,
			},
			Spec: autoscalingv1.ScaleSpec{
				Replicas: *req.Spec.Replicas,
			},
		}

		rackServices := map[string]*corev1.Service{}
		for _, svc := range services {
			svcRackName, ok := svc.Labels[naming.RackNameLabel]
			if ok && svcRackName == sts.Labels[naming.RackNameLabel] {
				rackServices[svc.Name] = svc
			}
		}

		// Wait if any decommissioning is in progress.
		for _, svc := range rackServices {
			if svc.Labels[naming.DecommissionedLabel] == naming.LabelValueFalse {
				klog.V(4).InfoS("Waiting for service to be decommissioned")
				progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(
					sdc,
					reasonWaitingForRackServiceDecommission,
					fmt.Sprintf("Waiting for rack service %q to decommission.", naming.ObjRef(svc)),
				))

				return progressingConditions, nil
			}
		}

		if scale.Spec.Replicas == *sts.Spec.Replicas {
			continue
		}

		if scale.Spec.Replicas < *sts.Spec.Replicas {
			// Make sure we always scale down by 1 member.
			scale.Spec.Replicas = *sts.Spec.Replicas - 1

			lastSvcName := fmt.Sprintf("%s-%d", sts.Name, *sts.Spec.Replicas-1)
			lastSvc, ok := rackServices[lastSvcName]
			if !ok {
				klog.V(4).InfoS("Missing service", "ScyllaDBDatacenter", klog.KObj(sdc), "ServiceName", lastSvcName)
				progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(
					sdc,
					reasonWaitingForMissingService,
					fmt.Sprintf("Statusfulset %q is waiting for service %q to be created", naming.ObjRef(req), lastSvcName),
				))
				// Services are managed in the other loop.
				// When informers see the new service, will get re-queued.
				return progressingConditions, nil
			}

			if len(lastSvc.Labels[naming.DecommissionedLabel]) == 0 {
				lastSvcCopy := lastSvc.DeepCopy()
				// Record the intent to decommission the member.
				// TODO: Move this into syncServices so it reconciles properly. This is edge triggered
				//  and nothing will reconcile the label if something goes wrong or the flow changes.
				lastSvcCopy.Labels[naming.DecommissionedLabel] = naming.LabelValueFalse
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, lastSvcCopy, "update", sdc.Generation)
				_, err := sdcc.kubeClient.CoreV1().Services(lastSvcCopy.Namespace).Update(ctx, lastSvcCopy, metav1.UpdateOptions{})
				if err != nil {
					return progressingConditions, err
				}
				return progressingConditions, nil
			}
		}

		klog.V(2).InfoS("Scaling StatefulSet", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts), "CurrentReplicas", *sts.Spec.Replicas, "UpdatedReplicas", scale.Spec.Replicas)
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, scale, "updateScale", sdc.Generation)
		_, err = sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).UpdateScale(ctx, sts.Name, scale, metav1.UpdateOptions{})
		if err != nil {
			return progressingConditions, fmt.Errorf("can't update scale: %w", err)
		}
		return progressingConditions, err
	}

	// TODO: This blocks unstucking by an update.
	//  	 Also blocks lowering resources when the cluster is running low.
	// Wait for all racks to be up and ready.
	for _, req := range requiredStatefulSets {
		sts := statefulSets[req.Name]

		cond, err := getStatefulSetRolloutProgressingCondition(sdc, sts)
		if err != nil {
			return progressingConditions, err
		}

		if cond != nil {
			progressingConditions = append(progressingConditions, *cond)
			return progressingConditions, nil
		}
	}

	upgradeContextConfigMap, ok := configMaps[naming.UpgradeContextConfigMapName(sdc)]
	// Run hooks if an upgrade is in progress.
	if ok {
		currentUpgradeContext, err := sdcc.decodeUpgradeContext(upgradeContextConfigMap)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't decode upgrade context for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
		}

		progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(sdc, reasonRunningUpgradeHooks, "Running upgrade hooks"))

		// Isolate the live values in a block to prevent accidental use.
		{
			// We could still see an old status. Although hooks are mandated to be reentrant,
			// they are pretty expensive to run so it's cheaper to recheck the partition with a live call.
			// TODO: Remove the live call when the hooks are migrated to run as Jobs.
			freshUpgradeContextConfigMap, err := sdcc.kubeClient.CoreV1().ConfigMaps(sdc.Namespace).Get(ctx, naming.UpgradeContextConfigMapName(sdc), metav1.GetOptions{})
			if err != nil {
				return progressingConditions, fmt.Errorf("can't get upgrade context ConfigMap %q: %w", naming.UpgradeContextConfigMapName(sdc), err)
			}

			freshUpgradeContext, err := sdcc.decodeUpgradeContext(freshUpgradeContextConfigMap)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't decode upgrade context for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
			}

			if freshUpgradeContext.State != currentUpgradeContext.State {
				// Wait for requeue.
				klog.V(2).InfoS("Stale upgrade context, waiting for requeue", "ScyllaDBDatacenter", sdc)
				return progressingConditions, err
			}
		}

		klog.V(4).InfoS("Upgrade is in progress", "Phase", currentUpgradeContext.State)
		switch currentUpgradeContext.State {
		case internalapi.PreHooksUpgradePhase:
			// TODO: Move the pre-upgrade hook into a Job.
			done, err := sdcc.beforeUpgrade(ctx, sdc, services, currentUpgradeContext)
			if err != nil {
				return progressingConditions, err
			}
			if !done {
				sdcc.queue.AddAfter(key, 5*time.Second)
				return progressingConditions, nil
			}

			currentUpgradeContext.State = internalapi.RolloutInitUpgradePhase
			cm, err := MakeUpgradeContextConfigMap(sdc, currentUpgradeContext)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't make upgrade context ConfigMap: %w", err)
			}

			cm, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, cm, "apply", sdc.Generation)
			}
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply upgrade context ConfigMap: %w", err)
			}

			return progressingConditions, nil

		case internalapi.RolloutInitUpgradePhase:
			// Partition all StatefulSet at once to block changes but no Pod update is done yet.
			var errs []error
			anyStsChanged := false
			for _, required := range requiredStatefulSets {
				existing, ok := statefulSets[required.Name]
				if !ok {
					// At this point all missing statefulSets should have been created.
					return progressingConditions, fmt.Errorf("internal error: can't lookup stateful set %s/%s", required.Namespace, required.Name)
				}
				// We are depending on the current values so we need to use optimistic concurrency.
				// It will make sure we always set the corresponding partition for the scale.
				// It also forces our informers to be up-to-date.
				required.ResourceVersion = existing.ResourceVersion
				// Avoid scaling.
				required.Spec.Replicas = pointer.Ptr(*existing.Spec.Replicas)
				required.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Ptr(*existing.Spec.Replicas)
				// Use apply to also update the spec.template
				updatedSts, changed, err := resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, required, resourceapply.ApplyOptions{})
				if err != nil {
					errs = append(errs, fmt.Errorf("can't apply statefulset to set partition: %w", err))
				}

				if changed {
					anyStsChanged = true

					err = updateRackStatus(sdcc.podLister, sdc, status, updatedSts, services)
					if err != nil {
						errs = append(errs, err)
						continue
					}
				}
			}
			if anyStsChanged {
				// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
				time.Sleep(sdcc.statefulSetCachePropagationDelay)
			}
			err = apimachineryutilerrors.NewAggregate(errs)
			if err != nil {
				return progressingConditions, err
			}

			currentUpgradeContext.State = internalapi.RolloutRunUpgradePhase
			cm, err := MakeUpgradeContextConfigMap(sdc, currentUpgradeContext)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't make upgrade context ConfigMap: %w", err)
			}

			cm, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, cm, "apply", sdc.Generation)
			}
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply upgrade context ConfigMap: %w", err)
			}

			return progressingConditions, nil

		case internalapi.RolloutRunUpgradePhase:
			for _, sts := range requiredStatefulSets {
				partition := *sts.Spec.UpdateStrategy.RollingUpdate.Partition

				// Isolate the live values in a block to prevent accidental use.
				{
					// TODO: Remove the live call when hooks are migrated into Jobs.
					// We could still see an old partition. Although hooks are mandated to be reentrant,
					// they are pretty expensive to run so it's cheaper to recheck the partition with a live call.
					freshSts, err := sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
					if err != nil {
						return progressingConditions, err
					}

					if freshSts.Spec.UpdateStrategy.RollingUpdate == nil ||
						*freshSts.Spec.UpdateStrategy.RollingUpdate.Partition != partition {
						// Wait for requeue.
						klog.V(2).InfoS("Stale StatefulSet partition, waiting for requeue", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
						return progressingConditions, nil
					}
				}

				if partition < *sts.Spec.Replicas {
					// TODO: Move the post-node-upgrade hook into a Job.
					err = sdcc.afterNodeUpgrade(ctx, sdc, sts, partition, services, currentUpgradeContext)
					if err != nil {
						return progressingConditions, err
					}
					klog.V(2).InfoS("AfterNodeUpgrade hook finished", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
				}

				if partition <= 0 {
					continue
				}

				nextPartition := partition - 1

				klog.V(4).InfoS("Upgrade is running a rollout", "Partition", partition, "NextPartition", nextPartition)

				// TODO: Move the pre-node-upgrade hook into a Job.
				done, err := sdcc.beforeNodeUpgrade(ctx, sdc, sts, nextPartition, services, currentUpgradeContext)
				if err != nil {
					return progressingConditions, err
				}

				if !done {
					klog.V(4).InfoS("PreNodeUpgrade hook in progress. Waiting a bit.", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
					sdcc.queue.AddAfter(key, 5*time.Second)
					return progressingConditions, nil
				}
				klog.V(2).InfoS("PreNodeUpgrade hook finished", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))

				// TODO: Use bare update when hooks are extracted into Jobs.
				//       But at this point rerunning them is expensive so we retry with condition check.
				err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					freshSts, err := sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}

					existingSts, found := statefulSets[freshSts.Name]
					if found && freshSts.UID != existingSts.UID {
						return fmt.Errorf("statefulset was recreated in the meantime")
					}

					if freshSts.Spec.UpdateStrategy.RollingUpdate == nil ||
						*freshSts.Spec.UpdateStrategy.RollingUpdate.Partition != partition {
						return fmt.Errorf("statefulset partition mismatch: expected %d, got %d", partition, *freshSts.Spec.UpdateStrategy.RollingUpdate.Partition)

					}

					freshSts.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Ptr(nextPartition)
					_, err = sdcc.kubeClient.AppsV1().StatefulSets(freshSts.Namespace).Update(ctx, freshSts, metav1.UpdateOptions{})
					if err != nil {
						return err
					}

					return nil
				})
				if err != nil {
					return progressingConditions, err
				}

				// Partition can move only one rack a time.
				return progressingConditions, nil
			}

			currentUpgradeContext.State = internalapi.PostHooksUpgradePhase
			cm, err := MakeUpgradeContextConfigMap(sdc, currentUpgradeContext)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't make upgrade context ConfigMap: %w", err)
			}

			cm, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, cm, "apply", sdc.Generation)
			}
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply upgrade context ConfigMap: %w", err)
			}

			return progressingConditions, nil

		case internalapi.PostHooksUpgradePhase:
			err = sdcc.afterUpgrade(ctx, sdc, services, currentUpgradeContext)
			if err != nil {
				return progressingConditions, err
			}

			cmName := naming.UpgradeContextConfigMapName(sdc)
			cm, ok := configMaps[cmName]
			if !ok {
				return progressingConditions, nil
			}

			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, cm, "delete", sdc.Generation)
			err = sdcc.kubeClient.CoreV1().ConfigMaps(sdc.Namespace).Delete(ctx, cmName, metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: &cm.UID,
				},
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationBackground),
			})
			if err != nil {
				return progressingConditions, fmt.Errorf("can't delete upgrade context ConfigMap %q: %w", naming.ManualRef(sdc.Namespace, cmName), err)
			}

			return progressingConditions, nil

		default:
			// An old cluster with an old state machine can still be going through an update, or stuck.
			// Given have to be reentrant we'll just start again to be sure no step is missed, even a new one.
			klog.Warningf("ScyllaCluster %q has an unknown upgrade phase %q. Resetting the phase.", klog.KObj(sdc), currentUpgradeContext.State)
			currentUpgradeContext.State = internalapi.PreHooksUpgradePhase
			cm, err := MakeUpgradeContextConfigMap(sdc, currentUpgradeContext)
			if err != nil {
				return progressingConditions, fmt.Errorf("can't make upgrade context ConfigMap: %w", err)
			}

			cm, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, cm, "apply", sdc.Generation)
			}
			if err != nil {
				return progressingConditions, fmt.Errorf("can't apply upgrade context ConfigMap: %w", err)
			}

			return progressingConditions, nil
		}
	}

	// Begin the update.
	anyStsChanged := false
	defer func() {
		if anyStsChanged {
			// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
			time.Sleep(sdcc.statefulSetCachePropagationDelay)
		}
	}()
	for _, required := range requiredStatefulSets {
		// Check for version upgrades first.
		existing, existingFound := statefulSets[required.Name]
		if existingFound && upgradeContextConfigMap == nil {
			requiredVersionString, requiredVersionLabelPresent := required.Labels[naming.ScyllaVersionLabel]
			existingVersionString, existingVersionLabelPresent := existing.Labels[naming.ScyllaVersionLabel]

			if requiredVersionLabelPresent && existingVersionLabelPresent {
				requiredVersion, err := semver.Parse(requiredVersionString)
				if err != nil {
					return progressingConditions, err
				}
				existingVersion, err := semver.Parse(existingVersionString)
				if err != nil {
					return progressingConditions, err
				}

				if requiredVersion.Major != existingVersion.Major ||
					requiredVersion.Minor != existingVersion.Minor {
					// We need to run hooks for version upgrades.
					sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeNormal, "UpgradeStarted", "Version changed from %q to %q", existingVersionString, requiredVersionString)

					progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(sdc, reasonUpgrading, "Starting cluster upgrade"))

					// Initiate the upgrade. This triggers a state machine to run hooks first.
					now := time.Now()

					cm, err := MakeUpgradeContextConfigMap(sdc, &internalapi.DatacenterUpgradeContext{
						State:             internalapi.PreHooksUpgradePhase,
						FromVersion:       existingVersionString,
						ToVersion:         requiredVersionString,
						SystemSnapshotTag: snapshotTag("system", now),
						DataSnapshotTag:   snapshotTag("data", now),
					})
					if err != nil {
						return progressingConditions, fmt.Errorf("can't make upgrade context ConfigMap: %w", err)
					}

					cm, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, cm, resourceapply.ApplyOptions{})
					if changed {
						controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, cm, "apply", sdc.Generation)
					}
					if err != nil {
						return progressingConditions, fmt.Errorf("can't apply upgrade context ConfigMap: %w", err)
					}

					return progressingConditions, nil
				}
			}
		}

		updatedSts, changed, err := resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, required, resourceapply.ApplyOptions{})
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply statefulset update: %w", err)
		}

		if changed {
			anyStsChanged = true

			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, required, "apply", sdc.Generation)

			err = updateRackStatus(sdcc.podLister, sdc, status, updatedSts, services)
			if err != nil {
				return progressingConditions, err
			}
		}

		// Wait for the StatefulSet to roll out.
		cond, err := getStatefulSetRolloutProgressingCondition(sdc, updatedSts)
		if err != nil {
			return progressingConditions, err
		}

		if cond != nil {
			progressingConditions = append(progressingConditions, *cond)
			return progressingConditions, nil
		}
	}

	return progressingConditions, nil
}

func (sdcc *Controller) setStatefulSetsAvailableStatusCondition(
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
) {
	desiredMembers := int32(0)
	updatedMembers := int32(0)
	readyMembers := int32(0)
	var racksInDifferentVersion []string
	for _, rack := range sdc.Spec.Racks {
		rackCount, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			klog.ErrorS(err, "can't get rack node count", "ScyllaDBDatacenter", naming.ObjRef(sdc), "Rack", rack.Name)
			continue
		}
		desiredMembers += *rackCount

		rackStatus, _, found := oslices.Find(status.Racks, func(status scyllav1alpha1.RackStatus) bool {
			return status.Name == rack.Name
		})
		if !found {
			klog.Errorf("Can't find status for rack %q", rack.Name)
			continue
		}

		expectedVersion, err := naming.ImageToVersion(sdc.Spec.ScyllaDB.Image)
		if err != nil {
			klog.ErrorS(err, "can't get version from image", "Image", sdc.Spec.ScyllaDB.Image)
			continue
		}

		if rackStatus.CurrentVersion != expectedVersion {
			racksInDifferentVersion = append(racksInDifferentVersion, rack.Name)
		}

		if rackStatus.Stale == nil || (*rackStatus.Stale) {
			continue
		}

		if rackStatus.ReadyNodes != nil {
			readyMembers += *rackStatus.ReadyNodes
		}

		if rackStatus.UpdatedNodes != nil {
			updatedMembers += *rackStatus.UpdatedNodes
		}
	}

	switch {
	case len(racksInDifferentVersion) > 0:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "RacksNotAtDesiredVersion",
			Message:            fmt.Sprintf("Racks %q are not in the desired version", strings.Join(racksInDifferentVersion, ", ")),
			ObservedGeneration: sdc.Generation,
		})

	case updatedMembers != desiredMembers:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "MembersNotUpdated",
			Message:            fmt.Sprintf("Only %d out of %d member(s) have been updated", updatedMembers, desiredMembers),
			ObservedGeneration: sdc.Generation,
		})

	case readyMembers != desiredMembers:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "MembersNotReady",
			Message:            fmt.Sprintf("Only %d out of %d member(s) are ready", readyMembers, desiredMembers),
			ObservedGeneration: sdc.Generation,
		})

	default:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sdc.Generation,
		})
	}

	return
}

func (sdcc *Controller) decodeUpgradeContext(upgradeContextConfigMap *corev1.ConfigMap) (*internalapi.DatacenterUpgradeContext, error) {
	ucRaw, ok := upgradeContextConfigMap.Data[naming.UpgradeContextConfigMapKey]
	if !ok {
		return nil, fmt.Errorf("upgrade context ConfigMap %q is missing %q key", naming.ObjRef(upgradeContextConfigMap), naming.UpgradeContextConfigMapKey)
	}

	uc := &internalapi.DatacenterUpgradeContext{}
	err := uc.Decode(strings.NewReader(ucRaw))
	if err != nil {
		return nil, fmt.Errorf("can't decode ugprade context from ConfigMap %q: %w", naming.ObjRef(upgradeContextConfigMap), err)
	}

	return uc, nil
}
