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
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	scyllasemver "github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	"github.com/scylladb/scylla-operator/pkg/util/parallel"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	setsutil "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

var systemKeyspaces = []string{"system", "system_schema"}

func snapshotTag(prefix string, t time.Time) string {
	return fmt.Sprintf("so_%s_%sUTC", prefix, t.UTC().Format(time.RFC3339))
}

func (sdcc *Controller) makeRacks(sdc *scyllav1alpha1.ScyllaDBDatacenter, statefulSets map[string]*appsv1.StatefulSet, inputsHash string) ([]*appsv1.StatefulSet, error) {
	sets := make([]*appsv1.StatefulSet, 0, len(sdc.Spec.Racks))
	for i, rack := range sdc.Spec.Racks {
		oldSts := statefulSets[naming.StatefulSetNameForRack(rack, sdc)]
		sts, err := StatefulSetForRack(rack, sdc, oldSts, sdcc.operatorImage, i, inputsHash)
		if err != nil {
			return nil, err
		}

		sets = append(sets, sts)
	}
	return sets, nil
}

func (sdcc *Controller) getScyllaManagerAgentToken(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter) (string, error) {
	secretName := naming.AgentAuthTokenSecretName(sdc)
	secret, err := sdcc.secretLister.Secrets(sdc.Namespace).Get(secretName)
	if err != nil {
		return "", fmt.Errorf("can't get manager agent auth secret %s/%s: %w", sdc.Namespace, secretName, err)
	}

	token, err := helpers.GetAgentAuthTokenFromSecret(secret)
	if err != nil {
		return "", fmt.Errorf("can't get agent token from secret %s: %w", naming.ObjRef(secret), err)
	}

	return token, nil
}

