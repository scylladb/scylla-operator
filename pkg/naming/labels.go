package naming

import (
	"fmt"
	"maps"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClusterLabels returns a map of label keys and values
// for the given Cluster.
func ClusterLabels(sdc *scyllav1alpha1.ScyllaDBDatacenter) map[string]string {
	labels := ScyllaLabels()
	labels[ClusterNameLabel] = sdc.Name
	return labels
}

func ClusterLabelsForScyllaCluster(sc *scyllav1.ScyllaCluster) map[string]string {
	labels := ScyllaLabels()
	labels[ClusterNameLabel] = sc.Name
	return labels
}

// ClusterSelector returns a labels selector for the given ScyllaCluster.
func ClusterSelector(sdc *scyllav1alpha1.ScyllaDBDatacenter) labels.Selector {
	return labels.SelectorFromSet(ClusterLabels(sdc))
}

// DatacenterLabels returns a map of label keys and values
// for the given Datacenter.
func DatacenterLabels(sdc *scyllav1alpha1.ScyllaDBDatacenter) map[string]string {
	recLabels := ScyllaLabels()
	dcLabels := ClusterLabels(sdc)
	dcLabels[DatacenterNameLabel] = GetScyllaDBDatacenterGossipDatacenterName(sdc)

	return mergeLabels(dcLabels, recLabels)
}

// RackSelectorLabels returns a map of label keys and values
// for the given Rack.
// Labels set cannot be changed.
func RackSelectorLabels(r scyllav1alpha1.RackSpec, sdc *scyllav1alpha1.ScyllaDBDatacenter) (map[string]string, error) {
	recLabels := ScyllaLabels()
	rackLabels := DatacenterLabels(sdc)
	rackLabels[RackNameLabel] = r.Name

	return mergeLabels(rackLabels, recLabels), nil
}

// StatefulSetPodLabel returns a map of labels to uniquely
// identify a StatefulSet Pod with the given name
func StatefulSetPodLabel(name string) map[string]string {
	return map[string]string{
		appsv1.StatefulSetPodNameLabel: name,
	}
}

// ScyllaDBNodePodLabels returns a map of labels for ScyllaDB node pods.
func ScyllaDBNodePodLabels() map[string]string {
	return map[string]string{
		PodTypeLabel: string(PodTypeScyllaDBNode),
	}
}

func ScyllaLabels() map[string]string {
	return map[string]string{
		"app": AppName,

		"app.kubernetes.io/name":       AppName,
		"app.kubernetes.io/managed-by": OperatorAppName,
	}
}

func ScyllaDBLabelsWithDomain() map[string]string {
	return map[string]string{
		KubernetesNameLabel:      AppNameWithDomain,
		KubernetesManagedByLabel: OperatorAppNameWithDomain,
	}
}

func ManagerSelector() labels.Selector {
	return labels.SelectorFromSet(map[string]string{
		"app.kubernetes.io/name": ManagerAppName,
	})
}

func ScyllaSelector() labels.Selector {
	return labels.SelectorFromSet(ScyllaLabels())
}

func mergeLabels(l1, l2 map[string]string) map[string]string {
	res := make(map[string]string)
	for k, v := range l1 {
		res[k] = v
	}
	for k, v := range l2 {
		res[k] = v
	}
	return res
}

func RemoteManagedResourcesLabels(managingClusterDomain string) map[string]string {
	return map[string]string{
		KubernetesManagedByLabel: RemoteOperatorAppNameWithDomain,
		ManagedByClusterLabel:    managingClusterDomain,
	}
}

func ScyllaDBClusterDatacenterSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter) map[string]string {
	selectorLabels := make(map[string]string)
	maps.Copy(selectorLabels, ScyllaDBClusterRemoteSelectorLabels(sc))
	selectorLabels[ParentClusterDatacenterNameLabel] = dc.Name
	return selectorLabels
}

func ScyllaDBClusterDatacenterLabels(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, managingClusterDomain string) map[string]string {
	dcLabels := make(map[string]string)
	if sc.Spec.Metadata != nil {
		maps.Copy(dcLabels, sc.Spec.Metadata.Labels)
	}

	if dc.Metadata != nil {
		maps.Copy(dcLabels, dc.Metadata.Labels)
	}
	maps.Copy(dcLabels, RemoteManagedResourcesLabels(managingClusterDomain))
	maps.Copy(dcLabels, ScyllaDBClusterRemoteSelectorLabels(sc))
	dcLabels[ParentClusterDatacenterNameLabel] = dc.Name
	return dcLabels
}

func ScyllaDBClusterDatacenterAnnotations(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter) map[string]string {
	dcAnnotations := make(map[string]string)
	if sc.Spec.Metadata != nil {
		maps.Copy(dcAnnotations, sc.Spec.Metadata.Annotations)
	}

	if dc.Metadata != nil {
		maps.Copy(dcAnnotations, dc.Metadata.Annotations)
	}

	return dcAnnotations
}

func ScyllaDBClusterLocalSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster) map[string]string {
	selectorLabels := make(map[string]string)

	maps.Copy(selectorLabels, ScyllaDBLabelsWithDomain())
	selectorLabels[ScyllaDBClusterNameLabel] = sc.Name

	return selectorLabels
}

func ScyllaDBClusterLocalSelector(sc *scyllav1alpha1.ScyllaDBCluster) labels.Selector {
	return labels.SelectorFromSet(ScyllaDBClusterLocalSelectorLabels(sc))
}

func ScyllaDBClusterLocalIdentityServiceSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster) map[string]string {
	selectorLabels := map[string]string{}

	maps.Copy(selectorLabels, ScyllaDBClusterLocalSelectorLabels(sc))
	selectorLabels[ScyllaDBClusterLocalServiceTypeLabel] = string(ScyllaDBClusterLocalServiceTypeIdentity)

	return selectorLabels
}

func ScyllaDBClusterRemoteSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster) map[string]string {
	clusterLabels := make(map[string]string)

	maps.Copy(clusterLabels, map[string]string{
		ParentClusterNamespaceLabel: sc.Namespace,
		ParentClusterNameLabel:      sc.Name,
	})

	return clusterLabels
}

func ScyllaDBClusterRemoteSelector(sc *scyllav1alpha1.ScyllaDBCluster) labels.Selector {
	return labels.SelectorFromSet(ScyllaDBClusterRemoteSelectorLabels(sc))
}

func ScyllaDBClusterDatacenterRemoteEndpointsLabels(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, managingClusterDomain string) map[string]string {
	dcLabels := make(map[string]string)
	if sc.Spec.Metadata != nil {
		maps.Copy(dcLabels, sc.Spec.Metadata.Labels)
	}

	if dc.Metadata != nil {
		maps.Copy(dcLabels, dc.Metadata.Labels)
	}
	maps.Copy(dcLabels, RemoteManagedResourcesLabels(managingClusterDomain))
	maps.Copy(dcLabels, ScyllaDBClusterRemoteEndpointsSelectorLabels(sc))
	dcLabels[ParentClusterDatacenterNameLabel] = dc.Name
	return dcLabels
}

func ScyllaDBClusterRemoteEndpointsSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster) map[string]string {
	scSelectorLabels := ScyllaDBClusterRemoteSelectorLabels(sc)
	scSelectorLabels[RemoteClusterEndpointsLabel] = sc.Name

	return scSelectorLabels
}

func ScyllaDBClusterRemoteEndpointsSelector(sc *scyllav1alpha1.ScyllaDBCluster) labels.Selector {
	return labels.SelectorFromSet(ScyllaDBClusterRemoteEndpointsSelectorLabels(sc))
}

func ScyllaDBClusterEndpointsSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster) map[string]string {
	selectorLabels := ScyllaDBClusterLocalSelectorLabels(sc)
	selectorLabels[ClusterEndpointsLabel] = sc.Name

	return selectorLabels
}

func ScyllaDBClusterEndpointsSelector(sc *scyllav1alpha1.ScyllaDBCluster) labels.Selector {
	return labels.SelectorFromSet(ScyllaDBClusterEndpointsSelectorLabels(sc))
}

func DatacenterPodsSelector(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter) labels.Selector {
	return labels.SelectorFromSet(
		ClusterLabels(
			&scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: ScyllaDBDatacenterName(sc, dc),
				},
			},
		),
	)
}

func GroupVersionResourceToLabelValue(gvr schema.GroupVersionResource) string {
	return fmt.Sprintf("%s-%s-%s", gvr.Group, gvr.Version, gvr.Resource)
}

func RemoteOwnerSelectorLabels(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter) map[string]string {
	return map[string]string{
		RemoteOwnerClusterLabel:   dc.RemoteKubernetesClusterName,
		RemoteOwnerNamespaceLabel: sc.Namespace,
		RemoteOwnerNameLabel:      sc.Name,
		RemoteOwnerGVR:            GroupVersionResourceToLabelValue(scyllav1alpha1.GroupVersion.WithResource("scylladbclusters")),
	}
}

func RemoteOwnerLabels(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, managingClusterDomain string) map[string]string {
	remoteOwnerLabels := make(map[string]string)

	maps.Copy(remoteOwnerLabels, RemoteManagedResourcesLabels(managingClusterDomain))
	maps.Copy(remoteOwnerLabels, RemoteOwnerSelectorLabels(sc, dc))

	return remoteOwnerLabels
}

func GlobalScyllaDBManagerClusterRegistrationSelectorLabels() map[string]string {
	return map[string]string{
		GlobalScyllaDBManagerLabel: LabelValueTrue,
	}
}

func GlobalScyllaDBManagerClusterRegistrationSelector() labels.Selector {
	return labels.SelectorFromSet(GlobalScyllaDBManagerClusterRegistrationSelectorLabels())
}
