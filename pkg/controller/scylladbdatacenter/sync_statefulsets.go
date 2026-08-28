package scylladbdatacenter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

// The StatefulSet sync reconciles the rack StatefulSets of a ScyllaDBDatacenter. It is a pipeline of steps run in a
// fixed order by syncStatefulSets. Every step returns the progressing conditions it produced; returning any condition
// stops the pipeline until the next requeue. That is how racks are created, scaled and updated one at a time.
//
// The steps live in this file, except for scaling (sync_statefulsets_scale.go) and the upgrade state machine
// (sync_statefulsets_upgrade.go) together with its hooks (sync_statefulsets_upgrade_hooks.go).

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

// statefulSetSyncContext carries the inputs shared by the steps of the StatefulSet sync.
type statefulSetSyncContext struct {
	// key is the work queue key of the ScyllaDBDatacenter, used to requeue it with a delay.
	key string

	sdc *scyllav1alpha1.ScyllaDBDatacenter
	// status is the status being computed in this sync. Steps that change StatefulSets refresh the rack statuses in it.
	status *scyllav1alpha1.ScyllaDBDatacenterStatus

	// requiredStatefulSets are the StatefulSets the ScyllaDBDatacenter calls for, in rack order.
	requiredStatefulSets []*appsv1.StatefulSet
	// existingStatefulSets are the StatefulSets owned by the ScyllaDBDatacenter, by name.
	existingStatefulSets map[string]*appsv1.StatefulSet

	services   map[string]*corev1.Service
	configMaps map[string]*corev1.ConfigMap
}

// statefulSetSyncStep is one step of the StatefulSet sync. Returning any progressing condition stops the sync until
// the next requeue.
type statefulSetSyncStep struct {
	name string
	run  func(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error)
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
	requiredStatefulSets, progressingConditions, err := sdcc.makeRequiredStatefulSets(sdc, soc, statefulSets, configMaps)
	if err != nil || len(progressingConditions) > 0 {
		return progressingConditions, err
	}

	sc := &statefulSetSyncContext{
		key:                  key,
		sdc:                  sdc,
		status:               status,
		requiredStatefulSets: requiredStatefulSets,
		existingStatefulSets: statefulSets,
		services:             services,
		configMaps:           configMaps,
	}

	steps := []statefulSetSyncStep{
		// Delete has to be the first action to avoid getting stuck on quota.
		{name: "prune excessive StatefulSets", run: sdcc.pruneStatefulSets},
		{name: "wait for the nodes status report controller to settle", run: sdcc.waitForNodesStatusReportController},
		{name: "wait for the existing StatefulSets to roll out", run: sdcc.waitForExistingStatefulSetsRollout},
		{name: "create the missing StatefulSets", run: sdcc.createStatefulSets},
		{name: "scale the StatefulSets", run: sdcc.scaleStatefulSets},
		{name: "wait for all StatefulSets to roll out", run: sdcc.waitForAllStatefulSetsRollout},
		{name: "run the upgrade", run: sdcc.syncUpgrade},
		{name: "update the StatefulSets", run: sdcc.updateStatefulSets},
	}
	for _, step := range steps {
		klog.V(5).InfoS("Running StatefulSet sync step", "ScyllaDBDatacenter", klog.KObj(sdc), "Step", step.name)

		stepProgressingConditions, err := step.run(ctx, sc)
		progressingConditions = append(progressingConditions, stepProgressingConditions...)
		if err != nil {
			return progressingConditions, err
		}

		if len(progressingConditions) > 0 {
			klog.V(4).InfoS("StatefulSet sync is waiting", "ScyllaDBDatacenter", klog.KObj(sdc), "Step", step.name)
			return progressingConditions, nil
		}
	}

	return progressingConditions, nil
}

// makeRequiredStatefulSets builds the StatefulSets the ScyllaDBDatacenter calls for. It returns a progressing
// condition instead when an input they depend on isn't available yet.
func (sdcc *Controller) makeRequiredStatefulSets(
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	statefulSets map[string]*appsv1.StatefulSet,
	configMaps map[string]*corev1.ConfigMap,
) ([]*appsv1.StatefulSet, []metav1.Condition, error) {
	if soc.Status.ScyllaDBNodeExporterImage == nil {
		return nil, []metav1.Condition{
			newStatefulSetProgressingCondition(
				sdc,
				reasonWaitingForScyllaDBNodeExporterImage,
				"Waiting for ScyllaOperatorConfig to have scylladb-node-exporter image available in the status.",
			),
		}, nil
	}
	nodeExporterImage := *soc.Status.ScyllaDBNodeExporterImage

	managedScyllaDBConfigCMName := naming.GetScyllaDBManagedConfigCMName(sdc.Name)
	managedScyllaDBConfigCM, found := configMaps[managedScyllaDBConfigCMName]
	if !found {
		klog.V(2).InfoS("Waiting for managed config map", "ScyllaDBDatacenter", klog.KObj(sdc), "ConfigMapName", managedScyllaDBConfigCMName)
		return nil, []metav1.Condition{
			newStatefulSetProgressingCondition(
				sdc,
				reasonWaitingForManagedConfig,
				fmt.Sprintf("Waiting for ConfigMap %q to be created.", managedScyllaDBConfigCMName),
			),
		}, nil
	}

	inputsHash, err := hash.HashObjects(managedScyllaDBConfigCM.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("can't hash inputs: %w", err)
	}

	requiredStatefulSets, err := sdcc.makeRacks(sdc, statefulSets, nodeExporterImage, inputsHash)
	if err != nil {
		sdcc.eventRecorder.Eventf(
			sdc,
			corev1.EventTypeWarning,
			"InvalidRack",
			"Failed to make rack: %v", err,
		)
		return nil, nil, err
	}

	return requiredStatefulSets, nil, nil
}

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

