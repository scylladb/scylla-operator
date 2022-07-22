package managerv2

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	v1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

func (smc *Controller) updateStatus(ctx context.Context, sm *v1alpha1.ScyllaManager, status *v1alpha1.ScyllaManagerStatus) error {
	if apiequality.Semantic.DeepEqual(&sm.Status, status) {
		return nil
	}

	smCopy := sm.DeepCopy()
	smCopy.Status = *status

	klog.V(2).InfoS("Updating status", "ScyllaManager", klog.KObj(smCopy))

	_, err := smc.scyllaClient.ScyllaManagers(sm.Namespace).UpdateStatus(ctx, smCopy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "ScyllaManager", klog.KObj(smCopy))

	return nil
}

func (smc *Controller) calculateStatus(
	sm *v1alpha1.ScyllaManager,
	deployments map[string]*v1.Deployment,
	clusters []*scyllav1.ScyllaCluster,
	syncedClusters []*managerclient.Cluster,
) *v1alpha1.ScyllaManagerStatus {
	status := sm.Status.DeepCopy()
	status.ObservedGeneration = pointer.Int64(sm.Generation)

	status = smc.calculateManagerConditions(status, sm, deployments)
	status = smc.calculateStatusDeployment(status, sm, deployments)
	status = smc.calculateClustersStatuses(status, clusters, syncedClusters)

	return status
}

func (smc *Controller) calculateStatusDeployment(status *v1alpha1.ScyllaManagerStatus, sm *v1alpha1.ScyllaManager, deployments map[string]*v1.Deployment) *v1alpha1.ScyllaManagerStatus {
	if deployment, ok := deployments[sm.Name]; ok {
		status.Replicas = deployment.Status.Replicas
		status.ReadyReplicas = deployment.Status.ReadyReplicas
		status.UnavailableReplicas = deployment.Status.UnavailableReplicas
		status.UpdatedReplicas = deployment.Status.UpdatedReplicas

	}
	return status
}

func (smc *Controller) calculateManagerConditions(status *v1alpha1.ScyllaManagerStatus, sm *v1alpha1.ScyllaManager, deployments map[string]*v1.Deployment) *v1alpha1.ScyllaManagerStatus {
	status.Conditions = []metav1.Condition{
		smc.calculateAvailableCondition(status, sm, deployments),
		smc.calculateDegradedCondition(status, sm, deployments),
	}

	return status
}

func (smc *Controller) calculateAvailableCondition(
	status *v1alpha1.ScyllaManagerStatus,
	sm *v1alpha1.ScyllaManager,
	deployments map[string]*v1.Deployment,
) metav1.Condition {
	newCond := metav1.Condition{
		Type:               v1alpha1.ScyllaManagerAvailable,
		ObservedGeneration: sm.Generation,
	}

	if deployment, ok := deployments[sm.Name]; ok {
		if deployment.Status.ReadyReplicas > 0 {
			newCond.Status = metav1.ConditionTrue
			newCond.Reason = "AsExpected"
			return smc.calculateConditionLastTransition(status.Conditions, newCond)
		}
	}

	newCond.Status = metav1.ConditionFalse
	newCond.Reason = "NoAvailableDeployments"
	newCond.Message = "There are not any available Scylla Manager replicas"
	return smc.calculateConditionLastTransition(status.Conditions, newCond)
}

func (smc *Controller) calculateDegradedCondition(
	status *v1alpha1.ScyllaManagerStatus,
	sm *v1alpha1.ScyllaManager,
	deployments map[string]*v1.Deployment,
) metav1.Condition {
	newCond := metav1.Condition{
		Type:               v1alpha1.ScyllaManagerDegraded,
		ObservedGeneration: sm.Generation,
	}

	if deployment, ok := deployments[sm.Name]; ok {
		if deployment.Status.Replicas != deployment.Status.UpdatedReplicas {
			newCond.Status = metav1.ConditionTrue
			newCond.Reason = "SomeSmAreOutOfDate"
			newCond.Message = fmt.Sprintf("%v outdated Scylla Managers", deployment.Status.Replicas-deployment.Status.UpdatedReplicas)
			return smc.calculateConditionLastTransition(status.Conditions, newCond)
		}
	}

	newCond.Status = metav1.ConditionFalse
	newCond.Reason = "AsExpected"
	return smc.calculateConditionLastTransition(status.Conditions, newCond)
}

func (smc *Controller) calculateConditionLastTransition(conditions []metav1.Condition, newCond metav1.Condition) metav1.Condition {
	for _, cond := range conditions {
		if cond.Type == newCond.Type {
			if cond.Status != newCond.Status {
				newCond.LastTransitionTime = metav1.Now()
			} else {
				newCond.LastTransitionTime = cond.LastTransitionTime
			}
			return newCond
		}
	}

	newCond.LastTransitionTime = metav1.Now()
	return newCond
}

func (smc *Controller) calculateClusterConditionLastTransition(status *v1alpha1.ScyllaManagerStatus, cluster string, newCond metav1.Condition) metav1.Condition {
	for _, mc := range status.ManagedClusters {
		if mc.Name == cluster {
			return smc.calculateConditionLastTransition(mc.Conditions, newCond)
		}
	}

	return smc.calculateConditionLastTransition(nil, newCond)
}

func (smc *Controller) calculateClustersStatuses(
	status *v1alpha1.ScyllaManagerStatus,
	clusters []*scyllav1.ScyllaCluster,
	syncedClusters []*managerclient.Cluster,
) *v1alpha1.ScyllaManagerStatus {
	managedClusters := make([]*v1alpha1.ManagedCluster, 0, len(clusters))
	for _, c := range clusters {
		mc := &v1alpha1.ManagedCluster{
			Name: naming.ManagerClusterName(c),
		}
		found := false
		for _, sc := range syncedClusters {
			if mc.Name == sc.Name {
				mc.ID = sc.ID
				found = true
			}
		}
		if found {
			cond := smc.calculateClusterConditionLastTransition(status, mc.Name,
				metav1.Condition{
					Type:               v1alpha1.ClusterRegistered,
					Status:             metav1.ConditionTrue,
					Reason:             "AsExpected",
					ObservedGeneration: c.Generation,
				})
			mc.Conditions = []metav1.Condition{cond}
		} else {
			cond := smc.calculateClusterConditionLastTransition(status, mc.Name,
				metav1.Condition{
					Type:               v1alpha1.ClusterRegistered,
					Status:             metav1.ConditionFalse,
					Reason:             "ClusterNotRegistered",
					ObservedGeneration: c.Generation,
				})
			mc.Conditions = []metav1.Condition{cond}
		}

		managedClusters = append(managedClusters, mc)
	}

	status.ManagedClusters = managedClusters
	return status
}