func (sdcc *Controller) getScyllaClient(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, hosts []string) (*scyllaclient.Client, error) {
	managerAgentAuthToken, err := sdcc.getScyllaManagerAgentToken(ctx, sdc)
	if err != nil {
		return nil, fmt.Errorf("can't get manager agent auth token: %w", err)
	}

	client, err := controllerhelpers.NewScyllaClientFromToken(hosts, managerAgentAuthToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (sdcc *Controller) backupKeyspaces(ctx context.Context, scyllaClient *scyllaclient.Client, hosts, keyspaces []string, snapshotTag string) error {
	return parallel.ForEach(len(hosts), func(i int) error {
		host := hosts[i]

		snapshots, err := scyllaClient.Snapshots(ctx, host)
		if err != nil {
			return fmt.Errorf("can't list snapshots on host %q: %w", host, err)
		}

		if slices.ContainsItem(snapshots, snapshotTag) {
			return nil
		}

		for _, keyspace := range keyspaces {
			err := scyllaClient.TakeSnapshot(ctx, host, snapshotTag, keyspace)
			if err != nil {
				return fmt.Errorf("can't take a snapshot on host %q and keyspace %q: %w", host, keyspace, err)
			}
		}

		return nil
	})
}

func (sdcc *Controller) removeSnapshot(ctx context.Context, scyllaClient *scyllaclient.Client, hosts, snapshotTags []string) error {
	return parallel.ForEach(len(hosts), func(i int) error {
		host := hosts[i]

		snapshots, err := scyllaClient.Snapshots(ctx, host)
		if err != nil {
			return fmt.Errorf("can't list snapshots on host %q: %w", host, err)
		}

		snapshotSet := setsutil.NewString(snapshots...)
		for _, snapshotTag := range snapshotTags {
			if !snapshotSet.Has(snapshotTag) {
				continue
			}

			err := scyllaClient.DeleteSnapshot(ctx, host, snapshotTag)
			if err != nil {
				return fmt.Errorf("can't delete snapshot %q on host %q: %w", snapshotTag, host, err)
			}
		}

		return nil
	})
}

// beforeUpgrade runs hooks before a cluster upgrade starts.
// It returns true if the action is done, false if the caller should repeat later.
func (sdcc *Controller) beforeUpgrade(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service, upgradeContext *internalapi.DatacenterUpgradeContext) (bool, error) {
	klog.V(2).InfoS("Running pre-upgrade hook", "ScyllaDBDatacenter", klog.KObj(sdc))
	defer klog.V(2).InfoS("Finished running pre-upgrade hook", "ScyllaDBDatacenter", klog.KObj(sdc))

	hosts, err := controllerhelpers.GetRequiredScyllaHosts(sdc, services, sdcc.podLister)
	if err != nil {
		return true, err
	}

	scyllaClient, err := sdcc.getScyllaClient(ctx, sdc, hosts)
	if err != nil {
		return true, err
	}
	defer scyllaClient.Close()

	klog.V(4).InfoS("Checking schema agreement", "ScyllaDBDatacenter", klog.KObj(sdc))
	hasSchemaAgreement, err := scyllaClient.HasSchemaAgreement(ctx)
	if err != nil {
		return true, fmt.Errorf("awaiting schema agreement: %w", err)
	}

	if !hasSchemaAgreement {
		klog.V(4).InfoS("Schema is not agreed yet, will retry.", "ScyllaDBDatacenter", klog.KObj(sdc))
		return false, nil
	}
	klog.V(4).InfoS("Schema agreed", "ScyllaDBDatacenter", klog.KObj(sdc))

	// Snapshot system tables.

	klog.V(4).InfoS("Backing up system keyspaces", "ScyllaDBDatacenter", klog.KObj(sdc))
	err = sdcc.backupKeyspaces(ctx, scyllaClient, hosts, systemKeyspaces, upgradeContext.SystemSnapshotTag)
	if err != nil {
		return true, err
	}
	klog.V(4).InfoS("Backed up system keyspaces", "ScyllaDBDatacenter", klog.KObj(sdc))

	return true, nil
}

func (sdcc *Controller) afterUpgrade(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service, upgradeContext *internalapi.DatacenterUpgradeContext) error {
	klog.V(2).InfoS("Running post-upgrade hook", "ScyllaDBDatacenter", klog.KObj(sdc))
	defer klog.V(2).InfoS("Finished running post-upgrade hook", "ScyllaDBDatacenter", klog.KObj(sdc))

	hosts, err := controllerhelpers.GetRequiredScyllaHosts(sdc, services, sdcc.podLister)
	if err != nil {
		return err
	}

	scyllaClient, err := sdcc.getScyllaClient(ctx, sdc, hosts)
	if err != nil {
		return err
	}
	defer scyllaClient.Close()

	// Clear system backup.
	err = sdcc.removeSnapshot(ctx, scyllaClient, hosts, []string{upgradeContext.SystemSnapshotTag})
	if err != nil {
		return err
	}

	return nil
}

// beforeNodeUpgrade runs hooks before a node upgrade.
// It returns true if the action is done, false if the caller should repeat later.
func (sdcc *Controller) beforeNodeUpgrade(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, sts *appsv1.StatefulSet, ordinal int32, services map[string]*corev1.Service, upgradeContext *internalapi.DatacenterUpgradeContext) (bool, error) {
	klog.V(2).InfoS("Running node pre-upgrade hook", "ScyllaDBDatacenter", klog.KObj(sdc))
	defer klog.V(2).InfoS("Finished running node pre-upgrade hook", "ScyllaDBDatacenter", klog.KObj(sdc))

	// Make sure node is marked as under maintenance so liveness checks won't fail during drain.
	svcName := fmt.Sprintf("%s-%d", sts.Name, ordinal)
	svc, ok := services[svcName]
	if !ok {
		return true, fmt.Errorf("missing service %s/%s", sdc.Namespace, svcName)
	}

	// Enable maintenance mode to make sure liveness checks won't fail.
	_, err := sdcc.kubeClient.CoreV1().Services(svc.Namespace).Patch(
		ctx,
		svc.Name,
		types.StrategicMergePatchType,
		[]byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": ""}}}`, naming.NodeMaintenanceLabel)),
		metav1.PatchOptions{},
	)
	if err != nil {
		return true, err
	}

	// Drain the node.
	podName := naming.PodNameFromService(svc)
	pod, err := sdcc.podLister.Pods(sdc.Namespace).Get(podName)
	if err != nil {
		return false, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
	}

	host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
	if err != nil {
		return true, err
	}

	scyllaClient, err := sdcc.getScyllaClient(ctx, sdc, []string{host})
	if err != nil {
		return true, err
	}
	defer scyllaClient.Close()

	om, err := scyllaClient.OperationMode(ctx, host)
	if err != nil {
		return true, err
	}

	if om.IsDraining() {
		klog.V(4).InfoS("Waiting for scylla node to finish draining", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", host)
		return false, nil
	}

	if !om.IsDrained() {
		klog.V(4).InfoS("Draining scylla node", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", host)
		err = scyllaClient.Drain(ctx, host)
		if err != nil {
			return true, err
		}
		klog.V(4).InfoS("Drained scylla node", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", host)
	}

	// Create data backup.

	allKeyspaces, err := scyllaClient.Keyspaces(ctx)
	if err != nil {
		return true, fmt.Errorf("can't list keyspaces for host %q: %w", host, err)
	}

	keyspaceSet := setsutil.NewString(allKeyspaces...)
	keyspaceSet.Delete(systemKeyspaces...)
	klog.V(4).InfoS("Backing up data keyspaces", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", host)
	err = sdcc.backupKeyspaces(ctx, scyllaClient, []string{host}, keyspaceSet.List(), upgradeContext.DataSnapshotTag)
	if err != nil {
		return true, err
	}
	klog.V(4).InfoS("Backed up data keyspaces", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", host)

	// Disable maintenance mode.
	_, err = sdcc.kubeClient.CoreV1().Services(svc.Namespace).Patch(
		ctx,
		svc.Name,
		types.StrategicMergePatchType,
		[]byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": null}}}`, naming.NodeMaintenanceLabel)),
		metav1.PatchOptions{},
	)
	if err != nil {
		return true, err
	}

	// Because we've drained the node, it can never come back to being ready. Unfortunately, there is a bug in Kubernetes
	// StatefulSet controller that won't update a broken StatefulSet, so we need to delete the pod manually.
	// https://github.com/kubernetes/kubernetes/issues/67250
	// Kubernetes can't evict pods when DesiredHealthy == 0 and it's already down, so we need to use DELETE
	// to succeed even when having just one replica.
	klog.V(2).InfoS("Deleting Pod", "ScyllaDBDatacenter", klog.KObj(sdc), "Pod", naming.ManualRef(sdc.Namespace, podName))
	err = sdcc.kubeClient.CoreV1().Pods(sdc.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return true, fmt.Errorf("can't delete pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
		}

		klog.V(3).InfoS("Pod already deleted", "ScyllaDBDatacenter", klog.KObj(sdc), "Pod", naming.ManualRef(sdc.Namespace, podName))
	} else {
		klog.V(2).InfoS("Pod deleted", "ScyllaDBDatacenter", klog.KObj(sdc), "Pod", naming.ManualRef(sdc.Namespace, podName))
	}

	return true, nil
}

func (sdcc *Controller) afterNodeUpgrade(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, sts *appsv1.StatefulSet, ordinal int32, services map[string]*corev1.Service, upgradeContext *internalapi.DatacenterUpgradeContext) error {
	svcName := fmt.Sprintf("%s-%d", sts.Name, ordinal)
	svc, ok := services[svcName]
	if !ok {
		return fmt.Errorf("missing service %q", naming.ManualRef(sdc.Namespace, svcName))
	}

	podName := naming.PodNameFromService(svc)
	pod, err := sdcc.podLister.Pods(sdc.Namespace).Get(podName)
	if err != nil {
		return fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
	}

	host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
	if err != nil {
		return err
	}

	scyllaClient, err := sdcc.getScyllaClient(ctx, sdc, []string{host})
	if err != nil {
		return err
	}
	defer scyllaClient.Close()

	// Clear data backup.
	err = sdcc.removeSnapshot(ctx, scyllaClient, []string{host}, []string{upgradeContext.DataSnapshotTag})
	if err != nil {
		return err
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

		status.Racks = slices.FilterOut(status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
			return rackStatus.Name == rackName
		})
	}
	return progressingConditions, utilerrors.NewAggregate(errs)
}

// createMissingStatefulSets creates missing StatefulSets.
// It return true if done and an error.
func (sdcc *Controller) createMissingStatefulSets(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	requiredStatefulSets []*appsv1.StatefulSet,
	statefulSets map[string]*appsv1.StatefulSet,
	services map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition
	for _, req := range requiredStatefulSets {
		klog.V(4).InfoS("Processing required StatefulSet", "StatefulSet", klog.KObj(req))
		// Check the adopted set.
		sts, found := statefulSets[req.Name]
		if !found {
			klog.V(2).InfoS("Creating missing StatefulSet", "StatefulSet", klog.KObj(req))
			var changed bool
			var err error
			sts, changed, err = resourceapply.ApplyStatefulSet(ctx, sdcc.kubeClient.AppsV1(), sdcc.statefulSetLister, sdcc.eventRecorder, req, resourceapply.ApplyOptions{})
			if err != nil {
				errs = append(errs, fmt.Errorf("can't create missing statefulset: %w", err))
				continue
			}
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, req, "apply", sdc.Generation)

				rackName, ok := sts.Labels[naming.RackNameLabel]
				if !ok {
					errs = append(errs, fmt.Errorf(
						"can't determine rack name: statefulset %s is missing label %q",
						naming.ObjRef(sts),
						naming.RackNameLabel),
					)
					continue
				}

				updatedRackStatus := *sdcc.calculateRackStatus(sdc, sts)
				_, idx, ok := slices.Find(status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
					return rackStatus.Name == rackName
				})
				if ok {
					status.Racks[idx] = updatedRackStatus
				} else {
					status.Racks = append(status.Racks, updatedRackStatus)
				}
			}
		} else {
			// When we decommission a member there is a pod left that's not ready until we scale.
			if req.Spec.Replicas != nil && sts.Spec.Replicas != nil &&
				*req.Spec.Replicas != *sts.Spec.Replicas {
				continue
			}
		}

		// Wait for the StatefulSet to roll out. Racks can only bootstrap one by one.
		rolledOut, err := controllerhelpers.IsStatefulSetRolledOut(sts)
		if err != nil {
			return progressingConditions, err
		}

		if !rolledOut {
			klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               statefulSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForStatefulSetRollout",
				Message:            fmt.Sprintf("Waiting for StatefulSet %q to roll out.", naming.ObjRef(req)),
				ObservedGeneration: sdc.Generation,
			})
			return progressingConditions, nil
		}
	}

	return progressingConditions, utilerrors.NewAggregate(errs)
}

func (sdcc *Controller) syncStatefulSets(
	ctx context.Context,
	key string,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	statefulSets map[string]*appsv1.StatefulSet,
	services map[string]*corev1.Service,
	configMaps map[string]*corev1.ConfigMap,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	managedScyllaDBConfigCMName := naming.GetScyllaDBManagedConfigCMName(sdc.Name)
	managedScyllaDBConfigCM, found := configMaps[managedScyllaDBConfigCMName]
	if !found {
		klog.V(2).InfoS("Waiting for managed config map", "ScyllaDBDatacenter", klog.KObj(sdc), "ConfigMapName", managedScyllaDBConfigCMName)
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               statefulSetControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForManagedConfig",
			Message:            fmt.Sprintf("Waiting for ConfigMap %q to be created.", managedScyllaDBConfigCMName),
			ObservedGeneration: sdc.Generation,
		})
		return progressingConditions, nil
	}

	inputsHash, err := hash.HashObjects(managedScyllaDBConfigCM.Data)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't hash inputs: %w", err)
	}

	requiredStatefulSets, err := sdcc.makeRacks(sdc, statefulSets, inputsHash)
	if err != nil {
		sdcc.eventRecorder.Eventf(
			sdc,
			corev1.EventTypeWarning,
			"InvalidRack",
			fmt.Sprintf("Failed to make rack: %v", err),
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

	// Before any update, make sure all StatefulSets are present.
	// Create any that are missing.
	createProgressingConditions, err := sdcc.createMissingStatefulSets(ctx, sdc, status, requiredStatefulSets, statefulSets, services)
	progressingConditions = append(progressingConditions, createProgressingConditions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't create StatefulSet(s): %w", err)
	}
	if len(createProgressingConditions) > 0 {
		// Wait for the informers to catch up.
		// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
		time.Sleep(artificialDelayForCachesToCatchUp)
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
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForRackServiceDecommission",
					Message:            fmt.Sprintf("Waiting for rack service %q to decommission.", naming.ObjRef(svc)),
					ObservedGeneration: sdc.Generation,
				})

				return progressingConditions, nil
			}

			requiredAnnotationsBeforeScaling := []string{
				// We need to ensure token ring annotation is noticed by the cleanup logic before we scale the rack.
				// Otherwise, new node could start joining, changing the ring hash and causing cleanup to be missed.
				naming.LastCleanedUpTokenRingHashAnnotation,
			}

			for _, requiredAnnotation := range requiredAnnotationsBeforeScaling {
				ord, err := naming.IndexFromName(svc.Name)
				if err != nil {
					return nil, fmt.Errorf("can't determine ordinal from Service name %q: %w", svc.Name, err)
				}

				if ord < *sts.Spec.Replicas {
					_, ok := svc.Annotations[requiredAnnotation]
					if !ok {
						progressingConditions = append(progressingConditions, metav1.Condition{
							Type:               statefulSetControllerProgressingCondition,
							Status:             metav1.ConditionTrue,
							Reason:             "WaitingForServiceState",
							Message:            fmt.Sprintf("Statusfulset %q is waiting for Service %q to have required annotation %q before scaling", naming.ObjRef(req), naming.ObjRef(svc), requiredAnnotation),
							ObservedGeneration: sdc.Generation,
						})
					}
				}
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
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForMissingService",
					Message:            fmt.Sprintf("Statusfulset %q is waiting for service %q to be created", naming.ObjRef(req), lastSvcName),
					ObservedGeneration: sdc.Generation,
				})
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

		rolledOut, err := controllerhelpers.IsStatefulSetRolledOut(sts)
		if err != nil {
			return progressingConditions, err
		}

		if !rolledOut {
			klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               statefulSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForStatefulSetRollout",
				Message:            fmt.Sprintf("Waiting for StatefulSet %q to roll out.", naming.ObjRef(req)),
				ObservedGeneration: sdc.Generation,
			})
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

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               statefulSetControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "RunningUpgradeHooks",
			Message:            "Running upgrade hooks",
			ObservedGeneration: sdc.Generation,
		})

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

					rackName, ok := updatedSts.Labels[naming.RackNameLabel]
					if !ok {
						errs = append(errs, fmt.Errorf(
							"can't determine rack name: statefulset %s is missing label %q",
							naming.ObjRef(updatedSts),
							naming.RackNameLabel),
						)
						continue
					}

					_, idx, ok := slices.Find(sdc.Status.Racks, func(status scyllav1alpha1.RackStatus) bool {
						return status.Name == rackName
					})
					if !ok {
						errs = append(errs, fmt.Errorf("can't find rack %q status in %q ScyllaDBDatacenter", rackName, naming.ObjRef(sdc)))
						continue
					}

					status.Racks[idx] = *sdcc.calculateRackStatus(sdc, updatedSts)
				}
			}
			if anyStsChanged {
				// TODO: Add expectations, not to reconcile sooner then we see this new StatefulSet in our caches. (#682)
				time.Sleep(artificialDelayForCachesToCatchUp)
			}
			err = utilerrors.NewAggregate(errs)
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

				if partition <= 0 {
					continue
				}

				nextPartition := partition - 1

				klog.V(4).InfoS("Upgrade is running a rollout", "Partition", partition, "NextPartition", nextPartition)

				if partition < *sts.Spec.Replicas {
					// TODO: Move the post-node-upgrade hook into a Job.
					err = sdcc.afterNodeUpgrade(ctx, sdc, sts, partition, services, currentUpgradeContext)
					if err != nil {
						return progressingConditions, err
					}
					klog.V(2).InfoS("AfterNodeUpgrade hook finished", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts))
				}

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
			time.Sleep(artificialDelayForCachesToCatchUp)
		}
	}()
	for _, required := range requiredStatefulSets {
		// Check for version upgrades first.
		existing, existingFound := statefulSets[required.Name]
		if existingFound && upgradeContextConfigMap == nil {
			requiredVersionString, requiredVersionLabelPresent := required.Labels[naming.ScyllaVersionLabel]
			existingVersionString, existingVersionLabelPresent := existing.Labels[naming.ScyllaVersionLabel]

			if requiredVersionLabelPresent && existingVersionLabelPresent {
				parsedSemRequiredVersion, parseErr := semver.Parse(requiredVersionString)
				if parseErr != nil {
					semRequiredVersion, _, getVersionErr := scyllasemver.GetImageVersionAndDigest("scylla", requiredVersionString)
					if getVersionErr != nil {
						return progressingConditions, getVersionErr
					}

					parsedSemRequiredVersion, err = semver.Parse(semRequiredVersion)
					if err != nil {
						return progressingConditions, err
					}
				}

				requiredVersion := parsedSemRequiredVersion

				parsedSemExistingVersion, parseErr := semver.Parse(existingVersionString)
				if parseErr != nil {
					semExistingVersion, _, getVersionErr := scyllasemver.GetImageVersionAndDigest("scylla", existingVersionString)
					if getVersionErr != nil {
						return progressingConditions, getVersionErr
					}

					parsedSemExistingVersion, err = semver.Parse(semExistingVersion)
					if err != nil {
						return progressingConditions, err
					}
				}

				existingVersion := parsedSemExistingVersion

				if requiredVersion.Major != existingVersion.Major ||
					requiredVersion.Minor != existingVersion.Minor {
					// We need to run hooks for version upgrades.
					sdcc.eventRecorder.Eventf(sdc, corev1.EventTypeNormal, "UpgradeStarted", "Version changed from %q to %q", existingVersionString, requiredVersionString)

					progressingConditions = append(progressingConditions, metav1.Condition{
						Type:               statefulSetControllerProgressingCondition,
						Status:             metav1.ConditionTrue,
						Reason:             "Upgrading",
						Message:            "Starting cluster upgrade",
						ObservedGeneration: sdc.Generation,
					})

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

			rackName, ok := updatedSts.Labels[naming.RackNameLabel]
			if !ok {
				return progressingConditions, fmt.Errorf(
					"can't determine rack name: statefulset %s is missing label %q",
					naming.ObjRef(updatedSts),
					naming.RackNameLabel,
				)
			}
			_, idx, ok := slices.Find(sdc.Status.Racks, func(status scyllav1alpha1.RackStatus) bool {
				return status.Name == rackName
			})
			if !ok {
				return progressingConditions, fmt.Errorf("can't find rack %q status in %q ScyllaDBDatacenter", rackName, naming.ObjRef(sdc))
			}

			status.Racks[idx] = *sdcc.calculateRackStatus(sdc, updatedSts)
		}

		// Wait for the StatefulSet to roll out.
		rolledOut, err := controllerhelpers.IsStatefulSetRolledOut(updatedSts)
		if err != nil {
			return progressingConditions, err
		}

		if !rolledOut {
			klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(updatedSts))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               statefulSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForStatefulSetRollout",
				Message:            fmt.Sprintf("Waiting for StatefulSet %q to roll out.", naming.ObjRef(required)),
				ObservedGeneration: sdc.Generation,
			})
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

		rackStatus, _, found := slices.Find(status.Racks, func(status scyllav1alpha1.RackStatus) bool {
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
