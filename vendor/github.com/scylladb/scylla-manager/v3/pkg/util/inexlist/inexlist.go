// Copyright (C) 2017 ScyllaDB

package inexlist

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/service"
)

type sign bool

func (s sign) String() string {
	if s {
		return ""
	}
	return "!"
}

type signedPattern struct {
	Sign    sign
	Pattern string
	g       glob.Glob
}

// InExList is an include/exclude list that can filter string slices based on
// a given set of glob patterns.
type InExList struct {
	patterns []string
	list     []signedPattern
}

func (sp signedPattern) String() string {
	return fmt.Sprintf("%s%s", sp.Sign, sp.Pattern)
}

// ParseInExList takes a list of patterns such as
// [keyspace_1.table_1 !keyspace_1.table_2] and parses it into executable rules.
// If any of the glob pattern is invalid an error is returned.
func ParseInExList(patterns []string) (InExList, error) {
	inExList := InExList{
		patterns: patterns,
	}

	parsed := make([]signedPattern, 0, len(patterns))

	for i, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		s := !strings.HasPrefix(p, "!")
		p = strings.TrimLeft(p, "!")
		g, err := glob.Compile(p)
		if err != nil {
			return InExList{}, service.ErrValidate(errors.Errorf("invalid glob p on position %d: %s", i, p))
		}
		parsed = append(parsed, signedPattern{Sign: sign(s), Pattern: p, g: g})
	}

	inExList.list = parsed

	return inExList, nil
}

// Filter simply applies the rules to the supplied list of strings.
// If an earlier rule excludes a given entry a succeeding rule can later add it back.
func (ie InExList) Filter(in []string) []string {
	if len(ie.patterns) == 0 {
		return in
	}

	var out []string

	for _, v := range in {
		ok := false
		for _, p := range ie.list {
			if p.g.Match(v) {
				ok = bool(p.Sign)
			}
		}
		if ok {
			out = append(out, v)
		}
	}

	return out
}

// FirstMatch takes a string and tries to find a matching rule and
// if it finds one the position of that rule is returned.
// If no matching rule is found then -1 is returned.
func (ie InExList) FirstMatch(s string) int {
	for i, sp := range ie.list {
		if bool(sp.Sign) && sp.g.Match(s) {
			return i
		}
	}

	return -1
}

// Size returns the number of patterns.
func (ie InExList) Size() int {
	return len(ie.list)
}
