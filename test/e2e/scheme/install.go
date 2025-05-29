package scheme

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
)

func init() {
	apimachineryutilruntime.Must(kubernetesscheme.AddToScheme(Scheme))

	apimachineryutilruntime.Must(scyllav1.Install(Scheme))
	apimachineryutilruntime.Must(scyllav1alpha1.Install(Scheme))
	apimachineryutilruntime.Must(cqlclientv1alpha1.Install(Scheme))
}
