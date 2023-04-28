package systemd

import (
	"io"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
)

type Mount struct {
	Description string
	Device      string
	MountPoint  string
	FSType      string
	Options     []string
}

func (m *Mount) MakeUnit() (*NamedUnit, error) {
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
					Value: m.MountPoint,
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
		FileName: unit.UnitNamePathEscape(m.MountPoint) + ".mount",
		Data:     data,
	}, nil
}
