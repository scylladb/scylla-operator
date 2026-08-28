package scylladbdatacenter

import (
	"context"
	"fmt"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// The upgrade step drives the upgrade state machine. A version upgrade (a change of the major or minor ScyllaDB
// version, started by the update step) is recorded in the upgrade context ConfigMap and goes through these phases:
//
//	PreHooks    -> run the pre-upgrade hook on the whole datacenter
//	RolloutInit -> partition all StatefulSets fully and update their templates, so no Pod is updated yet
//	RolloutRun  -> move the partitions one node at a time, running the node hooks before and after every node
//	PostHooks   -> run the post-upgrade hook and remove the upgrade context
//
// Every phase is reentrant. A phase transition is a change of the state recorded in the upgrade context ConfigMap.
// The hooks themselves are in sync_statefulsets_upgrade_hooks.go.

// syncUpgrade runs the current phase of the upgrade recorded in the upgrade context ConfigMap. It's a no-op when no
// upgrade is in progress.
func (sdcc *Controller) syncUpgrade(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error) {
	sdc := sc.sdc

	upgradeContextConfigMap, upgradeInProgress := sc.configMaps[naming.UpgradeContextConfigMapName(sdc)]
	if !upgradeInProgress {
		return nil, nil
	}

	var progressingConditions []metav1.Condition

	upgradeContext, err := sdcc.decodeUpgradeContext(upgradeContextConfigMap)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't decode upgrade context for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
	}

	progressingConditions = append(progressingConditions, newStatefulSetProgressingCondition(sdc, reasonRunningUpgradeHooks, "Running upgrade hooks"))

	fresh, err := sdcc.isUpgradeContextFresh(ctx, sdc, upgradeContext)
	if err != nil {
		return progressingConditions, err
	}
	if !fresh {
		klog.V(2).InfoS("Stale upgrade context, waiting for requeue", "ScyllaDBDatacenter", klog.KObj(sdc))
		return progressingConditions, nil
	}

	klog.V(4).InfoS("Upgrade is in progress", "Phase", upgradeContext.State)

	var phaseConditions []metav1.Condition
	switch upgradeContext.State {
	case internalapi.PreHooksUpgradePhase:
		phaseConditions, err = sdcc.runPreHooksUpgradePhase(ctx, sc, upgradeContext)

	case internalapi.RolloutInitUpgradePhase:
		phaseConditions, err = sdcc.runRolloutInitUpgradePhase(ctx, sc, upgradeContext)

	case internalapi.RolloutRunUpgradePhase:
		phaseConditions, err = sdcc.runRolloutRunUpgradePhase(ctx, sc, upgradeContext)

	case internalapi.PostHooksUpgradePhase:
		phaseConditions, err = sdcc.runPostHooksUpgradePhase(ctx, sc, upgradeContextConfigMap, upgradeContext)

	default:
		// An old cluster with an old state machine can still be going through an update, or stuck.
		// Given have to be reentrant we'll just start again to be sure no step is missed, even a new one.
		klog.Warningf("ScyllaCluster %q has an unknown upgrade phase %q. Resetting the phase.", klog.KObj(sdc), upgradeContext.State)
		phaseConditions, err = sdcc.transitionUpgradePhase(ctx, sdc, upgradeContext, internalapi.PreHooksUpgradePhase)
	}
	progressingConditions = append(progressingConditions, phaseConditions...)

	return progressingConditions, err
}

// runPreHooksUpgradePhase runs the pre-upgrade hook and moves on to initializing the rollout once it's done.
func (sdcc *Controller) runPreHooksUpgradePhase(ctx context.Context, sc *statefulSetSyncContext, upgradeContext *internalapi.DatacenterUpgradeContext) ([]metav1.Condition, error) {
	// TODO: Move the pre-upgrade hook into a Job.
	done, err := sdcc.beforeUpgrade(ctx, sc.sdc, sc.services, upgradeContext)
	if err != nil {
		return nil, err
	}
	if !done {
		sdcc.queue.AddAfter(sc.key, upgradeHookRequeueDelay)
		return nil, nil
	}

	return sdcc.transitionUpgradePhase(ctx, sc.sdc, upgradeContext, internalapi.RolloutInitUpgradePhase)
}

