package naming

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ClusterLabels returns a map of label keys and values
// for the given Cluster.
func ClusterLabels(sd *scyllav1alpha1.ScyllaDatacenter) map[string]string {
	labels := ScyllaLabels()
	labels[ClusterNameLabel] = sd.Name
	return labels
}

// DatacenterLabels returns a map of label keys and values
// for the given Datacenter.
func DatacenterLabels(sd *scyllav1alpha1.ScyllaDatacenter) map[string]string {
	recLabels := ScyllaLabels()
	dcLabels := ClusterLabels(sd)
	dcLabels[DatacenterNameLabel] = sd.Spec.DatacenterName

	return mergeLabels(dcLabels, recLabels)
}

// RackLabels returns a map of label keys and values
// for the given Rack.
func RackLabels(r scyllav1alpha1.RackSpec, sd *scyllav1alpha1.ScyllaDatacenter) map[string]string {
	recLabels := ScyllaLabels()
	rackLabels := DatacenterLabels(sd)
	rackLabels[RackNameLabel] = r.Name

	return mergeLabels(rackLabels, recLabels)
}

// StatefulSetPodLabel returns a map of labels to uniquely
// identify a StatefulSet Pod with the given name
func StatefulSetPodLabel(name string) map[string]string {
	return map[string]string{
		appsv1.StatefulSetPodNameLabel: name,
	}
}

// RackSelector returns a LabelSelector for the given rack.
func RackSelector(r scyllav1alpha1.RackSpec, sd *scyllav1alpha1.ScyllaDatacenter) labels.Selector {

	rackLabelsSet := labels.Set(RackLabels(r, sd))
	sel := labels.SelectorFromSet(rackLabelsSet)

	return sel
}

func ScyllaLabels() map[string]string {

	return map[string]string{
		"app": AppName,

		"app.kubernetes.io/name":       AppName,
		"app.kubernetes.io/managed-by": OperatorAppName,
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

func ParentClusterSelector(sc *scyllav2alpha1.ScyllaCluster) labels.Selector {
	return labels.SelectorFromSet(map[string]string{
		ParentClusterNamespaceLabel: sc.Namespace,
		ParentClusterNameLabel:      sc.Name,
	})
}
