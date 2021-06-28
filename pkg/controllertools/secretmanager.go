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

type SecretControlInterface interface {
	PatchSecret(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error
}

type RealSecretControl struct {
	KubeClient kubernetes.Interface
	Recorder   record.EventRecorder
}

var _ SecretControlInterface = &RealSecretControl{}

func (r RealSecretControl) PatchSecret(ctx context.Context, namespace, name string, pt types.PatchType, data []byte, options metav1.PatchOptions) error {
	_, err := r.KubeClient.CoreV1().Secrets(namespace).Patch(ctx, name, pt, data, options)
	return err
}

type SecretControllerRefManager struct {
	kubecontroller.BaseControllerRefManager
	ctx           context.Context
	controllerGVK schema.GroupVersionKind
	SecretControl SecretControlInterface
}

func NewSecretControllerRefManager(
	ctx context.Context,
	controller metav1.Object,
	controllerGVK schema.GroupVersionKind,
	selector labels.Selector,
	canAdopt func() error,
	SecretControl SecretControlInterface,
) *SecretControllerRefManager {
	return &SecretControllerRefManager{
		BaseControllerRefManager: kubecontroller.BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		ctx:           ctx,
		controllerGVK: controllerGVK,
		SecretControl: SecretControl,
	}
}

func (m *SecretControllerRefManager) ClaimSecrets(secrets []*corev1.Secret) (map[string]*corev1.Secret, error) {
	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptSecret(obj.(*corev1.Secret))
	}
	release := func(obj metav1.Object) error {
		return m.ReleaseSecret(obj.(*corev1.Secret))
	}

	claimedMap := make(map[string]*corev1.Secret, len(secrets))
	var errors []error
	for _, secret := range secrets {
		ok, err := m.ClaimObject(secret, match, adopt, release)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if ok {
			claimedMap[secret.Name] = secret
		}
	}
	return claimedMap, utilerrors.NewAggregate(errors)
}

func (m *SecretControllerRefManager) AdoptSecret(secret *corev1.Secret) error {
	err := m.CanAdopt()
	if err != nil {
		return fmt.Errorf("can't adopt Secret %v/%v (%v): %v", secret.Namespace, secret.Name, secret.UID, err)
	}

	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	patchBytes, err := kubecontroller.OwnerRefControllerPatch(m.Controller, m.controllerGVK, secret.UID)
	if err != nil {
		return err
	}

	return m.SecretControl.PatchSecret(m.ctx, secret.Namespace, secret.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

func (m *SecretControllerRefManager) ReleaseSecret(secret *corev1.Secret) error {
	klog.V(2).InfoS("Patching Secret to remove its controllerRef",
		"Secret",
		klog.KObj(secret),
		"ControllerRef",
		fmt.Sprintf("%s/%s:%s", m.controllerGVK.GroupVersion(), m.controllerGVK.Kind, m.Controller.GetName()),
	)

	patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(secret.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}

	err = m.SecretControl.PatchSecret(m.ctx, secret.Namespace, secret.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// If the Secret no longer exists, ignore it.
			klog.V(4).InfoS("Couldn't patch Secret as it was missing", klog.KObj(secret))
			return nil
		}

		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases:
			// 1. the Secret has no owner reference
			// 2. the UID of the Secret doesn't match because it was recreated
			// In both cases, the error can be ignored.
			klog.V(4).InfoS("Couldn't patch Secret as it was invalid", klog.KObj(secret))
			return nil
		}

		return err
	}

	return nil
}
