package controllertools

import (
	"context"
	"fmt"

	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	appsv1 "k8s.io/api/apps/v1"
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

type DeploymentControlInterface interface {
	PatchDeployment(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealDeploymentControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ DeploymentControlInterface = &RealDeploymentControl{}

func (r RealDeploymentControl) PatchDeployment(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.AppsV1().Deployments(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type DeploymentControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx               context.Context
	controllerGVK     schema.GroupVersionKind
	DeploymentControl DeploymentControlInterface
}

func NewDeploymentControllerRefManager(ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	DeploymentControl DeploymentControlInterface,
) *DeploymentControllerRefManager {
	return &DeploymentControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:               ctx,
		controllerGVK:     controllerGVK,
		DeploymentControl: DeploymentControl,
	}
}

func (m *DeploymentControllerRefManager) ClaimDeployments(deployments []*appsv1.Deployment) (map[string]*appsv1.Deployment, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptDeployment(obj.(*appsv1.Deployment))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseDeployment(obj.(*appsv1.Deployment))
	}

	claimedMap := make(map[string]*appsv1.Deployment, len(deployments))
	var errors []error
	for _, deployment := range deployments {
		ok, err := m.ClaimObject(deployment, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[deployment.Name] = deployment
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *DeploymentControllerRefManager) AdoptDeployment(deployment *appsv1.Deployment) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt Deployment %v/%v (%v): %v", deployment.Namespace, deployment.Name, deployment.UID, err)
	}

	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, deployment.UID)
	if err != nil {
		return fmt.Errorf("can't get patchBytes: %v", err)
	}

	return m.DeploymentControl.PatchDeployment(m.ctx, deployment.Namespace, deployment.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *DeploymentControllerRefManager) ReleaseDeployment(deployment *appsv1.Deployment) error {
	klog.V(2).InfoS("Patching Deployment to remove its controllerRef",
		"Deployment",
		klog.KObj(deployment),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(deployment.UID, m.Controller.GetUID())
	if err != nil {
		return fmt.Errorf("can't get patchBytes: %v", err)
	}

	err = m.DeploymentControl.PatchDeployment(m.ctx, deployment.Namespace, deployment.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Deployment no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch Deployment as it was missing", klog.KObj(deployment))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the Deployment has no owner reference
			// 2. the UID of the Deployment doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch Deployment as it was invalid", klog.KObj(deployment))
			return nil
		}
	}

	return err
}
