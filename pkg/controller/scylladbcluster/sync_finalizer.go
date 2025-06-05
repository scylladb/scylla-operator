// Copyright (c) 2024 ScyllaDB.

package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (scc *Controller) syncFinalizer(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespaces map[string]*corev1.Namespace) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var err error

	if !scc.hasFinalizer(sc.GetFinalizers()) {
		klog.V(4).InfoS("Object is already finalized", "ScyllaDBCluster", klog.KObj(sc), "UID", sc.UID)
		return progressingConditions, nil
	}

	// Delete remote Namespace for every ScyllaDBCluster's datacenter.
	// As all remotely reconciled objects are namespaced, except Namespace,
	// it's enough to remove the Namespace and rely on remote GC to clear the rest.
	// This may need to be adjusted when the Operator will reconcile other cluster-wide resources.
	klog.V(4).InfoS("Finalizing object", "ScyllaDBCluster", klog.KObj(sc), "UID", sc.UID)

	informerRemoteNamespaces := map[string][]*corev1.Namespace{}
	for _, dc := range sc.Spec.Datacenters {
		remoteNamespace, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if ok {
			informerRemoteNamespaces[dc.RemoteKubernetesClusterName] = append(informerRemoteNamespaces[dc.RemoteKubernetesClusterName], remoteNamespace)
		}
	}

	deletionProgressingCondition, err := scc.deleteRemoteNamespaces(ctx, sc, informerRemoteNamespaces)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't finalize remote Namespaces: %w", err)
	}

	progressingConditions = append(progressingConditions, deletionProgressingCondition...)

	// Wait until all Namespaces from Informers are gone.
	if len(informerRemoteNamespaces) != 0 {
		return progressingConditions, nil
	}

	// Live list remote Namespaces to be 100% sure before we delete. Informer cache might not be updated yet.
	var errs []error
	clientRemoteNamespaces := map[string][]*corev1.Namespace{}

	for _, dc := range sc.Spec.Datacenters {
		remoteClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get remote kube client for %q cluster: %w", dc.RemoteKubernetesClusterName, err))
			continue
		}

		rnss, err := remoteClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(naming.ScyllaDBClusterDatacenterSelectorLabels(sc, &dc)).String(),
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("can't list remote Namespaces via %q cluster client: %w", dc.RemoteKubernetesClusterName, err))
			continue
		}

		if len(rnss.Items) == 0 {
			continue
		}

		clientRemoteNamespaces[dc.RemoteKubernetesClusterName] = oslices.ConvertSlice(rnss.Items, pointer.Ptr[corev1.Namespace])
	}

	deletionProgressingCondition, err = scc.deleteRemoteNamespaces(ctx, sc, clientRemoteNamespaces)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't finalize remote Namespaces: %w", err)
	}

	progressingConditions = append(progressingConditions, deletionProgressingCondition...)

	// Wait until all Namespaces from clients are gone.
	if len(clientRemoteNamespaces) != 0 {
		return progressingConditions, nil
	}

	klog.V(2).InfoS("ScyllaDBCluster no longer has dependant objects, removing finalizer")
	err = scc.removeFinalizer(ctx, sc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't remove finalizer from ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	return progressingConditions, nil
}

func (scc *Controller) deleteRemoteNamespaces(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster, namespacesToDelete map[string][]*corev1.Namespace) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var errs []error

	for remoteCluster, remoteNamespaces := range namespacesToDelete {
		remoteClient, err := scc.kubeRemoteClient.Cluster(remoteCluster)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get remote kube client for %q cluster: %w", remoteCluster, err))
			continue
		}

		for _, remoteNamespace := range remoteNamespaces {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, scyllaDBClusterFinalizerProgressingCondition, remoteNamespace, "delete", sc.Generation)
			err = remoteClient.CoreV1().Namespaces().Delete(ctx, remoteNamespace.Name, metav1.DeleteOptions{
				Preconditions:     metav1.NewUIDPreconditions(string(remoteNamespace.UID)),
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
			})
			if err != nil {
				errs = append(errs, fmt.Errorf("can't delete remote Namespace %q from %q cluster: %w", naming.ObjRef(remoteNamespace), remoteCluster, err))
				continue
			}
		}
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't delete remote namespaces: %w", err)
	}

	return progressingConditions, nil
}

func (scc *Controller) hasFinalizer(finalizers []string) bool {
	return oslices.ContainsItem(finalizers, naming.ScyllaDBClusterFinalizer)
}

func (scc *Controller) addFinalizer(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster) error {
	if scc.hasFinalizer(sc.GetFinalizers()) {
		return nil
	}

	patch, err := controllerhelpers.AddFinalizerPatch(sc, naming.ScyllaDBClusterFinalizer)
	if err != nil {
		return fmt.Errorf("can't create add finalizer patch: %w", err)
	}

	_, err = scc.scyllaClient.ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Patch(ctx, sc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	klog.V(2).InfoS("Added finalizer to ScyllaDBCluster", "ScyllaDBCluster", klog.KObj(sc))
	return nil
}

func (scc *Controller) removeFinalizer(ctx context.Context, sc *scyllav1alpha1.ScyllaDBCluster) error {
	patch, err := controllerhelpers.RemoveFinalizerPatch(sc, naming.ScyllaDBClusterFinalizer)
	if err != nil {
		return fmt.Errorf("can't create remove finalizer patch: %w", err)
	}

	_, err = scc.scyllaClient.ScyllaV1alpha1().ScyllaDBClusters(sc.Namespace).Patch(ctx, sc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
	}

	klog.V(2).InfoS("Removed finalizer from ScyllaDBCluster", "ScyllaDBCluster", klog.KObj(sc))
	return nil
}
