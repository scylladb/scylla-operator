// Copyright (C) 2017 ScyllaDB

package uuid

import (
	"encoding/binary"
	"testing"

	"github.com/gocql/gocql"
)

func TestUUIDFromUint64(t *testing.T) {
	t.Parallel()

	l := uint64(11400714785074694791)&(uint64(0x0F)<<48) | (uint64(0x40) << 48)
	h := uint64(14029467366897019727)&uint64(0x3F) | uint64(0x80)
	u := NewFromUint64(l, h)

	if l != binary.LittleEndian.Uint64(u.Bytes()[0:8]) {
		t.Fatal("wrong lower bits")
	}
	if h != binary.LittleEndian.Uint64(u.Bytes()[8:16]) {
		t.Fatal("wrong higher bits")
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		if id, err := Parse(""); id != Nil || err == nil {
			t.Fatal(id, err)
		}
	})

	t.Run("random", func(t *testing.T) {
		id0 := MustRandom()
		if id1, err := Parse(id0.String()); id0 != id1 {
			t.Fatal(err)
		}
	})
}

func TestUUIDMarshalUnmarshalCQL(t *testing.T) {
	t.Parallel()

	id0 := MustRandom()
	g := gocql.NewNativeType(1, gocql.TypeUUID, "")

	b, err := id0.MarshalCQL(g)
	if err != nil {
		t.Fatal(err)
	}

	var id1 UUID
	if err := id1.UnmarshalCQL(g, b); err != nil {
		t.Fatal(err)
	}

	if id0 != id1 {
		t.Fatal("id mismatch")
	}
}

func TestTimeUUIDMarshal(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		b, err := Nil.MarshalCQL(gocql.NewNativeType(1, gocql.TypeUUID, ""))
		if err != nil {
			t.Fatal(err)
		}
		if b != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("random as time", func(t *testing.T) {
		t.Parallel()

		id := MustRandom()
		_, err := id.MarshalCQL(gocql.NewNativeType(1, gocql.TypeTimeUUID, ""))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("time as random", func(t *testing.T) {
		t.Parallel()

		id := NewTime()
		_, err := id.MarshalCQL(gocql.NewNativeType(1, gocql.TypeUUID, ""))
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestUUID_MarshalUnmarshalJSON(t *testing.T) {
	t.Parallel()

	id0, err := NewRandom()
	if err != nil {
		t.Fatal(err)
	}

	b, err := id0.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var id1 UUID
	if err := id1.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}

	if id0 != id1 {
		t.Fatal("id mismatch")
	}
}

func TestUUID_MarshalUnmarshalText(t *testing.T) {
	t.Parallel()

	id0, err := NewRandom()
	if err != nil {
		t.Fatal(err)
	}

	b, err := id0.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	var id1 UUID
	if err := id1.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}

	if id0 != id1 {
		t.Fatal("id mismatch")
	}
}