// runRolloutInitUpgradePhase partitions all StatefulSets fully while updating their templates, so that changes are
// blocked but no Pod is updated yet, and moves on to running the rollout.
func (sdcc *Controller) runRolloutInitUpgradePhase(ctx context.Context, sc *statefulSetSyncContext, upgradeContext *internalapi.DatacenterUpgradeContext) ([]metav1.Condition, error) {
	sdc := sc.sdc

	var errs []error
	anyStsChanged := false
	for _, required := range sc.requiredStatefulSets {
		existing, ok := sc.existingStatefulSets[required.Name]
		if !ok {
			// At this point all missing statefulSets should have been created.
			return nil, fmt.Errorf("internal error: can't lookup stateful set %s/%s", required.Namespace, required.Name)
		}
		// Apply a copy: the required StatefulSets are shared with the following steps and must stay as built.
		partitioned := required.DeepCopy()
		// We are depending on the current values so we need to use optimistic concurrency.
		// It will make sure we always set the corresponding partition for the scale.
		// It also forces our informers to be up-to-date.
		partitioned.ResourceVersion = existing.ResourceVersion
		// Avoid scaling.
		partitioned.Spec.Replicas = pointer.Ptr(*existing.Spec.Replicas)
		partitioned.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Ptr(*existing.Spec.Replicas)
		// Use apply to also update the spec.template
		updatedSts, changed, err := resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, partitioned, resourceapply.ApplyOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply statefulset to set partition: %w", err))
		}

		if changed {
			anyStsChanged = true

			err = updateRackStatus(sdcc.podLister, sdc, sc.status, updatedSts, sc.services)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	if anyStsChanged {
		sdcc.waitForStatefulSetCachePropagation()
	}
	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return sdcc.transitionUpgradePhase(ctx, sdc, upgradeContext, internalapi.RolloutRunUpgradePhase)
}

// runRolloutRunUpgradePhase moves the partition of the first StatefulSet that hasn't finished rolling by one node,
// running the post-node-upgrade hook for the node that was just rolled and the pre-node-upgrade hook for the next one.
// Once all StatefulSets are fully rolled it moves on to the post-hooks.
func (sdcc *Controller) runRolloutRunUpgradePhase(ctx context.Context, sc *statefulSetSyncContext, upgradeContext *internalapi.DatacenterUpgradeContext) ([]metav1.Condition, error) {
	sdc := sc.sdc
	services := sc.services

	for _, sts := range sc.requiredStatefulSets {
		partition := *sts.Spec.UpdateStrategy.RollingUpdate.Partition

		fresh, err := sdcc.isStatefulSetPartitionFresh(ctx, sts, partition)
		if err != nil {
			return nil, err
		}
		if !fresh {
			klog.V(2).InfoS("Stale StatefulSet partition, waiting for requeue", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
			return nil, nil
		}

		if partition < *sts.Spec.Replicas {
			// TODO: Move the post-node-upgrade hook into a Job.
			err = sdcc.afterNodeUpgrade(ctx, sdc, sts, partition, services, upgradeContext)
			if err != nil {
				return nil, err
			}
			klog.V(2).InfoS("AfterNodeUpgrade hook finished", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
		}

		if partition <= 0 {
			continue
		}

		nextPartition := partition - 1

		klog.V(4).InfoS("Upgrade is running a rollout", "Partition", partition, "NextPartition", nextPartition)

		// TODO: Move the pre-node-upgrade hook into a Job.
		done, err := sdcc.beforeNodeUpgrade(ctx, sdc, sts, nextPartition, services, upgradeContext)
		if err != nil {
			return nil, err
		}

		if !done {
			klog.V(4).InfoS("PreNodeUpgrade hook in progress. Waiting a bit.", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
			sdcc.queue.AddAfter(sc.key, upgradeHookRequeueDelay)
			return nil, nil
		}
		klog.V(2).InfoS("PreNodeUpgrade hook finished", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))

		err = sdcc.advanceStatefulSetPartition(ctx, sts, sc.existingStatefulSets[sts.Name], partition, nextPartition)
		if err != nil {
			return nil, err
		}

		// Partition can move only one rack a time.
		return nil, nil
	}

	return sdcc.transitionUpgradePhase(ctx, sdc, upgradeContext, internalapi.PostHooksUpgradePhase)
}

// runPostHooksUpgradePhase runs the post-upgrade hook and removes the upgrade context, which ends the upgrade.
func (sdcc *Controller) runPostHooksUpgradePhase(ctx context.Context, sc *statefulSetSyncContext, upgradeContextConfigMap *corev1.ConfigMap, upgradeContext *internalapi.DatacenterUpgradeContext) ([]metav1.Condition, error) {
	sdc := sc.sdc

	err := sdcc.afterUpgrade(ctx, sdc, sc.services, upgradeContext)
	if err != nil {
		return nil, err
	}

	var progressingConditions []metav1.Condition
	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, upgradeContextConfigMap, "delete", sdc.Generation)
	err = sdcc.kubeClient.CoreV1().ConfigMaps(upgradeContextConfigMap.Namespace).Delete(ctx, upgradeContextConfigMap.Name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{
			UID: &upgradeContextConfigMap.UID,
		},
		PropagationPolicy: pointer.Ptr(metav1.DeletePropagationBackground),
	})
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete upgrade context ConfigMap %q: %w", naming.ObjRef(upgradeContextConfigMap), err)
	}

	return progressingConditions, nil
}

