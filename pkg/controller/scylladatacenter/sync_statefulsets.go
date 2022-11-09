// Copyright (c) 2022 ScyllaDB.

package scylladatacenter

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
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/parallel"
	"github.com/scylladb/scylla-operator/pkg/util/slices"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	setsutil "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

type UpgradePhase string

const (
	PreHooksUpgradePhase    UpgradePhase = "PreHooks"
	RolloutInitUpgradePhase UpgradePhase = "RolloutInit"
	RolloutRunUpgradePhase  UpgradePhase = "RolloutRun"
	PostHooksUpgradePhase   UpgradePhase = "PostHooks"
)

var systemKeyspaces = []string{"system", "system_schema"}

func snapshotTag(prefix string, t time.Time) string {
	return fmt.Sprintf("so_%s_%sUTC", prefix, t.UTC().Format(time.RFC3339))
}

func (sdc *Controller) makeRacks(sd *scyllav1alpha1.ScyllaDatacenter, statefulSets map[string]*appsv1.StatefulSet) ([]*appsv1.StatefulSet, error) {
	sets := make([]*appsv1.StatefulSet, 0, len(sd.Spec.Racks))
	for _, rack := range sd.Spec.Racks {
		oldSts := statefulSets[naming.StatefulSetNameForRack(rack, sd)]
		sts, err := StatefulSetForRack(rack, sd, oldSts, sdc.operatorImage)
		if err != nil {
			return nil, err
		}

		sets = append(sets, sts)
	}
	return sets, nil
}

