package util

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

// NewControllerRef returns an OwnerReference to
// the provided Cluster Object
func NewControllerRef(c *scyllav1.ScyllaCluster) metav1.OwnerReference {
	return *metav1.NewControllerRef(c, schema.GroupVersionKind{
		Group:   "scylla.scylladb.com",
		Version: "v1",
		Kind:    "ScyllaCluster",
	})
}

func NewNodeConfigControllerRef(snt *scyllav1alpha1.ScyllaNodeConfig) metav1.OwnerReference {
	return *metav1.NewControllerRef(snt, schema.GroupVersionKind{
		Group:   "scylla.scylladb.com",
		Version: "v1alpha1",
		Kind:    "ScyllaNodeConfig",
	})
}

func NewPodOwnerReference(pod *corev1.Pod) metav1.OwnerReference {
	gvk := corev1.SchemeGroupVersion.WithKind("Pod")
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               pod.GetName(),
		UID:                pod.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}
}
