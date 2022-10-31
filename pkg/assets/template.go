package assets

import (
	"bytes"
	"fmt"
	"text/template"
)

func RenderTemplate(tmpl *template.Template, data any) ([]byte, error) {
	// We always want correctness. (Accidentally missing a key might have side effects.)
	tmpl.Option("missingkey=error")

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("can't execute template %q: %w", tmpl.Name(), err)
	}

	return buf.Bytes(), nil
}