func (sdc *Controller) getScyllaManagerAgentToken(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (string, error) {
	secretName := naming.AgentAuthTokenSecretName(sd.Name)
	secret, err := sdc.secretLister.Secrets(sd.Namespace).Get(secretName)
	if err != nil {
		return "", fmt.Errorf("can't get manager agent auth secret %s/%s: %w", sd.Namespace, secretName, err)
	}

	token, err := helpers.GetAgentAuthTokenFromSecret(secret)
	if err != nil {
		return "", fmt.Errorf("can't get agent token from secret %s: %w", naming.ObjRef(secret), err)
	}

	return token, nil
}

func (sdc *Controller) getScyllaClient(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter, hosts []string) (*scyllaclient.Client, error) {
	managerAgentAuthToken, err := sdc.getScyllaManagerAgentToken(ctx, sd)
	if err != nil {
		return nil, fmt.Errorf("can't get manager agent auth token: %w", err)
	}

	client, err := controllerhelpers.NewScyllaClientFromToken(hosts, managerAgentAuthToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (sdc *Controller) backupKeyspaces(ctx context.Context, scyllaClient *scyllaclient.Client, hosts, keyspaces []string, snapshotTag string) error {
	return parallel.ForEach(len(hosts), func(i int) error {
		host := hosts[i]

		snapshots, err := scyllaClient.Snapshots(ctx, host)
		if err != nil {
			return fmt.Errorf("can't list snapshots on host %q: %w", host, err)
		}

		if slices.ContainsString(snapshotTag, snapshots) {
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

func (sdc *Controller) removeSnapshot(ctx context.Context, scyllaClient *scyllaclient.Client, hosts, snapshotTags []string) error {
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
func (sdc *Controller) beforeUpgrade(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter, services map[string]*corev1.Service) (bool, error) {
	klog.V(2).InfoS("Running pre-upgrade hook", "ScyllaDatacenter", klog.KObj(sd))
	defer klog.V(2).InfoS("Finished running pre-upgrade hook", "ScyllaDatacenter", klog.KObj(sd))

	hosts, err := controllerhelpers.GetRequiredScyllaHosts(sd, services)
	if err != nil {
		return true, err
	}

	scyllaClient, err := sdc.getScyllaClient(ctx, sd, hosts)
	if err != nil {
		return true, err
	}
	defer scyllaClient.Close()

	klog.V(4).InfoS("Checking schema agreement", "ScyllaDatacenter", klog.KObj(sd))
	hasSchemaAgreement, err := scyllaClient.HasSchemaAgreement(ctx)
	if err != nil {
		return true, fmt.Errorf("awaiting schema agreement: %w", err)
	}

	if !hasSchemaAgreement {
		klog.V(4).InfoS("Schema is not agreed yet, will retry.", "ScyllaDatacenter", klog.KObj(sd))
		return false, nil
	}
	klog.V(4).InfoS("Schema agreed", "ScyllaDatacenter", klog.KObj(sd))

	// Snapshot system tables.

	klog.V(4).InfoS("Backing up system keyspaces", "ScyllaDatacenter", klog.KObj(sd))
	err = sdc.backupKeyspaces(ctx, scyllaClient, hosts, systemKeyspaces, sd.Status.Upgrade.SystemSnapshotTag)
	if err != nil {
		return true, err
	}
	klog.V(4).InfoS("Backed up system keyspaces", "ScyllaDatacenter", klog.KObj(sd))

	return true, nil
}

func (sdc *Controller) afterUpgrade(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter, services map[string]*corev1.Service) error {
	klog.V(2).InfoS("Running post-upgrade hook", "ScyllaDatacenter", klog.KObj(sd))
	defer klog.V(2).InfoS("Finished running post-upgrade hook", "ScyllaDatacenter", klog.KObj(sd))

	hosts, err := controllerhelpers.GetRequiredScyllaHosts(sd, services)
	if err != nil {
		return err
	}

	scyllaClient, err := sdc.getScyllaClient(ctx, sd, hosts)
	if err != nil {
		return err
	}
	defer scyllaClient.Close()

	// Clear system backup.
	err = sdc.removeSnapshot(ctx, scyllaClient, hosts, []string{sd.Status.Upgrade.SystemSnapshotTag})
	if err != nil {
		return err
	}

	return nil
}

// beforeNodeUpgrade runs hooks before a node upgrade.
// It returns true if the action is done, false if the caller should repeat later.
func (sdc *Controller) beforeNodeUpgrade(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter, sts *appsv1.StatefulSet, ordinal int32, services map[string]*corev1.Service) (bool, error) {
	klog.V(2).InfoS("Running node pre-upgrade hook", "ScyllaDatacenter", klog.KObj(sd))
	defer klog.V(2).InfoS("Finished running node pre-upgrade hook", "ScyllaDatacenter", klog.KObj(sd))

	// Make sure node is marked as under maintenance so liveness checks won't fail during drain.
	svcName := fmt.Sprintf("%s-%d", sts.Name, ordinal)
	svc, ok := services[svcName]
	if !ok {
		return true, fmt.Errorf("missing service %s/%s", sd.Namespace, svcName)
	}

	// Enable maintenance mode to make sure liveness checks won't fail.
	_, err := sdc.kubeClient.CoreV1().Services(svc.Namespace).Patch(
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

	host, err := controllerhelpers.GetScyllaIPFromService(svc)
	if err != nil {
		return true, err
	}

	scyllaClient, err := sdc.getScyllaClient(ctx, sd, []string{host})
	if err != nil {
		return true, err
	}
	defer scyllaClient.Close()

	om, err := scyllaClient.OperationMode(ctx, host)
	if err != nil {
		return true, err
	}

	if om.IsDraining() {
		klog.V(4).InfoS("Waiting for scylla node to finish draining", "ScyllaDatacenter", klog.KObj(sd), "Host", host)
		return false, nil
	}

	if !om.IsDrained() {
		klog.V(4).InfoS("Draining scylla node", "ScyllaDatacenter", klog.KObj(sd), "Host", host)
		err = scyllaClient.Drain(ctx, host)
		if err != nil {
			return true, err
		}
		klog.V(4).InfoS("Drained scylla node", "ScyllaDatacenter", klog.KObj(sd), "Host", host)
	}

	// Create data backup.

	allKeyspaces, err := scyllaClient.Keyspaces(ctx)
	if err != nil {
		return true, fmt.Errorf("can't list keyspaces for host %q: %w", host, err)
	}

	keyspaceSet := setsutil.NewString(allKeyspaces...)
	keyspaceSet.Delete(systemKeyspaces...)
	klog.V(4).InfoS("Backing up data keyspaces", "ScyllaDatacenter", klog.KObj(sd), "Host", host)
	err = sdc.backupKeyspaces(ctx, scyllaClient, []string{host}, keyspaceSet.List(), sd.Status.Upgrade.DataSnapshotTag)
	if err != nil {
		return true, err
	}
	klog.V(4).InfoS("Backed up data keyspaces", "ScyllaDatacenter", klog.KObj(sd), "Host", host)

	// Disable maintenance mode.
	_, err = sdc.kubeClient.CoreV1().Services(svc.Namespace).Patch(
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
	podName := svcName
	klog.V(2).InfoS("Deleting Pod", "ScyllaDatacenter", klog.KObj(sd), "Pod", naming.ManualRef(sd.Namespace, podName))
	err = sdc.kubeClient.CoreV1().Pods(sd.Namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return true, fmt.Errorf("can't delete pod %q: %w", naming.ManualRef(sd.Namespace, podName), err)
		}

		klog.V(3).InfoS("Pod already deleted", "ScyllaDatacenter", klog.KObj(sd), "Pod", naming.ManualRef(sd.Namespace, podName))
	} else {
		klog.V(2).InfoS("Pod deleted", "ScyllaDatacenter", klog.KObj(sd), "Pod", naming.ManualRef(sd.Namespace, podName))
	}

	return true, nil
}

func (sdc *Controller) afterNodeUpgrade(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter, sts *appsv1.StatefulSet, ordinal int32, services map[string]*corev1.Service) error {
	host, err := controllerhelpers.GetScyllaHost(sts.Name, ordinal, services)
	if err != nil {
		return err
	}

	scyllaClient, err := sdc.getScyllaClient(ctx, sd, []string{host})
	if err != nil {
		return err
	}
	defer scyllaClient.Close()

	// Clear data backup.
	err = sdc.removeSnapshot(ctx, scyllaClient, []string{host}, []string{sd.Status.Upgrade.DataSnapshotTag})
	if err != nil {
		return err
	}

	return nil
}

func (sdc *Controller) pruneStatefulSets(
	ctx context.Context,
	sd *scyllav1alpha1.ScyllaDatacenter,
	status *scyllav1alpha1.ScyllaDatacenterStatus,
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
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, sts, "delete", sd.Generation)
		err := sdc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Delete(ctx, sts.Name, metav1.DeleteOptions{
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

		delete(status.Racks, rackName)
	}
	return progressingConditions, utilerrors.NewAggregate(errs)
}

// createMissingStatefulSets creates missing StatefulSets.
// It return true if done and an error.
func (sdc *Controller) createMissingStatefulSets(
	ctx context.Context,
	sd *scyllav1alpha1.ScyllaDatacenter,
	status *scyllav1alpha1.ScyllaDatacenterStatus,
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
			sts, changed, err = resourceapply.ApplyStatefulSet(ctx, sdc.kubeClient.AppsV1(), sdc.statefulSetLister, sdc.eventRecorder, req, false)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't create missing statefulset: %w", err))
				continue
			}
			if changed {
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, req, "apply", sd.Generation)

				rackName, ok := sts.Labels[naming.RackNameLabel]
				if !ok {
					errs = append(errs, fmt.Errorf(
						"can't determine rack name: statefulset %s is missing label %q",
						naming.ObjRef(sts),
						naming.RackNameLabel),
					)
					continue
				}
				oldRackStatus := sd.Status.Racks[rackName]
				status.Racks[rackName] = *sdc.calculateRackStatus(sd, rackName, sts, &oldRackStatus, services)
			}
		} else {
			// When we decommission a member there is a pod left that's not ready until we scale.
			if req.Spec.Replicas != nil && sts.Spec.Replicas != nil &&
				*req.Spec.Replicas != *sts.Spec.Replicas {
				continue
			}
		}

		// Wait for the StatefulSet to rollout. Racks can only bootstrap one by one.
		rolledOut, err := controllerhelpers.IsStatefulSetRolledOut(sts)
		if err != nil {
			return progressingConditions, err
		}

		if !rolledOut {
			klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               statefulSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForStatefulSetRollout",
				Message:            fmt.Sprintf("Waiting for StatefulSet %q to rollout.", naming.ObjRef(req)),
				ObservedGeneration: sd.Generation,
			})
			return progressingConditions, nil
		}
	}

	return progressingConditions, utilerrors.NewAggregate(errs)
}

func (sdc *Controller) syncStatefulSets(
	ctx context.Context,
	key string,
	sd *scyllav1alpha1.ScyllaDatacenter,
	status *scyllav1alpha1.ScyllaDatacenterStatus,
	statefulSets map[string]*appsv1.StatefulSet,
	services map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var err error
	var progressingConditions []metav1.Condition

	requiredStatefulSets, err := sdc.makeRacks(sd, statefulSets)
	if err != nil {
		sdc.eventRecorder.Eventf(
			sd,
			corev1.EventTypeWarning,
			"InvalidRack",
			fmt.Sprintf("Failed to make rack: %v", err),
		)
		return progressingConditions, err
	}

	// Delete any excessive StatefulSets.
	// Delete has to be the first action to avoid getting stuck on quota.
	pruneProgressingConditions, err := sdc.pruneStatefulSets(ctx, sd, status, requiredStatefulSets, statefulSets)
	progressingConditions = append(progressingConditions, pruneProgressingConditions...)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete StatefulSet(s): %w", err)
	}

	// Before any update, make sure all StatefulSets are present.
	// Create any that are missing.
	createProgressingConditions, err := sdc.createMissingStatefulSets(ctx, sd, status, requiredStatefulSets, statefulSets, services)
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
					ObservedGeneration: sd.Generation,
				})

				rackName, ok := sts.Labels[naming.RackNameLabel]
				if ok && len(rackName) != 0 {
					rackStatus := status.Racks[rackName]
					meta.SetStatusCondition(&rackStatus.Conditions, metav1.Condition{
						Status:  metav1.ConditionTrue,
						Type:    scyllav1alpha1.RackConditionTypeNodeDecommissioning,
						Reason:  "DecommissionInProgress",
						Message: "Node in rack is decommissioning",
					})
					status.Racks[rackName] = rackStatus
				} else {
					klog.Warningf("Can't set decommissioning condition sts %s/%s because it's missing rack label.", sts.Namespace, sts.Name)
				}

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
				klog.V(4).InfoS("Missing service", "ScyllaDatacenter", klog.KObj(sd), "ServiceName", lastSvcName)
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               statefulSetControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForMissingService",
					Message:            fmt.Sprintf("Statusfulset %q is waiting for service %q to be created", naming.ObjRef(req), lastSvcName),
					ObservedGeneration: sd.Generation,
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
				controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, lastSvcCopy, "update", sd.Generation)
				_, err := sdc.kubeClient.CoreV1().Services(lastSvcCopy.Namespace).Update(ctx, lastSvcCopy, metav1.UpdateOptions{})
				if err != nil {
					return progressingConditions, err
				}
				return progressingConditions, nil
			}
		}

		klog.V(2).InfoS("Scaling StatefulSet", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts), "CurrentReplicas", *sts.Spec.Replicas, "UpdatedReplicas", scale.Spec.Replicas)
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, scale, "updateScale", sd.Generation)
		_, err = sdc.kubeClient.AppsV1().StatefulSets(sts.Namespace).UpdateScale(ctx, sts.Name, scale, metav1.UpdateOptions{})
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
			klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               statefulSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForStatefulSetRollout",
				Message:            fmt.Sprintf("Waiting for StatefulSet %q to rollout.", naming.ObjRef(req)),
				ObservedGeneration: sd.Generation,
			})
			return progressingConditions, nil
		}
	}

	// Run hooks if an upgrade is in progress.
	if status.Upgrade != nil {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               statefulSetControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "RunningUpgradeHooks",
			Message:            "Running upgrade hooks",
			ObservedGeneration: sd.Generation,
		})

		// Isolate the live values in a block to prevent accidental use.
		{
			// We could still see an old status. Although hooks are mandated to be reentrant,
			// they are pretty expensive to run so it's cheaper to recheck the partition with a live call.
			// TODO: Remove the live call when the hooks are migrated to run as Jobs.
			freshSD, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
			if err != nil {
				return progressingConditions, err
			}

			if freshSD.Status.Upgrade == nil ||
				freshSD.Status.Upgrade.State != status.Upgrade.State {
				// Wait for requeue.
				klog.V(2).InfoS("Stale upgrade status, waiting for requeue", "ScyllaDatacenter", sd)
				return progressingConditions, err
			}
		}

		klog.V(4).InfoS("Upgrade is in progress", "Phase", status.Upgrade.State)
		switch status.Upgrade.State {
		case string(PreHooksUpgradePhase):
			// TODO: Move the pre-upgrade hook into a Job.
			done, err := sdc.beforeUpgrade(ctx, sd, services)
			if err != nil {
				return progressingConditions, err
			}
			if !done {
				sdc.queue.AddAfter(key, 5*time.Second)
				return progressingConditions, nil
			}

			status.Upgrade.State = string(RolloutInitUpgradePhase)
			return progressingConditions, nil

		case string(RolloutInitUpgradePhase):
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
				required.Spec.Replicas = pointer.Int32Ptr(*existing.Spec.Replicas)
				required.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Int32Ptr(*existing.Spec.Replicas)
				// Use apply to also update the spec.template
				updatedSts, changed, err := resourceapply.ApplyStatefulSet(ctx, sdc.kubeClient.AppsV1(), sdc.statefulSetLister, sdc.eventRecorder, required, false)
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
					oldRackStatus := sd.Status.Racks[rackName]
					status.Racks[rackName] = *sdc.calculateRackStatus(sd, rackName, updatedSts, &oldRackStatus, services)
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

			status.Upgrade.State = string(RolloutRunUpgradePhase)
			return progressingConditions, nil

		case string(RolloutRunUpgradePhase):
			for _, sts := range requiredStatefulSets {
				partition := *sts.Spec.UpdateStrategy.RollingUpdate.Partition

				// Isolate the live values in a block to prevent accidental use.
				{
					// TODO: Remove the live call when hooks are migrated into Jobs.
					// We could still see an old partition. Although hooks are mandated to be reentrant,
					// they are pretty expensive to run so it's cheaper to recheck the partition with a live call.
					freshSts, err := sdc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
					if err != nil {
						return progressingConditions, err
					}

					if freshSts.Spec.UpdateStrategy.RollingUpdate == nil ||
						*freshSts.Spec.UpdateStrategy.RollingUpdate.Partition != partition {
						// Wait for requeue.
						klog.V(2).InfoS("Stale StatefulSet partition, waiting for requeue", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts))
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
					err = sdc.afterNodeUpgrade(ctx, sd, sts, partition, services)
					if err != nil {
						return progressingConditions, err
					}
					klog.V(2).InfoS("AfterNodeUpgrade hook finished", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts))
				}

				// TODO: Move the pre-node-upgrade hook into a Job.
				done, err := sdc.beforeNodeUpgrade(ctx, sd, sts, nextPartition, services)
				if err != nil {
					return progressingConditions, err
				}

				if !done {
					klog.V(4).InfoS("PreNodeUpgrade hook in progress. Waiting a bit.", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts))
					sdc.queue.AddAfter(key, 5*time.Second)
					return progressingConditions, nil
				}
				klog.V(2).InfoS("PreNodeUpgrade hook finished", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(sts))

				// TODO: Use bare update when hooks are extracted into Jobs.
				//       But at this point rerunning them is expensive so we retry with condition check.
				err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					freshSts, err := sdc.kubeClient.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
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

					freshSts.Spec.UpdateStrategy.RollingUpdate.Partition = pointer.Int32Ptr(nextPartition)
					_, err = sdc.kubeClient.AppsV1().StatefulSets(freshSts.Namespace).Update(ctx, freshSts, metav1.UpdateOptions{})
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

			status.Upgrade.State = string(PostHooksUpgradePhase)
			return progressingConditions, nil

		case string(PostHooksUpgradePhase):
			err = sdc.afterUpgrade(ctx, sd, services)
			if err != nil {
				return progressingConditions, err
			}

			status.Upgrade = nil
			return progressingConditions, nil

		default:
			// An old cluster with an old state machine can still be going through an update, or stuck.
			// Given have to be reentrant we'll just start again to be sure no step is missed, even a new one.
			klog.Warning("ScyllaDatacenter %q has an unknown upgrade phase %q. Resetting the phase.", klog.KObj(sd), status.Upgrade.State)
			status.Upgrade.State = string(PreHooksUpgradePhase)
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
		if existingFound && status.Upgrade == nil {
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
					sdc.eventRecorder.Eventf(sd, corev1.EventTypeNormal, "UpgradeStarted", "Version changed from %q to %q", existingVersionString, requiredVersionString)

					progressingConditions = append(progressingConditions, metav1.Condition{
						Type:               statefulSetControllerProgressingCondition,
						Status:             metav1.ConditionTrue,
						Reason:             "Upgrading",
						Message:            "Starting cluster upgrade",
						ObservedGeneration: sd.Generation,
					})

					// Initiate the upgrade. This triggers a state machine to run hooks first.
					now := time.Now()
					status.Upgrade = &scyllav1alpha1.UpgradeStatus{
						State:             string(PreHooksUpgradePhase),
						FromVersion:       existingVersionString,
						ToVersion:         requiredVersionString,
						SystemSnapshotTag: snapshotTag("system", now),
						DataSnapshotTag:   snapshotTag("data", now),
					}
					return progressingConditions, nil
				}
			}
		}

		updatedSts, changed, err := resourceapply.ApplyStatefulSet(ctx, sdc.kubeClient.AppsV1(), sdc.statefulSetLister, sdc.eventRecorder, required, false)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't apply statefulset update: %w", err)
		}

		if changed {
			anyStsChanged = true

			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, required, "apply", sd.Generation)

			rackName, ok := updatedSts.Labels[naming.RackNameLabel]
			if !ok {
				return progressingConditions, fmt.Errorf(
					"can't determine rack name: statefulset %s is missing label %q",
					naming.ObjRef(updatedSts),
					naming.RackNameLabel,
				)
			}
			oldRackStatus := sd.Status.Racks[rackName]
			status.Racks[rackName] = *sdc.calculateRackStatus(sd, rackName, updatedSts, &oldRackStatus, services)
		}

		// Wait for the StatefulSet to rollout.
		rolledOut, err := controllerhelpers.IsStatefulSetRolledOut(updatedSts)
		if err != nil {
			return progressingConditions, err
		}

		if !rolledOut {
			klog.V(4).InfoS("Waiting for StatefulSet rollout", "ScyllaDatacenter", klog.KObj(sd), "StatefulSet", klog.KObj(updatedSts))
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               statefulSetControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForStatefulSetRollout",
				Message:            fmt.Sprintf("Waiting for StatefulSet %q to rollout.", naming.ObjRef(required)),
				ObservedGeneration: sd.Generation,
			})
			return progressingConditions, nil
		}
	}

	return progressingConditions, nil
}

