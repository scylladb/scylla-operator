package scylladb

import (
	_ "embed"
	"text/template"
)

var (
	//go:embed "defaultconfig.yaml"
	defaultConfigTemplateString string
	DefaultConfigTemplate       = template.Must(template.New("scylladb-config").Parse(defaultConfigTemplateString))
)
