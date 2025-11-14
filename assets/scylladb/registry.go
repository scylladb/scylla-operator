package scylladb

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/util/lazy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) *assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "managedconfig.cm.yaml"
	scyllaDBManagedConfigTemplateString string
	ScyllaDBManagedConfigTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ConfigMap] {
		return ParseObjectTemplateOrDie[*corev1.ConfigMap]("scylladb-managed-config", scyllaDBManagedConfigTemplateString)
	})

	//go:embed "snitchconfig.cm.yaml"
	scyllaDBSnitchConfigTemplateString string
	ScyllaDBSnitchConfigTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ConfigMap] {
		return ParseObjectTemplateOrDie[*corev1.ConfigMap]("scylladb-snitch-config", scyllaDBSnitchConfigTemplateString)
	})

	//go:embed "scylla-manager-agent-config.cm.yaml"
	scyllaDBManagerAgentConfigTemplateString string
	ScyllaDBManagerAgentConfigTemplate       = lazy.New(func() *assets.ObjectTemplate[*corev1.ConfigMap] {
		return ParseObjectTemplateOrDie[*corev1.ConfigMap]("scylladb-manager-agent-config", scyllaDBManagerAgentConfigTemplateString)
	})
)