// transitionUpgradePhase records the next phase in the upgrade context.
func (sdcc *Controller) transitionUpgradePhase(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, upgradeContext *internalapi.DatacenterUpgradeContext, nextPhase internalapi.UpgradePhase) ([]metav1.Condition, error) {
	upgradeContext.State = nextPhase
	return sdcc.applyUpgradeContext(ctx, sdc, upgradeContext)
}

// applyUpgradeContext writes the upgrade context into its ConfigMap.
func (sdcc *Controller) applyUpgradeContext(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, upgradeContext *internalapi.DatacenterUpgradeContext) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	cm, err := MakeUpgradeContextConfigMap(sdc, upgradeContext)
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

// isUpgradeContextFresh checks with a live call that the cached upgrade context is in the same phase as the one in the
// API server. Although hooks are mandated to be reentrant, they are pretty expensive to run so it's cheaper to recheck
// the phase than to run them against a stale cache.
// TODO: Remove the live call when the hooks are migrated to run as Jobs.
func (sdcc *Controller) isUpgradeContextFresh(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, cachedUpgradeContext *internalapi.DatacenterUpgradeContext) (bool, error) {
	cmName := naming.UpgradeContextConfigMapName(sdc)
	freshUpgradeContextConfigMap, err := sdcc.kubeClient.CoreV1().ConfigMaps(sdc.Namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("can't get upgrade context ConfigMap %q: %w", cmName, err)
	}

	freshUpgradeContext, err := sdcc.decodeUpgradeContext(freshUpgradeContextConfigMap)
	if err != nil {
		return false, fmt.Errorf("can't decode upgrade context for ScyllaDBDatacenter %q: %w", naming.ObjRef(sdc), err)
	}

	return freshUpgradeContext.State == cachedUpgradeContext.State, nil
}

// isStatefulSetPartitionFresh checks with a live call that the StatefulSet in the API server has the given partition.
// Although hooks are mandated to be reentrant, they are pretty expensive to run so it's cheaper to recheck the partition
// than to run them against a stale cache.
// TODO: Remove the live call when the hooks are migrated to run as Jobs.
func (sdcc *Controller) isStatefulSetPartitionFresh(ctx context.Context, sts *appsv1.StatefulSet, partition int32) (bool, error) {
	freshSts, err := sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	return freshSts.Spec.UpdateStrategy.RollingUpdate != nil && *freshSts.Spec.UpdateStrategy.RollingUpdate.Partition == partition, nil
}

// advanceStatefulSetPartition moves the partition of the StatefulSet from the given value to the next one, making sure
// it is still the StatefulSet we know at the given partition.
// TODO: Use bare update when hooks are extracted into Jobs. But at this point rerunning them is expensive so we retry
// with a condition check.
func (sdcc *Controller) advanceStatefulSetPartition(ctx context.Context, sts, existingSts *appsv1.StatefulSet, partition, nextPartition int32) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		freshSts, err := sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if existingSts != nil && freshSts.UID != existingSts.UID {
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

// upgradeHookRequeueDelay is how long to wait before checking a hook that is still in progress.
const upgradeHookRequeueDelay = 5 * time.Second
