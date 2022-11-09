// Copyright (c) 2022 ScyllaDB.

package migration

import (
	"context"
	"fmt"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	kubecontroller "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (scmc *Controller) orphanStatefulSets(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})
	lister := func(namespace string, selector labels.Selector) ([]*appsv1.StatefulSet, error) {
		return scmc.statefulSetLister.StatefulSets(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*appsv1.StatefulSet, error) {
		return scmc.kubeClient.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanServices(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*corev1.Service, error) {
		return scmc.serviceLister.Services(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*corev1.Service, error) {
		return scmc.kubeClient.CoreV1().Services(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanSecrets(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
		return scmc.secretLister.Secrets(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*corev1.Secret, error) {
		return scmc.kubeClient.CoreV1().Secrets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanConfigMaps(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*corev1.ConfigMap, error) {
		return scmc.configMapLister.ConfigMaps(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*corev1.ConfigMap, error) {
		return scmc.kubeClient.CoreV1().ConfigMaps(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanServiceAccounts(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*corev1.ServiceAccount, error) {
		return scmc.serviceAccountLister.ServiceAccounts(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*corev1.ServiceAccount, error) {
		return scmc.kubeClient.CoreV1().ServiceAccounts(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanRoleBindings(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
		return scmc.roleBindingLister.RoleBindings(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*rbacv1.RoleBinding, error) {
		return scmc.kubeClient.RbacV1().RoleBindings(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanPodDisruptionBudgets(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*policyv1.PodDisruptionBudget, error) {
		return scmc.pdbLister.PodDisruptionBudgets(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*policyv1.PodDisruptionBudget, error) {
		return scmc.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) orphanIngresses(ctx context.Context, sc *scyllav1.ScyllaCluster) error {
	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sc.Name,
	})

	lister := func(namespace string, selector labels.Selector) ([]*networkingv1.Ingress, error) {
		return scmc.ingressLister.Ingresses(namespace).List(selector)
	}
	patcher := func(ctx context.Context, namespace string, name string, patchType types.PatchType, patchBytes []byte, patchOptions metav1.PatchOptions) (*networkingv1.Ingress, error) {
		return scmc.kubeClient.NetworkingV1().Ingresses(namespace).Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err := orphanObject(ctx, sc, selector, lister, patcher); err != nil {
		return err
	}

	return nil
}

func (scmc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started migrating ScyllaCluster children", "ScyllaCluster", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished migrating ScyllaCluster children", "ScyllaCluster", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sd, err := scmc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaCluster has been deleted", "ScyllaCluster", klog.KObj(sd))
		return nil
	}
	if err != nil {
		return err
	}

	var errs []error

	if err := scmc.orphanStatefulSets(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan statefulsets: %w", err))
	}

	if err := scmc.orphanServices(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan services: %w", err))
	}

	if err := scmc.orphanSecrets(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan secrets: %w", err))
	}

	if err := scmc.orphanConfigMaps(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan configMaps: %w", err))
	}

	if err := scmc.orphanServiceAccounts(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan serviceAccounts: %w", err))
	}

	if err := scmc.orphanRoleBindings(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan roleBindings: %w", err))
	}

	if err := scmc.orphanPodDisruptionBudgets(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan podDisruptionBudgets: %w", err))
	}

	if err := scmc.orphanIngresses(ctx, sd); err != nil {
		errs = append(errs, fmt.Errorf("can't orphan ingresses: %w", err))
	}

	return utilerrors.NewAggregate(errs)
}

func orphanObject[T metav1.Object](
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	selector labels.Selector,
	lister func(namespace string, selector labels.Selector) ([]T, error),
	patcher func(context.Context, string, string, types.PatchType, []byte, metav1.PatchOptions) (T, error),
) error {
	objects, err := lister(sc.Namespace, selector)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		if !shouldOrphanObject(obj, sc) {
			continue
		}

		patchBytes, err := kubecontroller.DeleteOwnerRefStrategicMergePatch(obj.GetUID(), sc.GetUID())
		if err != nil {
			return err
		}
		_, err = patcher(ctx, obj.GetNamespace(), obj.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// If the StatefulSet no longer exists, ignore it.
				klog.V(4).InfoS("Couldn't patch object as it was missing", klog.KObj(obj))
				return nil
			}

			if errors.IsInvalid(err) {
				// Invalid error will be returned in two cases:
				// 1. the StatefulSet has no owner reference
				// 2. the UID of the StatefulSet doesn't match because it was recreated
				// In both cases, the error can be ignored.
				klog.V(4).InfoS("Couldn't patch object as it was invalid", klog.KObj(obj))
				return nil
			}

			return err
		}
	}

	return nil
}

func shouldOrphanObject(obj metav1.Object, owner metav1.Object) bool {
	controllerRef := metav1.GetControllerOfNoCopy(obj)
	if controllerRef == nil {
		return false
	}

	if controllerRef.UID != owner.GetUID() {
		return false
	}

	if controllerRef.Kind != scyllaClusterControllerGVK.Kind {
		return false
	}

	if controllerRef.APIVersion != scyllaClusterControllerGVK.GroupVersion().String() {
		return false
	}

	return true
}
