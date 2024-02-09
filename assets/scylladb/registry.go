package scylladb

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "managedconfig.cm.yaml"
	scyllaDBManagedConfigTemplateString string
	ScyllaDBManagedConfigTemplate       = ParseObjectTemplateOrDie[*corev1.ConfigMap]("scylladb-managed-config", scyllaDBManagedConfigTemplateString)
)
