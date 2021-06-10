package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
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

type PodDisruptionBudgetControlInterface interface {
	PatchPodDisruptionBudget(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealPodDisruptionBudgetControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ PodDisruptionBudgetControlInterface = &RealPodDisruptionBudgetControl{}

func (r RealPodDisruptionBudgetControl) PatchPodDisruptionBudget(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.PolicyV1beta1().PodDisruptionBudgets(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type PodDisruptionBudgetControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx                        context.Context
	controllerGVK              schema.GroupVersionKind
	PodDisruptionBudgetControl PodDisruptionBudgetControlInterface
}

func NewPodDisruptionBudgetControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	PodDisruptionBudgetControl PodDisruptionBudgetControlInterface,
) *PodDisruptionBudgetControllerRefManager {
	return &PodDisruptionBudgetControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:                        ctx,
		controllerGVK:              controllerGVK,
		PodDisruptionBudgetControl: PodDisruptionBudgetControl,
	}
}

func (m *PodDisruptionBudgetControllerRefManager) ClaimPodDisruptionBudgets(podDisruptionBudgets []*policyv1beta1.PodDisruptionBudget) (map[string]*policyv1beta1.PodDisruptionBudget, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptPodDisruptionBudget(obj.(*policyv1beta1.PodDisruptionBudget))
	}
	release := func(obj metav1.Object) error {
		return m.ReleasePodDisruptionBudget(obj.(*policyv1beta1.PodDisruptionBudget))
	}

	claimedMap := make(map[string]*policyv1beta1.PodDisruptionBudget, len(podDisruptionBudgets))
	var errors []error
	for _, podDisruptionBudget := range podDisruptionBudgets {
		ok, err := m.ClaimObject(podDisruptionBudget, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[podDisruptionBudget.Name] = podDisruptionBudget
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *PodDisruptionBudgetControllerRefManager) AdoptPodDisruptionBudget(podDisruptionBudget *policyv1beta1.PodDisruptionBudget) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt PodDisruptionBudget %v/%v (%v): %v", podDisruptionBudget.Namespace, podDisruptionBudget.Name, podDisruptionBudget.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, podDisruptionBudget.UID)
	if err != nil {
		return err
	}

	return m.PodDisruptionBudgetControl.PatchPodDisruptionBudget(m.ctx, podDisruptionBudget.Namespace, podDisruptionBudget.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *PodDisruptionBudgetControllerRefManager) ReleasePodDisruptionBudget(podDisruptionBudget *policyv1beta1.PodDisruptionBudget) error {
	klog.V(2).InfoS("Patching PodDisruptionBudget to remove its controllerRef",
		"PodDisruptionBudget",
		klog.KObj(podDisruptionBudget),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(podDisruptionBudget.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.PodDisruptionBudgetControl.PatchPodDisruptionBudget(m.ctx, podDisruptionBudget.Namespace, podDisruptionBudget.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the PodDisruptionBudget no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch PodDisruptionBudget as it was missing", klog.KObj(podDisruptionBudget))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the PodDisruptionBudget has no owner reference
			// 2. the UID of the PodDisruptionBudget doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch PodDisruptionBudget as it was invalid", klog.KObj(podDisruptionBudget))
			return nil
		}

		return err
	}

	return nil
}
