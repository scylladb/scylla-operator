package scheme

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	kubernetesscheme.AddToScheme(Scheme)
	scyllav1.Install(Scheme)
	scyllav1alpha1.Install(Scheme)
}
