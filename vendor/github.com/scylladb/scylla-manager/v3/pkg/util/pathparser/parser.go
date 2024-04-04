// Copyright (C) 2017 ScyllaDB

package pathparser

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/util/uuid"
)

// PathParser can be used to parse parts of string separated by provided sep.
type PathParser struct {
	value string
	sep   string
}

// New returns instance of PathParser.
func New(v, sep string) *PathParser {
	return &PathParser{
		value: v,
		sep:   sep,
	}
}

// Parser describes interface of part parser, user can implement his own parsers.
// PartGetter is used for part retrieval, single parser may consume multiple parts.
type Parser func(string) error

// Parse iterates over provided parsers which consumes string parts.
func (p PathParser) Parse(parsers ...Parser) (int, error) {
	parts := strings.Split(p.value, p.sep)
	for i, p := range parsers {
		if i >= len(parts) {
			return i, nil
		}
		if err := p(parts[i]); err != nil {
			return i, errors.Wrapf(err, "invalid path element at position %d", i)
		}
	}
	return len(parsers), nil
}

// ID parser saves UUID value under given ptr.
func ID(ptr *uuid.UUID) Parser {
	return func(v string) error {
		return ptr.UnmarshalText([]byte(v))
	}
}

// String parser saves string under given ptr.
func String(ptr *string) Parser {
	return func(v string) error {
		*ptr = v
		return nil
	}
}

// Static validates a path part is one of the provided values.
func Static(s ...string) Parser {
	if len(s) == 0 {
		panic("Expected list of values")
	}
	return func(v string) error {
		if len(s) == 1 {
			if v != s[0] {
				return errors.Errorf("expected %s got %s", s[0], v)
			}
		} else {
			var ok bool
			for i := range s {
				if v == s[i] {
					ok = true
					break
				}
			}
			if !ok {
				return errors.Errorf("expected one of %s got %s", s, v)
			}
		}
		return nil
	}
}
