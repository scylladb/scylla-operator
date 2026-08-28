package scylladbdatacenter

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// detectVersionUpgrade compares the ScyllaDB versions of the required and the existing StatefulSet and reports whether
// they differ in the major or minor version, in which case the change has to go through the upgrade state machine.
func detectVersionUpgrade(required, existing *appsv1.StatefulSet) (bool, string, string, error) {
	requiredVersionString, requiredVersionLabelPresent := required.Labels[naming.ScyllaVersionLabel]
	existingVersionString, existingVersionLabelPresent := existing.Labels[naming.ScyllaVersionLabel]

	if !requiredVersionLabelPresent || !existingVersionLabelPresent {
		return false, "", "", nil
	}

	requiredVersion, err := semver.Parse(requiredVersionString)
	if err != nil {
		return false, "", "", err
	}
	existingVersion, err := semver.Parse(existingVersionString)
	if err != nil {
		return false, "", "", err
	}

	if requiredVersion.Major != existingVersion.Major || requiredVersion.Minor != existingVersion.Minor {
		return true, existingVersionString, requiredVersionString, nil
	}

	return false, "", "", nil
}

// startUpgrade records a new upgrade context in its pre-hooks phase, which starts the upgrade state machine.
func (sdcc *Controller) startUpgrade(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, fromVersion, toVersion string) ([]metav1.Condition, error) {
	sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeNormal, "UpgradeStarted", "Version changed from %q to %q", fromVersion, toVersion)

	progressingConditions := []metav1.Condition{
		newStatefulSetProgressingCondition(sdc, reasonUpgrading, "Starting cluster upgrade"),
	}

	now := time.Now()
	applyConditions, err := sdcc.applyUpgradeContext(ctx, sdc, &internalapi.DatacenterUpgradeContext{
		State:             internalapi.PreHooksUpgradePhase,
		FromVersion:       fromVersion,
		ToVersion:         toVersion,
		SystemSnapshotTag: snapshotTag("system", now),
		DataSnapshotTag:   snapshotTag("data", now),
	})
	progressingConditions = append(progressingConditions, applyConditions...)

	return progressingConditions, err
}
