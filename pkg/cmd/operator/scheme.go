package operator

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = scyllav1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}
