package systemd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-operator/pkg/assets"
)

func TestMount_MakeUnit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDirEscaped := unit.UnitNamePathEscape(tmpDir)

	evalTemplate := func(s string) []byte {
		s = strings.TrimLeft(s, "\n")

		tmpl, evalErr := template.New("").Funcs(nil).Parse(s)
		if evalErr != nil {
			t.Error(evalErr)
		}

		data, evalErr := assets.RenderTemplate(tmpl, map[string]interface{}{
			"tmpDir": tmpDir,
		})

		return data
	}

	nonexistingDir := filepath.Join(tmpDir, "nonexisting")

	realDir := filepath.Join(tmpDir, "real")
	err := os.Mkdir(realDir, 0755)
	if err != nil {
		t.Error(err)
	}

	symlinkedDir := filepath.Join(tmpDir, "symlink")
	err = os.Symlink(realDir, symlinkedDir)
	if err != nil {
		t.Error(err)
	}

	tt := []struct {
		name         string
		mount        *Mount
		expectedUnit *NamedUnit
		expectedErr  error
	}{
		{
			name: "preexisting mount path succeeds",
			mount: &Mount{
				Description: "desc",
				Device:      "/dev/nvme0n1",
				MountPoint:  realDir,
				Options:     []string{"prjquota"},
				FSType:      "xfs",
			},
			expectedErr: nil,
			expectedUnit: &NamedUnit{
				FileName: tmpDirEscaped + "-real.mount",
				Data: evalTemplate(`
[Unit]
Description=desc

[Install]
WantedBy=multi-user.target

[Mount]
What=/dev/nvme0n1
Where={{ .tmpDir }}/real
Type=xfs
Options=prjquota
`,
				),
			},
		},
		{
			name: "preexisting symlinked mount path succeeds",
			mount: &Mount{
				Description: "desc",
				Device:      "/dev/nvme0n1",
				MountPoint:  symlinkedDir,
				Options:     []string{"prjquota"},
				FSType:      "xfs",
			},
			expectedErr: nil,
			expectedUnit: &NamedUnit{
				FileName: tmpDirEscaped + "-real.mount",
				Data: evalTemplate(`
[Unit]
Description=desc

[Install]
WantedBy=multi-user.target

[Mount]
What=/dev/nvme0n1
Where={{ .tmpDir }}/real
Type=xfs
Options=prjquota
`,
				),
			},
		},
		{
			name: "nonexisting path with dashes succeeds",
			mount: &Mount{
				Description: "desc",
				Device:      "/dev/nvme0n1",
				MountPoint:  nonexistingDir + "-foo",
				Options:     []string{"prjquota"},
				FSType:      "xfs",
			},
			expectedErr: nil,
			expectedUnit: &NamedUnit{
				FileName: tmpDirEscaped + `-nonexisting\x2dfoo.mount`,
				Data: evalTemplate(`
[Unit]
Description=desc

[Install]
WantedBy=multi-user.target

[Mount]
What=/dev/nvme0n1
Where={{ .tmpDir }}/nonexisting-foo
Type=xfs
Options=prjquota
`,
				),
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.mount.MakeUnit()
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}
			if !reflect.DeepEqual(got, tc.expectedUnit) {
				t.Errorf("expected and got units differ:\n%s", cmp.Diff(tc.expectedUnit, got))
			}
		})
	}
}
