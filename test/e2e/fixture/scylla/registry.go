package scylla

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) *assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "scyllacluster.yaml.tmpl"
	ScyllaClusterTemplateString string
	ScyllaClusterTemplate       = ParseObjectTemplateOrDie[*scyllav1.ScyllaCluster]("scyllacluster", ScyllaClusterTemplateString)

	//go:embed "zonal.scyllacluster.yaml.tmpl"
	ZonalScyllaClusterTemplateString string
	ZonalScyllaClusterTemplate       = ParseObjectTemplateOrDie[*scyllav1.ScyllaCluster]("zonal-scyllacluster", ZonalScyllaClusterTemplateString)

	//go:embed "scylladb-config.yaml.tmpl"
	ScyllaDBConfigTemplateString string
	ScyllaDBConfigTemplate       = ParseObjectTemplateOrDie[*corev1.ConfigMap]("scylladb-config", ScyllaDBConfigTemplateString)

	//go:embed "nodeconfig.yaml"
	NodeConfig NodeConfigBytes

	//go:embed "default.scyllaoperatorconfig.yaml"
	DefaultScyllaOperatorConfig ScyllaOperatorConfigBytes

	//go:embed "scylladbmonitoring.yaml.tmpl"
	scyllaDBMonitoringTemplateString string
	ScyllaDBMonitoringTemplate       = ParseObjectTemplateOrDie[*scyllav1alpha1.ScyllaDBMonitoring]("scylladbmonitoring", scyllaDBMonitoringTemplateString)

	//go:embed "scylladbdatacenter.yaml.tmpl"
	ScyllaDBDatacenterTemplateString string
	ScyllaDBDatacenterTemplate       = ParseObjectTemplateOrDie[*scyllav1alpha1.ScyllaDBDatacenter]("scylladbdatacenter", ScyllaDBDatacenterTemplateString)

	//go:embed "scylladbcluster.yaml.tmpl"
	ScyllaDBClusterTemplateString string
	ScyllaDBClusterTemplate       = ParseObjectTemplateOrDie[*scyllav1alpha1.ScyllaDBCluster]("scylladbcluster", ScyllaDBClusterTemplateString)

	//go:embed "unauthorized.kubeconfig.yaml"
	UnauthorizedKubeconfigBytes []byte
)

type NodeConfigBytes []byte

func (sc NodeConfigBytes) ReadOrFail() *scyllav1alpha1.NodeConfig {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.NodeConfig)
}

type ScyllaOperatorConfigBytes []byte

func (soc ScyllaOperatorConfigBytes) ReadOrFail() *scyllav1alpha1.ScyllaOperatorConfig {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(soc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.ScyllaOperatorConfig)
}
