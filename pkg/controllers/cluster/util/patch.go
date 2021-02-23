package util

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// UpgradeStatefulSetScyllaImage attempts to set the image of a StatefulSet
func UpgradeStatefulSetScyllaImage(ctx context.Context, sts *appsv1.StatefulSet, image string, kubeClient kubernetes.Interface) error {
	upgradeSts := sts.DeepCopy()
	idx, err := naming.FindScyllaContainer(upgradeSts.Spec.Template.Spec.Containers)
	if err != nil {
		return errors.WithStack(err)
	}
	upgradeSts.Spec.Template.Spec.Containers[idx].Image = image
	return PatchStatefulSet(ctx, sts, upgradeSts, kubeClient)
}

// ScaleStatefulSet attempts to scale a StatefulSet by the given amount
func ScaleStatefulSet(ctx context.Context, sts *appsv1.StatefulSet, amount int32, kubeClient kubernetes.Interface) error {
	updatedSts := sts.DeepCopy()
	updatedReplicas := *updatedSts.Spec.Replicas + amount
	if updatedReplicas < 0 {
		return errors.New("error, can't scale statefulset below 0 replicas")
	}
	updatedSts.Spec.Replicas = &updatedReplicas
	err := PatchStatefulSet(ctx, sts, updatedSts, kubeClient)
	return err
}

// ScaleScyllaContainerResources attempts to scale a StatefulSet's Scylla-container's resources
func ScaleScyllaContainerResources(ctx context.Context, sts *appsv1.StatefulSet, newRequirements corev1.ResourceRequirements, kubeClient kubernetes.Interface, c *scyllav1.ScyllaCluster, r record.EventRecorder) error {
	updatedSts := sts.DeepCopy()
	idx, err := naming.FindScyllaContainer(sts.Spec.Template.Spec.Containers)
	if err != nil {
		r.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf("Error trying to find container named scylla to scale it's resources."))
		return errors.WithStack(err)
	}
	updatedSts.Spec.Template.Spec.Containers[idx].Resources.Limits = newRequirements.Limits
	updatedSts.Spec.Template.Spec.Containers[idx].Resources.Requests = newRequirements.Requests
	err = PatchStatefulSet(ctx, sts, updatedSts, kubeClient)
	return err
}

// PatchStatefulSet patches the old StatefulSet so that it matches the
// new StatefulSet.
func PatchStatefulSet(ctx context.Context, old, new *appsv1.StatefulSet, kubeClient kubernetes.Interface) error {

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

	_, err = kubeClient.AppsV1().StatefulSets(old.Namespace).Patch(ctx, old.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	return errors.WithStack(err)
}

// PatchService patches the old Service so that it matches the
// new Service.
func PatchService(ctx context.Context, old, new *corev1.Service, kubeClient kubernetes.Interface) error {

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

	_, err = kubeClient.CoreV1().Services(old.Namespace).Patch(ctx, old.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

// The StrategicMergePatchFunc type is an adapter to allow the use of ordinary functions as Strategic Merge Patch.
type StrategicMergePatchFunc func(obj runtime.Object) ([]byte, error)

// Type returns PatchType of Strategic Merge Patch
func (f StrategicMergePatchFunc) Type() types.PatchType {
	return types.StrategicMergePatchType
}

// Data returns the raw data representing the patch.
func (f StrategicMergePatchFunc) Data(obj runtime.Object) ([]byte, error) {
	return f(obj)
}
