// Copyright (c) 2024 ScyllaDB.

package remotekubernetescluster

import (
	"context"
	"fmt"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func (rkcc *Controller) syncFinalizer(ctx context.Context, rkc *scyllav1alpha1.RemoteKubernetesCluster) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var err error

	if !slices.ContainsItem(rkc.GetFinalizers(), naming.RemoteKubernetesClusterFinalizer) {
		klog.V(4).InfoS("Object is already finalized", "RemoteKubernetesCluster", klog.KObj(rkc), "UID", rkc.UID)
		return progressingConditions, nil
	}

	klog.V(4).InfoS("Finalizing object", "RemoteKubernetesCluster", klog.KObj(rkc), "UID", rkc.UID)

	isUsed, users, err := rkcc.isBeingUsed(ctx, rkc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't check if RemoteKubernetesCluster %q is being used: %w", naming.ObjRef(rkc), err)
	}

	if isUsed {
		klog.V(2).InfoS("Keeping RemoteKubernetesCluster because it's being used", "RemoteKubernetesCluster", klog.KObj(rkc))

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               remoteKubernetesClusterFinalizerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "IsBeingUsed",
			Message:            fmt.Sprintf("Object is being used by following ScyllaDBCluster(s): %s", strings.Join(users, ",")),
			ObservedGeneration: rkc.Generation,
		})

		return progressingConditions, nil
	}

	err = rkcc.removeFinalizer(ctx, rkc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't remove finalizer from RemoteKubernetesCluster %q: %w", naming.ObjRef(rkc), err)
	}
	return progressingConditions, nil
}

func (rkcc *Controller) addFinalizer(ctx context.Context, rkc *scyllav1alpha1.RemoteKubernetesCluster) error {
	patch, err := controllerhelpers.AddFinalizerPatch(rkc, naming.RemoteKubernetesClusterFinalizer)
	if err != nil {
		return fmt.Errorf("can't create add finalizer patch: %w", err)
	}

	_, err = rkcc.scyllaClient.RemoteKubernetesClusters().Patch(ctx, rkc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch RemoteKubernetesCluster %q: %w", naming.ObjRef(rkc), err)
	}

	klog.V(2).InfoS("Added finalizer to RemoteKubernetesCluster", "RemoteKubernetesCluster", klog.KObj(rkc))
	return nil
}

func (rkcc *Controller) removeFinalizer(ctx context.Context, rkc *scyllav1alpha1.RemoteKubernetesCluster) error {
	patch, err := controllerhelpers.RemoveFinalizerPatch(rkc, naming.RemoteKubernetesClusterFinalizer)
	if err != nil {
		return fmt.Errorf("can't create remove finalizer patch: %w", err)
	}

	_, err = rkcc.scyllaClient.RemoteKubernetesClusters().Patch(ctx, rkc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch RemoteKubernetesCluster %q: %w", naming.ObjRef(rkc), err)
	}

	klog.V(2).InfoS("Removed finalizer from RemoteKubernetesCluster", "RemoteKubernetesCluster", klog.KObj(rkc))
	return nil
}

func (rkcc *Controller) isBeingUsed(ctx context.Context, rkc *scyllav1alpha1.RemoteKubernetesCluster) (bool, []string, error) {
	scs, err := rkcc.scyllaDBClusterLister.List(labels.Everything())
	if err != nil {
		return false, nil, fmt.Errorf("can't list all ScyllaClusters using lister: %w", err)
	}

	var scyllaDBClusterReferents []string
	for _, sc := range scs {
		for _, dc := range sc.Spec.Datacenters {
			if dc.RemoteKubernetesClusterName == rkc.Name {
				scyllaDBClusterReferents = append(scyllaDBClusterReferents, naming.ObjRef(sc))
			}
		}
	}

	if len(scyllaDBClusterReferents) != 0 {
		klog.V(4).InfoS("Listed ScyllaClusters using Informer and found ScyllaDBCluster's referencing it", "RemoteKubernetesCluster", klog.KObj(rkc), "ScyllaDBClusters", scyllaDBClusterReferents)
		return true, scyllaDBClusterReferents, nil
	}

	klog.V(4).InfoS("No ScyllaClusters referencing RemoteKubernetesCluster found in the Informer cache", "RemoteKubernetesCluster", klog.KObj(rkc))

	// Live list ScyllaClusters to be 100% sure before we delete. Informer cache might not be updated yet.
	scList, err := rkcc.scyllaClient.ScyllaDBClusters(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})
	if err != nil {
		return false, nil, fmt.Errorf("list all ScyllaClusters using lister: %w", err)
	}

	scyllaDBClusterReferents = scyllaDBClusterReferents[:0]
	for _, sc := range scList.Items {
		for _, dc := range sc.Spec.Datacenters {
			if dc.RemoteKubernetesClusterName == rkc.Name {
				scyllaDBClusterReferents = append(scyllaDBClusterReferents, naming.ObjRef(&sc))
			}
		}
	}

	if len(scyllaDBClusterReferents) != 0 {
		klog.V(4).InfoS("Listed ScyllaClusters using live call and found ScyllaCluster's referencing it", "RemoteKubernetesCluster", klog.KObj(rkc), "ScyllaClusters", scyllaDBClusterReferents)
		return true, scyllaDBClusterReferents, nil
	}

	klog.V(2).InfoS("RemoteKubernetesCluster doesn't have any referents", "RemoteKubernetesCluster", klog.KObj(rkc))

	return false, nil, nil
}
