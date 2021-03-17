package naming

import (
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ClusterLabels returns a map of label keys and values
// for the given Cluster.
func ClusterLabels(c *scyllav1.ScyllaCluster) map[string]string {
	labels := recommendedLabels()
	labels[ClusterNameLabel] = c.Name
	return labels
}

// DatacenterLabels returns a map of label keys and values
// for the given Datacenter.
func DatacenterLabels(c *scyllav1.ScyllaCluster) map[string]string {
	recLabels := recommendedLabels()
	dcLabels := ClusterLabels(c)
	dcLabels[DatacenterNameLabel] = c.Spec.Datacenter.Name

	return mergeLabels(dcLabels, recLabels)
}

// RackLabels returns a map of label keys and values
// for the given Rack.
func RackLabels(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster) map[string]string {
	recLabels := recommendedLabels()
	rackLabels := DatacenterLabels(c)
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
func RackSelector(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster) labels.Selector {

	rackLabelsSet := labels.Set(RackLabels(r, c))
	sel := labels.SelectorFromSet(rackLabelsSet)

	return sel
}

func SelectorForSeeds(clusterName string) string {
	return fmt.Sprintf("%s,%s=%s", SeedLabel, ClusterNameLabel, clusterName)
}

func recommendedLabels() map[string]string {

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
