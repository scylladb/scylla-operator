package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// The StatefulSet sync reconciles the rack StatefulSets of a ScyllaDBDatacenter. It is a pipeline of steps run in a
// fixed order by syncStatefulSets. Every step returns the progressing conditions it produced; returning any condition
// stops the pipeline until the next requeue. That is how racks are created, scaled and updated one at a time.
//
// Every step lives in its own sync_statefulsets_<step>.go file, reading top-down from the step to what it calls. The
// helpers shared by the steps are in sync_statefulsets_helpers.go.

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

// statefulSetSyncContext carries the inputs shared by the steps of the StatefulSet sync. The steps run one after
// another, so a step sees what the previous ones wrote.
//
// Everything but status is read-only: the objects come from the informer caches or are shared with the following
// steps, so they must not be modified. Changes to the cluster go through the API server and are observed by the next
// sync.
type statefulSetSyncContext struct {
	// key is the work queue key of the ScyllaDBDatacenter, used to requeue it with a delay. Read-only.
	key string

	// sdc is the ScyllaDBDatacenter being synced. Read-only.
	sdc *scyllav1alpha1.ScyllaDBDatacenter

	// status is the status of the ScyllaDBDatacenter being computed in this sync; it is written to the API server once
	// all the syncs are done. It is writable: a step that deletes, creates or updates a StatefulSet updates the
	// corresponding rack status right away (see updateRackStatus) so that the recorded status reflects the change
	// before the informer caches catch up. Conditions are only read from it.
	status *scyllav1alpha1.ScyllaDBDatacenterStatus

	// requiredStatefulSets are the StatefulSets the ScyllaDBDatacenter calls for, in rack order. Read-only: a step that
	// needs to apply a variation of one, e.g. with a partition set, applies a copy.
	requiredStatefulSets []*appsv1.StatefulSet

	// existingStatefulSets are the StatefulSets owned by the ScyllaDBDatacenter, by name. Read-only.
	existingStatefulSets map[string]*appsv1.StatefulSet

	// services are the Services owned by the ScyllaDBDatacenter, by name. Read-only.
	services map[string]*corev1.Service

	// configMaps are the ConfigMaps owned by the ScyllaDBDatacenter, by name. Read-only.
	configMaps map[string]*corev1.ConfigMap
}

// statefulSetSyncStep is one step of the StatefulSet sync. Returning any progressing condition stops the sync until
// the next requeue.
type statefulSetSyncStep struct {
	name string
	run  func(ctx context.Context, sc *statefulSetSyncContext) ([]metav1.Condition, error)
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
