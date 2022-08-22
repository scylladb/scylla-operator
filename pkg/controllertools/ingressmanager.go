package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	networkingv1 "k8s.io/api/networking/v1"
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

type IngressControlInterface interface {
	PatchIngress(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealIngressControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ IngressControlInterface = &RealIngressControl{}

func (r RealIngressControl) PatchIngress(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.NetworkingV1().Ingresses(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type IngressControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx            context.Context
	controllerGVK  schema.GroupVersionKind
	ingressControl IngressControlInterface
}

func NewIngressControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	IngressControl IngressControlInterface,
) *IngressControllerRefManager {
	return &IngressControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:            ctx,
		controllerGVK:  controllerGVK,
		ingressControl: IngressControl,
	}
}

func (m *IngressControllerRefManager) ClaimIngresss(ingresses []*networkingv1.Ingress) (map[string]*networkingv1.Ingress, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptIngress(obj.(*networkingv1.Ingress))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseIngress(obj.(*networkingv1.Ingress))
	}

	claimedMap := make(map[string]*networkingv1.Ingress, len(ingresses))
	var errors []error
	for _, ingress := range ingresses {
		ok, err := m.ClaimObject(ingress, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[ingress.Name] = ingress
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *IngressControllerRefManager) AdoptIngress(ingress *networkingv1.Ingress) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt Ingress %v/%v (%v): %w", ingress.Namespace, ingress.Name, ingress.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, ingress.UID)
	if err != nil {
		return err
	}

	return m.ingressControl.PatchIngress(m.ctx, ingress.Namespace, ingress.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *IngressControllerRefManager) ReleaseIngress(ingress *networkingv1.Ingress) error {
	klog.V(2).InfoS("Patching Ingress to remove its controllerRef",
		"Ingress",
		klog.KObj(ingress),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(ingress.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.ingressControl.PatchIngress(m.ctx, ingress.Namespace, ingress.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Ingress no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch Ingress as it was missing", klog.KObj(ingress))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the Ingress has no owner reference
			// 2. the UID of the Ingress doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch Ingress as it was invalid", klog.KObj(ingress))
			return nil
		}

		return err
	}

	return nil
}
