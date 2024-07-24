package systemd

import (
	"fmt"
	"io"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/scylladb/scylla-operator/pkg/fsutils"
)

type Mount struct {
	Description string
	Device      string
	MountPoint  string
	FSType      string
	Options     []string
}

func (m *Mount) resolveMountPoint() (string, error) {
	return fsutils.ResolveSymlinks(m.MountPoint)
}

func (m *Mount) MakeUnit() (*NamedUnit, error) {
	resolvedMontPoint, err := m.resolveMountPoint()
	if err != nil {
		return nil, fmt.Errorf("can't resolve mount point %q: %w", m.MountPoint, err)
	}

	data, err := io.ReadAll(unit.SerializeSections([]*unit.UnitSection{
		{
			Section: "Unit",
			Entries: []*unit.UnitEntry{
				{
					Name:  "Description",
					Value: m.Description,
				},
			},
		},
		{
			Section: "Install",
			Entries: []*unit.UnitEntry{
				{
					Name:  "WantedBy",
					Value: "multi-user.target",
				},
			},
		},
		{
			Section: "Mount",
			Entries: []*unit.UnitEntry{
				{
					Name:  "What",
					Value: m.Device,
				},
				{
					Name:  "Where",
					Value: resolvedMontPoint,
				},
				{
					Name:  "Type",
					Value: m.FSType,
				},
				{
					Name:  "Options",
					Value: strings.Join(m.Options, ","),
				},
			},
		},
	}))
	if err != nil {
		return nil, err
	}

	return &NamedUnit{
		FileName: unit.UnitNamePathEscape(resolvedMontPoint) + ".mount",
		Data:     data,
	}, nil
}
