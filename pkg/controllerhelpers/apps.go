package controllerhelpers

import (
	"fmt"

	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func IsStatefulSetRolledOut(sts *appsv1.StatefulSet) (bool, error) {
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		return false, fmt.Errorf("can't determine rollout status for %s strategy type", sts.Spec.UpdateStrategy.Type)
	}

	if sts.Spec.Replicas == nil {
		// This should never happen, but better safe then sorry.
		return false, fmt.Errorf("statefulset.spec.replicas can't be nil")
	}

	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return false, nil
	}

	if sts.Status.Replicas != *sts.Spec.Replicas {
		return false, nil
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return false, nil
	}

	if sts.Status.AvailableReplicas < *sts.Spec.Replicas {
		return false, nil
	}

	if sts.Spec.UpdateStrategy.RollingUpdate != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		return sts.Status.UpdatedReplicas == (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition), nil
	} else {
		return sts.Status.UpdateRevision == sts.Status.CurrentRevision, nil
	}
}

func IsDeploymentRolledOut(deploy *appsv1.Deployment) (bool, error) {
	if deploy.Spec.Replicas == nil {
		return false, fmt.Errorf("deployment.spec.replicas can't be nil")
	}

	if deploy.Status.ObservedGeneration == 0 || deploy.Generation > deploy.Status.ObservedGeneration {
		klog.V(4).InfoS("Observed generation not caught up yet", "Deployment", klog.KObj(deploy))
		return false, nil
	}

	if deploy.Status.UpdatedReplicas < *deploy.Spec.Replicas {
		klog.V(4).InfoS("Not all replicas are updated yet", "Deployment", klog.KObj(deploy))
		return false, nil
	}

	if deploy.Status.ReadyReplicas < *deploy.Spec.Replicas {
		klog.V(4).InfoS("Not all replicas are ready yet", "Deployment", klog.KObj(deploy))
		return false, nil
	}

	if deploy.Status.AvailableReplicas < *deploy.Spec.Replicas {
		klog.V(4).InfoS("Not all replicas are available yet", "Deployment", klog.KObj(deploy))
		return false, nil
	}

	_, _, availableConditionTrue := oslices.Find(deploy.Status.Conditions, func(condition appsv1.DeploymentCondition) bool {
		return condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue
	})
	if !availableConditionTrue {
		klog.V(4).InfoS("Available condition is not True", "Deployment", klog.KObj(deploy))
		return false, nil
	}

	klog.V(4).InfoS("Fully rolled out", "Deployment", klog.KObj(deploy))

	return true, nil
}

func IsDaemonSetRolledOut(ds *appsv1.DaemonSet) (bool, error) {
	if ds.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return false, fmt.Errorf("can't determine rollout status for %s strategy type", ds.Spec.UpdateStrategy.Type)
	}

	if ds.Status.ObservedGeneration == 0 || ds.Generation > ds.Status.ObservedGeneration {
		klog.V(4).InfoS("Observed generation not caught up yet", "DaemonSet", klog.KObj(ds))
		return false, nil
	}

	if ds.Status.UpdatedNumberScheduled < ds.Status.DesiredNumberScheduled {
		klog.V(4).InfoS("Not all pods are updated yet", "DaemonSet", klog.KObj(ds))
		return false, nil
	}

	if ds.Status.NumberAvailable < ds.Status.DesiredNumberScheduled {
		klog.V(4).InfoS("Not all pods are available yet", "DaemonSet", klog.KObj(ds))
		return false, nil
	}

	klog.V(4).InfoS("Fully rolled out", "DaemonSet", klog.KObj(ds))

	return true, nil
}
