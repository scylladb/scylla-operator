package scheme

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	utilruntime.Must(kubernetesscheme.AddToScheme(Scheme))

	utilruntime.Must(scyllav1.Install(Scheme))
	utilruntime.Must(scyllav1alpha1.Install(Scheme))
	utilruntime.Must(cqlclientv1alpha1.Install(Scheme))
}
