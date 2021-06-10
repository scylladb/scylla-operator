package util

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

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
