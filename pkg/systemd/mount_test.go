package systemd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestMount_MakeUnit(t *testing.T) {
	tt := []struct {
		name         string
		mount        *Mount
		expectedUnit *NamedUnit
		expectedErr  error
	}{
		{
			name: "simple path succeeds",
			mount: &Mount{
				Description: "desc",
				Device:      "/dev/nvme0n1",
				MountPoint:  "/mnt/pvs",
				Options:     []string{"prjquota"},
				FSType:      "xfs",
			},
			expectedErr: nil,
			expectedUnit: &NamedUnit{
				FileName: "mnt-pvs.mount",
				Data: []byte(strings.TrimLeft(`
[Unit]
Description=desc

[Install]
WantedBy=multi-user.target

[Mount]
What=/dev/nvme0n1
Where=/mnt/pvs
Type=xfs
Options=prjquota
`,
					"\n",
				)),
			},
		},
		{
			name: "path with dashes succeeds",
			mount: &Mount{
				Description: "desc",
				Device:      "/dev/nvme0n1",
				MountPoint:  "/mnt/pvs-foo",
				Options:     []string{"prjquota"},
				FSType:      "xfs",
			},
			expectedErr: nil,
			expectedUnit: &NamedUnit{
				FileName: `mnt-pvs\x2dfoo.mount`,
				Data: []byte(strings.TrimLeft(`
[Unit]
Description=desc

[Install]
WantedBy=multi-user.target

[Mount]
What=/dev/nvme0n1
Where=/mnt/pvs-foo
Type=xfs
Options=prjquota
`,
					"\n",
				)),
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
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
