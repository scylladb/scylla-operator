// Copyright (C) 2017 ScyllaDB

package uuid

import (
	"encoding/binary"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
)

// UUID reference:
// https://tools.ietf.org/html/rfc4122

// Nil UUID is special form of UUID that is specified to have all 128 bits set
// to zero (see https://tools.ietf.org/html/rfc4122#section-4.1.7).
var Nil UUID

// UUID is a wrapper for a UUID type.
type UUID struct {
	uuid gocql.UUID
}

// NewRandom returns a random (Version 4) UUID and error if it fails to read
// from it's random source.
func NewRandom() (UUID, error) {
	u, err := gocql.RandomUUID()
	if err != nil {
		return Nil, err
	}
	return UUID{u}, nil
}

// MustRandom works like NewRandom but will panic on error.
func MustRandom() UUID {
	u, err := gocql.RandomUUID()
	if err != nil {
		panic(err)
	}
	return UUID{u}
}

// NewTime generates a new time based UUID (version 1) using the current
// time as the timestamp.
func NewTime() UUID {
	return UUID{gocql.TimeUUID()}
}

// NewFromUint64 creates a UUID from a uint64 pair.
func NewFromUint64(l, h uint64) UUID {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[:8], l)
	binary.LittleEndian.PutUint64(b[8:], h)

	b[6] &= 0x0F // clear version
	b[6] |= 0x40 // set version to 4 (random uuid)
	b[8] &= 0x3F // clear variant
	b[8] |= 0x80 // set to IETF variant

	return UUID{b}
}

// Parse creates a new UUID from string.
func Parse(s string) (UUID, error) {
	var u UUID
	err := u.UnmarshalText([]byte(s))
	return u, err
}

// MustParse creates a new UUID from string and panics if s is not an UUID.
func MustParse(s string) UUID {
	u, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// Bytes returns the raw byte slice for this UUID. A UUID is always 128 bits
// (16 bytes) long.
func (u UUID) Bytes() []byte {
	b := make([]byte, 16)
	copy(b, u.uuid[:])
	return b
}

// Bytes16 returns the raw byte array for this UUID.
func (u UUID) Bytes16() [16]byte {
	return u.uuid
}

// MarshalCQL implements gocql.Marshaler.
func (u UUID) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	if u == Nil {
		return nil, nil
	}

	switch info.Type() {
	case gocql.TypeUUID:
		return u.uuid[:], nil
	case gocql.TypeTimeUUID:
		if u.uuid[6]&0x10 != 0x10 {
			return nil, errors.New("not a timeuuid")
		}
		return u.uuid[:], nil
	default:
		return nil, errors.Errorf("unsupported type %q", info.Type())
	}
}

// UnmarshalCQL implements gocql.Unmarshaler.
func (u *UUID) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	if info.Type() != gocql.TypeUUID && info.Type() != gocql.TypeTimeUUID {
		return errors.Errorf("unsupported type %q", info.Type())
	}

	if len(data) == 0 {
		*u = Nil
		return nil
	}

	if len(data) != 16 {
		return errors.New("UUIDs must be exactly 16 bytes long")
	}

	copy(u.uuid[:], data)
	return nil
}

// MarshalJSON implements json.Marshaller.
func (u UUID) MarshalJSON() ([]byte, error) {
	return u.uuid.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (u *UUID) UnmarshalJSON(data []byte) error {
	return u.uuid.UnmarshalJSON(data)
}

// MarshalText implements text.Marshaller.
func (u UUID) MarshalText() ([]byte, error) {
	return u.uuid.MarshalText()
}

// UnmarshalText implements text.Marshaller.
func (u *UUID) UnmarshalText(text []byte) error {
	return u.uuid.UnmarshalText(text)
}

// String returns the UUID in it's canonical form, a 32 digit hexadecimal
// number in the form of xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func (u UUID) String() string {
	return u.uuid.String()
}
