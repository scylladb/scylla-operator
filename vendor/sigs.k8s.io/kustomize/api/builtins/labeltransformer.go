// Code generated by pluginator on LabelTransformer; DO NOT EDIT.
// pluginator {unknown  1970-01-01T00:00:00Z  }

package builtins

import (
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/transform"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

// Add the given labels to the given field specifications.
type LabelTransformerPlugin struct {
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	FieldSpecs []types.FieldSpec `json:"fieldSpecs,omitempty" yaml:"fieldSpecs,omitempty"`
}

func (p *LabelTransformerPlugin) Config(
	h *resmap.PluginHelpers, c []byte) (err error) {
	p.Labels = nil
	p.FieldSpecs = nil
	return yaml.Unmarshal(c, p)
}

func (p *LabelTransformerPlugin) Transform(m resmap.ResMap) error {
	t, err := transform.NewMapTransformer(
		p.FieldSpecs,
		p.Labels,
	)
	if err != nil {
		return err
	}
	return t.Transform(m)
}

func NewLabelTransformerPlugin() resmap.TransformerPlugin {
	return &LabelTransformerPlugin{}
}
