// Copyright (C) 2017 ScyllaDB

package duration

import (
	"testing"
	"time"

	"github.com/gocql/gocql"
)

func TestDurationMarshalUnmarshalCQL(t *testing.T) {
	t.Parallel()

	g := gocql.NewNativeType(1, gocql.TypeInt, "")

	d0 := Duration(time.Hour)

	b, err := d0.MarshalCQL(g)
	if err != nil {
		t.Fatal(err)
	}

	var d1 Duration
	if err := d1.UnmarshalCQL(g, b); err != nil {
		t.Fatal(err)
	}

	if d0 != d1 {
		t.Fatal("mismatch", d0, d1)
	}
}

func TestDurationMarshalUnmarshalText(t *testing.T) {
	t.Parallel()

	d0 := Duration(time.Hour)

	b, err := d0.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	var d1 Duration
	if err := d1.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}

	if d0 != d1 {
		t.Fatal("mismatch", d0, d1)
	}
}

func TestDurationMarshalTextZero(t *testing.T) {
	t.Parallel()

	d0 := Duration(0)

	b, err := d0.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	if len(b) != 0 {
		t.Fatal("mismatch", string(b))
	}
}

func TestDurationUnmarshalTextZero(t *testing.T) {
	t.Parallel()

	var d Duration
	if err := d.UnmarshalText(nil); err != nil {
		t.Fatal(err)
	}

	if d != 0 {
		t.Fatal("mismatch", d)
	}
}

func TestDurationUnmarshalTextBelowSeconds(t *testing.T) {
	t.Parallel()

	var d Duration
	if err := d.UnmarshalText([]byte("150ms")); err == nil {
		t.Fatal("expected error")
	}
}

func TestDurationUnmarshalTextDays(t *testing.T) {
	t.Parallel()

	var d Duration
	if err := d.UnmarshalText([]byte("7d")); err != nil {
		t.Fatal(err)
	}
	if d.Duration() != 7*24*time.Hour {
		t.Fatal("expected 7 days, got", d)
	}
}
