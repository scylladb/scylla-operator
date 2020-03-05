package util

import (
	"encoding/json"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

// UpgradeStatefulSetVersion attempts to set the image of a StatefulSet
func UpgradeStatefulSetVersion(sts *appsv1.StatefulSet, image string, kubeClient kubernetes.Interface) error {
	upgradeSts := sts.DeepCopy()
	idx := findContainerByName(upgradeSts, "scylla")
	if idx < 0 {
		return errors.New("error, can't find statefulset named 'scylla'")
	}
	upgradeSts.Spec.Template.Spec.Containers[idx].Image = image
	return PatchStatefulSet(sts, upgradeSts, kubeClient)
}

// ScaleStatefulSet attempts to scale a StatefulSet by the given amount
func ScaleStatefulSet(sts *appsv1.StatefulSet, amount int32, kubeClient kubernetes.Interface) error {
	updatedSts := sts.DeepCopy()
	updatedReplicas := *updatedSts.Spec.Replicas + amount
	if updatedReplicas < 0 {
		return errors.New("error, can't scale statefulset below 0 replicas")
	}
	updatedSts.Spec.Replicas = &updatedReplicas
	err := PatchStatefulSet(sts, updatedSts, kubeClient)
	return err
}

// PatchStatefulSet patches the old StatefulSet so that it matches the
// new StatefulSet.
func PatchStatefulSet(old, new *appsv1.StatefulSet, kubeClient kubernetes.Interface) error {

	oldJSON, err := json.Marshal(old)
	if err != nil {
		return errors.WithStack(err)
	}

	newJSON, err := json.Marshal(new)
	if err != nil {
		return errors.WithStack(err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, appsv1.StatefulSet{})
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = kubeClient.AppsV1().StatefulSets(old.Namespace).Patch(old.Name, types.StrategicMergePatchType, patchBytes)
	return errors.WithStack(err)
}

// PatchService patches the old Service so that it matches the
// new Service.
func PatchService(old, new *corev1.Service, kubeClient kubernetes.Interface) error {

	oldJSON, err := json.Marshal(old)
	if err != nil {
		return err
	}

	newJSON, err := json.Marshal(new)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, corev1.Service{})
	if err != nil {
		return err
	}

	_, err = kubeClient.CoreV1().Services(old.Namespace).Patch(old.Name, types.StrategicMergePatchType, patchBytes)
	return err
}

func findContainerByName(sts *appsv1.StatefulSet, name string) int {
	for i, container := range sts.Spec.Template.Spec.Containers {
		if container.Name == name {
			return i
		}
	}
	return -1
}
