package scheme

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	apimachineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	Scheme                = runtime.NewScheme()
	Codecs                = serializer.NewCodecFactory(Scheme, serializer.EnableStrict)
	DefaultYamlSerializer = json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		Scheme,
		Scheme,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: true,
		},
	)
	localSchemeBuilder = runtime.SchemeBuilder{
		kscheme.AddToScheme,
		scyllav1.Install,
		scyllav1alpha1.Install,
		cqlclientv1alpha1.Install,
		monitoringv1.AddToScheme,
	}

	AddToScheme = localSchemeBuilder.AddToScheme
)

func init() {
	apimachineryutilruntime.Must(AddToScheme(Scheme))
}
