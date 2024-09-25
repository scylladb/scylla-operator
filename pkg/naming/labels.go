package naming

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
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
