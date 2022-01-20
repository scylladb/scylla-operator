package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type ServiceAccountControlInterface interface {
	PatchServiceAccount(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealServiceAccountControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ ServiceAccountControlInterface = &RealServiceAccountControl{}

func (r RealServiceAccountControl) PatchServiceAccount(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.CoreV1().ServiceAccounts(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type ServiceAccountControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx                   context.Context
	controllerGVK         schema.GroupVersionKind
	ServiceAccountControl ServiceAccountControlInterface
}

func NewServiceAccountControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	ServiceAccountControl ServiceAccountControlInterface,
) *ServiceAccountControllerRefManager {
	return &ServiceAccountControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:                   ctx,
		controllerGVK:         controllerGVK,
		ServiceAccountControl: ServiceAccountControl,
	}
}

func (m *ServiceAccountControllerRefManager) ClaimServiceAccounts(serviceAccounts []*corev1.ServiceAccount) (map[string]*corev1.ServiceAccount, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptServiceAccount(obj.(*corev1.ServiceAccount))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseServiceAccount(obj.(*corev1.ServiceAccount))
	}

	claimedMap := make(map[string]*corev1.ServiceAccount, len(serviceAccounts))
	var errors []error
	for _, serviceAccount := range serviceAccounts {
		ok, err := m.ClaimObject(serviceAccount, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[serviceAccount.Name] = serviceAccount
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *ServiceAccountControllerRefManager) AdoptServiceAccount(serviceAccount *corev1.ServiceAccount) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt ServiceAccount %v/%v (%v): %v", serviceAccount.Namespace, serviceAccount.Name, serviceAccount.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, serviceAccount.UID)
	if err != nil {
		return err
	}

	return m.ServiceAccountControl.PatchServiceAccount(m.ctx, serviceAccount.Namespace, serviceAccount.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *ServiceAccountControllerRefManager) ReleaseServiceAccount(serviceAccount *corev1.ServiceAccount) error {
	klog.V(2).InfoS("Patching ServiceAccount to remove its controllerRef",
		"ServiceAccount",
		klog.KObj(serviceAccount),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(serviceAccount.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.ServiceAccountControl.PatchServiceAccount(m.ctx, serviceAccount.Namespace, serviceAccount.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the ServiceAccount no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch ServiceAccount as it was missing", klog.KObj(serviceAccount))
			return nil
		}

		if apierrors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the ServiceAccount has no owner reference
			// 2. the UID of the ServiceAccount doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch ServiceAccount as it was invalid", klog.KObj(serviceAccount))
			return nil
		}

		return err
	}

	return nil
}