func (scc *Controller) setStatefulSetsAvailableStatusCondition(
	sc *scyllav1alpha1.ScyllaDatacenter,
	status *scyllav1alpha1.ScyllaDatacenterStatus,
) {
	desiredMembers := int32(0)
	updatedMembers := int32(0)
	readyMembers := int32(0)
	var racksInDifferentVersion []string
	for _, rack := range sc.Spec.Racks {
		if rack.Nodes == nil {
			continue
		}

		desiredMembers += *rack.Nodes

		rackStatus, found := status.Racks[rack.Name]
		if !found {
			klog.Errorf("Can't find status for rack %q", rack.Name)
			continue
		}

		if rackStatus.Image != sc.Spec.Scylla.Image {
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
			ObservedGeneration: sc.Generation,
		})

	case updatedMembers != desiredMembers:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "MembersNotUpdated",
			Message:            fmt.Sprintf("Only %d out of %d member(s) have been updated", updatedMembers, desiredMembers),
			ObservedGeneration: sc.Generation,
		})

	case readyMembers != desiredMembers:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionFalse,
			Reason:             "MembersNotReady",
			Message:            fmt.Sprintf("Only %d out of %d member(s) are ready", readyMembers, desiredMembers),
			ObservedGeneration: sc.Generation,
		})

	default:
		apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:               statefulSetControllerAvailableCondition,
			Status:             metav1.ConditionTrue,
			Reason:             internalapi.AsExpectedReason,
			Message:            "",
			ObservedGeneration: sc.Generation,
		})
	}

	return
}
