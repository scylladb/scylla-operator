package scylladbdatacenter

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/parallel"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

// This file holds the upgrade hooks: the actions run against the ScyllaDB nodes before and after a datacenter upgrade
// and before and after every node upgrade. They talk to the ScyllaDB API and are the only part of the StatefulSet sync
// that does.
//
// TODO: Move the hooks into Jobs so that they don't run inside the controller.

var systemKeyspaces = []string{"system", "system_schema"}

func snapshotTag(prefix string, t time.Time) string {
	return fmt.Sprintf("so_%s_%sUTC", prefix, t.UTC().Format(time.RFC3339))
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

	client, err := sdcc.newScyllaDBClient(hosts, managerAgentAuthToken)
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

		if oslices.ContainsItem(snapshots, snapshotTag) {
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

		snapshotSet := apimachineryutilsets.NewString(snapshots...)
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

// upgradeNode identifies a single ScyllaDB node targeted by the node upgrade hooks.
type upgradeNode struct {
	service *corev1.Service
	pod     *corev1.Pod
	host    string
}

// resolveUpgradeNode looks up the member Service, the Pod and the ScyllaDB host of the node at the given ordinal of
// the StatefulSet.
func (sdcc *Controller) resolveUpgradeNode(sdc *scyllav1alpha1.ScyllaDBDatacenter, sts *appsv1.StatefulSet, ordinal int32, services map[string]*corev1.Service) (*upgradeNode, error) {
	svcName := fmt.Sprintf("%s-%d", sts.Name, ordinal)
	svc, ok := services[svcName]
	if !ok {
		return nil, fmt.Errorf("missing service %q", naming.ManualRef(sdc.Namespace, svcName))
	}

	podName := naming.PodNameFromService(svc)
	pod, err := sdcc.podLister.Pods(sdc.Namespace).Get(podName)
	if err != nil {
		return nil, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err)
	}

	host, err := controllerhelpers.GetScyllaHost(sdc, svc, pod)
	if err != nil {
		return nil, err
	}

	return &upgradeNode{
		service: svc,
		pod:     pod,
		host:    host,
	}, nil
}

// setNodeMaintenanceMode toggles the maintenance label on the member Service. A node under maintenance doesn't fail
// its liveness checks, e.g. while it is drained.
func (sdcc *Controller) setNodeMaintenanceMode(ctx context.Context, svc *corev1.Service, enabled bool) error {
	labelValue := "null"
	if enabled {
		labelValue = `""`
	}

	_, err := sdcc.kubeClient.CoreV1().Services(svc.Namespace).Patch(
		ctx,
		svc.Name,
		types.StrategicMergePatchType,
		[]byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": %s}}}`, naming.NodeMaintenanceLabel, labelValue)),
		metav1.PatchOptions{},
	)
	if err != nil {
		return err
	}

	return nil
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

	node, err := sdcc.resolveUpgradeNode(sdc, sts, ordinal, services)
	if err != nil {
		return true, err
	}

	// Make sure node is marked as under maintenance so liveness checks won't fail during drain.
	err = sdcc.setNodeMaintenanceMode(ctx, node.service, true)
	if err != nil {
		return true, err
	}

	scyllaClient, err := sdcc.getScyllaClient(ctx, sdc, []string{node.host})
	if err != nil {
		return true, err
	}
	defer scyllaClient.Close()

	// Drain the node.
	om, err := scyllaClient.OperationMode(ctx, node.host)
	if err != nil {
		return true, err
	}

	if om.IsDraining() {
		klog.V(4).InfoS("Waiting for scylla node to finish draining", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", node.host)
		return false, nil
	}

	if !om.IsDrained() {
		klog.V(4).InfoS("Draining scylla node", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", node.host)
		err = scyllaClient.Drain(ctx, node.host)
		if err != nil {
			return true, err
		}
		klog.V(4).InfoS("Drained scylla node", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", node.host)
	}

	// Create data backup.

	allKeyspaces, err := scyllaClient.Keyspaces(ctx)
	if err != nil {
		return true, fmt.Errorf("can't list keyspaces for host %q: %w", node.host, err)
	}

	keyspaceSet := apimachineryutilsets.NewString(allKeyspaces...)
	keyspaceSet.Delete(systemKeyspaces...)
	klog.V(4).InfoS("Backing up data keyspaces", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", node.host)
	err = sdcc.backupKeyspaces(ctx, scyllaClient, []string{node.host}, keyspaceSet.List(), upgradeContext.DataSnapshotTag)
	if err != nil {
		return true, err
	}
	klog.V(4).InfoS("Backed up data keyspaces", "ScyllaDBDatacenter", klog.KObj(sdc), "Host", node.host)

	err = sdcc.setNodeMaintenanceMode(ctx, node.service, false)
	if err != nil {
		return true, err
	}

	// Because we've drained the node, it can never come back to being ready. Unfortunately, there is a bug in Kubernetes
	// StatefulSet controller that won't update a broken StatefulSet, so we need to delete the pod manually.
	// https://github.com/kubernetes/kubernetes/issues/67250
	// Kubernetes can't evict pods when DesiredHealthy == 0 and it's already down, so we need to use DELETE
	// to succeed even when having just one replica.
	podRef := naming.ObjRef(node.pod)
	klog.V(2).InfoS("Deleting Pod", "ScyllaDBDatacenter", klog.KObj(sdc), "Pod", podRef)
	err = sdcc.kubeClient.CoreV1().Pods(node.pod.Namespace).Delete(ctx, node.pod.Name, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return true, fmt.Errorf("can't delete pod %q: %w", podRef, err)
		}

		klog.V(3).InfoS("Pod already deleted", "ScyllaDBDatacenter", klog.KObj(sdc), "Pod", podRef)
	} else {
		klog.V(2).InfoS("Pod deleted", "ScyllaDBDatacenter", klog.KObj(sdc), "Pod", podRef)
	}

	return true, nil
}

func (sdcc *Controller) afterNodeUpgrade(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, sts *appsv1.StatefulSet, ordinal int32, services map[string]*corev1.Service, upgradeContext *internalapi.DatacenterUpgradeContext) error {
	node, err := sdcc.resolveUpgradeNode(sdc, sts, ordinal, services)
	if err != nil {
		return err
	}

	scyllaClient, err := sdcc.getScyllaClient(ctx, sdc, []string{node.host})
	if err != nil {
		return err
	}
	defer scyllaClient.Close()

	// Clear data backup.
	err = sdcc.removeSnapshot(ctx, scyllaClient, []string{node.host}, []string{upgradeContext.DataSnapshotTag})
	if err != nil {
		return err
	}

	return nil
}
