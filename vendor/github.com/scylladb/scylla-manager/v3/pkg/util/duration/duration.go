// Copyright (C) 2017 ScyllaDB

package duration

import (
	"time"

	"github.com/gocql/gocql"
)

// Duration adds marshalling to time.Duration.
type Duration time.Duration

// MarshalCQL implements gocql.Marshaler.
func (d Duration) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	return gocql.Marshal(info, int(time.Duration(d).Seconds()))
}

// UnmarshalCQL implements gocql.Unmarshaler.
func (d *Duration) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	var t time.Duration
	if err := gocql.Unmarshal(info, data, &t); err != nil {
		return err
	}
	*d = Duration(t * time.Second)
	return nil
}

// MarshalText implements text.Marshaller.
func (d Duration) MarshalText() ([]byte, error) {
	if d == 0 {
		return []byte{}, nil
	}
	return []byte(d.String()), nil
}

// UnmarshalText implements text.Marshaller.
func (d *Duration) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = 0
		return nil
	}

	t, err := ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = t
	return nil
}

// Duration returns this duration as time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}
