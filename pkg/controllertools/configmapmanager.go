package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type ConfigMapControlInterface interface {
	PatchConfigMap(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealConfigMapControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ ConfigMapControlInterface = &RealConfigMapControl{}

func (r RealConfigMapControl) PatchConfigMap(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.CoreV1().ConfigMaps(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type ConfigMapControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx              context.Context
	controllerGVK    schema.GroupVersionKind
	ConfigMapControl ConfigMapControlInterface
}

func NewConfigMapControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	ConfigMapControl ConfigMapControlInterface,
) *ConfigMapControllerRefManager {
	return &ConfigMapControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:              ctx,
		controllerGVK:    controllerGVK,
		ConfigMapControl: ConfigMapControl,
	}
}

func (m *ConfigMapControllerRefManager) ClaimConfigMaps(configMaps []*corev1.ConfigMap) (map[string]*corev1.ConfigMap, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptConfigMap(obj.(*corev1.ConfigMap))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseConfigMap(obj.(*corev1.ConfigMap))
	}

	claimedMap := make(map[string]*corev1.ConfigMap, len(configMaps))
	var errors []error
	for _, configMap := range configMaps {
		ok, err := m.ClaimObject(configMap, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[configMap.Name] = configMap
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *ConfigMapControllerRefManager) AdoptConfigMap(configMap *corev1.ConfigMap) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt ConfigMap %v/%v (%v): %v", configMap.Namespace, configMap.Name, configMap.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, configMap.UID)
	if err != nil {
		return err
	}

	return m.ConfigMapControl.PatchConfigMap(m.ctx, configMap.Namespace, configMap.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *ConfigMapControllerRefManager) ReleaseConfigMap(configMap *corev1.ConfigMap) error {
	klog.V(2).InfoS("Patching ConfigMap to remove its controllerRef",
		"ConfigMap",
		klog.KObj(configMap),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(configMap.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.ConfigMapControl.PatchConfigMap(m.ctx, configMap.Namespace, configMap.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the ConfigMap no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch ConfigMap as it was missing", klog.KObj(configMap))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the ConfigMap has no owner reference
			// 2. the UID of the ConfigMap doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch ConfigMap as it was invalid", klog.KObj(configMap))
			return nil
		}

		return err
	}

	return nil
}
