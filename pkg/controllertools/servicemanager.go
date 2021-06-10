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

type ServiceControlInterface interface {
	PatchService(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealServiceControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ ServiceControlInterface = &RealServiceControl{}

func (r RealServiceControl) PatchService(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.CoreV1().Services(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type ServiceControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx            context.Context
	controllerGVK  schema.GroupVersionKind
	ServiceControl ServiceControlInterface
}

func NewServiceControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	ServiceControl ServiceControlInterface,
) *ServiceControllerRefManager {
	return &ServiceControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:            ctx,
		controllerGVK:  controllerGVK,
		ServiceControl: ServiceControl,
	}
}

func (m *ServiceControllerRefManager) ClaimServices(services []*corev1.Service) (map[string]*corev1.Service, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptService(obj.(*corev1.Service))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseService(obj.(*corev1.Service))
	}

	claimedMap := make(map[string]*corev1.Service, len(services))
	var errors []error
	for _, service := range services {
		ok, err := m.ClaimObject(service, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[service.Name] = service
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *ServiceControllerRefManager) AdoptService(service *corev1.Service) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt Service %v/%v (%v): %v", service.Namespace, service.Name, service.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, service.UID)
	if err != nil {
		return err
	}

	return m.ServiceControl.PatchService(m.ctx, service.Namespace, service.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *ServiceControllerRefManager) ReleaseService(service *corev1.Service) error {
	klog.V(2).InfoS("Patching Service to remove its controllerRef",
		"Service",
		klog.KObj(service),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(service.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.ServiceControl.PatchService(m.ctx, service.Namespace, service.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Service no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch Service as it was missing", klog.KObj(service))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the Service has no owner reference
			// 2. the UID of the Service doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch Service as it was invalid", klog.KObj(service))
			return nil
		}

		return err
	}

	return nil
}
