// Copyright (C) 2017 ScyllaDB

package backupspec

type LocationValue Location

var nilLocation = Location{}

func (v *LocationValue) String() string {
	if v.Value() == nilLocation {
		return ""
	}
	return Location(*v).String()
}

func (v *LocationValue) Set(s string) error {
	return (*Location)(v).UnmarshalText([]byte(s))
}

func (v *LocationValue) Type() string {
	return "string"
}

func (v *LocationValue) Value() Location {
	return Location(*v)
}

type SnapshotTagValue string

func (v *SnapshotTagValue) String() string {
	return string(*v)
}

func (v *SnapshotTagValue) Set(s string) error {
	if !IsSnapshotTag(s) {
		return errInvalidSnapshotTag
	}
	*v = SnapshotTagValue(s)
	return nil
}

func (v *SnapshotTagValue) Type() string {
	return "string"
}

func (v *SnapshotTagValue) Value() string {
	return string(*v)
}
