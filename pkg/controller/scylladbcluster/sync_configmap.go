package scylladbcluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncRemoteConfigMaps(
	ctx context.Context,
	sc *scyllav1alpha1.ScyllaDBCluster,
	dc *scyllav1alpha1.ScyllaDBClusterDatacenter,
	remoteNamespace *corev1.Namespace,
	remoteController metav1.Object,
	remoteConfigMaps map[string]*corev1.ConfigMap,
	managingClusterDomain string,
) ([]metav1.Condition, error) {
	progressingConditions, requiredConfigMaps, err := MakeRemoteConfigMaps(sc, dc, remoteNamespace, remoteController, scc.configMapLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make required configmaps: %w", err)
	}

	if len(progressingConditions) > 0 {
		return progressingConditions, nil
	}

	clusterClient, err := scc.kubeRemoteClient.Cluster(dc.RemoteKubernetesClusterName)
	if err != nil {
		return nil, fmt.Errorf("can't get client to %q cluster: %w", dc.RemoteKubernetesClusterName, err)
	}

	// Delete has to be the first action to avoid getting stuck on quota.
	err = controllerhelpers.Prune(ctx,
		requiredConfigMaps,
		remoteConfigMaps,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: clusterClient.CoreV1().ConfigMaps(remoteNamespace.Name).Delete,
		},
		scc.eventRecorder,
	)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't prune configmap(s) in %q Datacenter of %q ScyllaDBCluster: %w", dc.Name, naming.ObjRef(sc), err)
	}

	var errs []error
	for _, cm := range requiredConfigMaps {
		_, changed, err := resourceapply.ApplyConfigMap(ctx, clusterClient.CoreV1(), scc.remoteConfigMapLister.Cluster(dc.RemoteKubernetesClusterName), scc.eventRecorder, cm, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, makeRemoteConfigMapControllerDatacenterProgressingCondition(dc.Name), cm, "apply", sc.Generation)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply configmap: %w", err))
			continue
		}
	}

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply configmap(s): %w", err)
	}

	return progressingConditions, nil
}
