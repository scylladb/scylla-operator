package assets

import (
	"errors"
	"testing"
	"text/template"
)

func TestRenderTemplate(t *testing.T) {
	t.Parallel()

	tpl, err := template.New("test").Parse("Hello, {{.Name}}!")
	if err != nil {
		t.Fatalf("can't parse template: %v", err)
	}

	tests := []struct {
		name    string
		data    map[string]string
		want    string
		wantErr error
	}{
		{
			name: "simple",
			data: map[string]string{"Name": "World"},
			want: "Hello, World!",
		},
		{
			name:    "missing key",
			data:    map[string]string{},
			wantErr: errors.New(`can't execute template "test": template: test:1:9: executing "test" at <.Name>: map has no entry for key "Name"`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := RenderTemplate(tpl, tc.data)
			if tc.wantErr != nil {
				if err == nil || err.Error() != tc.wantErr.Error() {
					t.Fatalf("expected error %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("can't render template: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, string(out))
			}
		})
	}
}