// pruneStatefulSets deletes the StatefulSets that aren't required anymore and drops their rack statuses.
func (sdcc *Controller) pruneStatefulSets(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
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

		// TODO: Decommission the rack before removal.

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
		return progressingConditions, fmt.Errorf("can't delete StatefulSet(s): %w", err)
	}

	return progressingConditions, nil
}

// waitForNodesStatusReportController waits for the ScyllaDBDatacenterNodesStatusReport controller to settle before
// making any changes. This ensures that the status report is up to date, which lowers the chance of a new node being
// bootstrapped while the cluster is unhealthy.
func (sdcc *Controller) waitForNodesStatusReportController(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
	isProgressing := apimeta.IsStatusConditionTrue(sc.status.Conditions, scyllaDBDatacenterNodesStatusReportControllerProgressingCondition)
	isDegraded := apimeta.IsStatusConditionTrue(sc.status.Conditions, scyllaDBDatacenterNodesStatusReportControllerDegradedCondition)
	if !isProgressing && !isDegraded {
		return nil, nil
	}

	klog.V(4).InfoS("Waiting for ScyllaDBDatacenterNodesStatusReport controller to settle", "ScyllaDBDatacenter", klog.KObj(sc.sdc), "Progressing", isProgressing, "Degraded", isDegraded)
	return []metav1.Condition{
		newStatefulSetProgressingCondition(
			sc.sdc,
			reasonWaitingForScyllaDBDatacenterNodesStatusReportController,
			"Waiting for ScyllaDBDatacenterNodesStatusReport controller to settle.",
		),
	}, nil
}

// waitForExistingStatefulSetsRollout returns progressing conditions for the existing StatefulSets that haven't rolled
// out yet, so that racks bootstrap one by one. StatefulSets about to be scaled are skipped: when a member is
// decommissioned there is a Pod left that's not ready until the scale happens.
func (sdcc *Controller) waitForExistingStatefulSetsRollout(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	for _, req := range sc.requiredStatefulSets {
		sts, ok := sc.existingStatefulSets[req.Name]
		if !ok {
			continue
		}

		if req.Spec.Replicas != nil && sts.Spec.Replicas != nil &&
			*req.Spec.Replicas != *sts.Spec.Replicas {
			continue
		}

		cond, err := getStatefulSetRolloutProgressingCondition(sc.sdc, sts)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if cond != nil {
			progressingConditions = append(progressingConditions, *cond)
		}
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't check existing statefulset(s) rollout status: %w", err)
	}

	return progressingConditions, nil
}

// createStatefulSets creates the missing StatefulSets and records the statuses of the racks they belong to.
func (sdcc *Controller) createStatefulSets(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
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

// waitForAllStatefulSetsRollout waits for all racks to be up and ready before any of them is upgraded or updated.
// TODO: This blocks unstucking by an update. Also blocks lowering resources when the cluster is running low.
func (sdcc *Controller) waitForAllStatefulSetsRollout(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
	for _, req := range sc.requiredStatefulSets {
		sts := sc.existingStatefulSets[req.Name]

		cond, err := getStatefulSetRolloutProgressingCondition(sc.sdc, sts)
		if err != nil {
			return nil, err
		}

		if cond != nil {
			return []metav1.Condition{*cond}, nil
		}
	}

	return nil, nil
}

// updateStatefulSets applies the required StatefulSets one at a time, waiting for each to roll out before moving on
// to the next one. A change of the major or minor ScyllaDB version isn't applied directly but starts an upgrade
// instead.
func (sdcc *Controller) updateStatefulSets(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	anyStsChanged := false
	defer func() {
		if anyStsChanged {
			sdcc.waitForStatefulSetCachePropagation()
		}
	}()
	for _, required := range sc.requiredStatefulSets {
		existing, existingFound := sc.existingStatefulSets[required.Name]
		if existingFound {
			upgradeNeeded, fromVersion, toVersion, err := detectVersionUpgrade(required, existing)
			if err != nil {
				return progressingConditions, err
			}

			if upgradeNeeded {
				startConditions, err := sdcc.startUpgrade(ctx, sc.sdc, fromVersion, toVersion)
				progressingConditions = append(progressingConditions, startConditions...)
				return progressingConditions, err
			}
		}

		updatedSts, changed, err := resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, required, resourceapply.ApplyOptions{})
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply statefulset update: %w", err)
		}

		if changed {
			anyStsChanged = true

			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, required, "apply", sc.sdc.Generation)

			err = updateRackStatus(sdcc.podLister, sc.sdc, sc.status, updatedSts, sc.services)
			if err != nil {
				return progressingConditions, err
			}
		}

		// Wait for the StatefulSet to roll out.
		cond, err := getStatefulSetRolloutProgressingCondition(sc.sdc, updatedSts)
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

// waitForStatefulSetCachePropagation gives the informers time to observe a StatefulSet we've just changed, so that
// the next sync doesn't act on a stale cache.
// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
func (sdcc *Controller) waitForStatefulSetCachePropagation() {
	time.Sleep(sdcc.statefulSetCachePropagationDelay)
}

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
