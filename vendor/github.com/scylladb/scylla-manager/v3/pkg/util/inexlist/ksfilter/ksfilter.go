// Copyright (C) 2017 ScyllaDB

package ksfilter

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/service"
	"github.com/scylladb/scylla-manager/v3/pkg/util/inexlist"
	"go.uber.org/multierr"
)

// Unit specifies keyspace and it's tables.
type Unit struct {
	Keyspace  string
	Tables    []string
	AllTables bool
}

// Filter is a builder that let's you filter keyspaces and tables by adding them
// on keyspace by keyspace basis.
type Filter struct {
	filters   []string
	inex      inexlist.InExList
	units     []Unit
	keyspaces []string
}

func NewFilter(filters []string) (*Filter, error) {
	// Validate filters
	var errs error
	for i, f := range filters {
		err := validate(filters[i])
		if err != nil {
			errs = multierr.Append(errs, errors.Wrapf(err, "%q on position %d", f, i))
			continue
		}
	}
	if errs != nil {
		return nil, service.ErrValidate(errors.Wrap(errs, "invalid filters"))
	}

	// Decorate filters and create inexlist
	inex, err := inexlist.ParseInExList(decorate(filters))
	if err != nil {
		return nil, err
	}

	return &Filter{
		filters: filters,
		inex:    inex,
	}, nil
}

func validate(filter string) error {
	if filter == "*" || filter == "!*" {
		return nil
	}
	if strings.HasPrefix(filter, ".") {
		return errors.New("missing keyspace")
	}
	return nil
}

func decorate(filters []string) []string {
	if len(filters) == 0 {
		filters = append(filters, "*.*")
	}

	for i, f := range filters {
		if strings.Contains(f, ".") {
			continue
		}
		if strings.HasSuffix(f, "*") {
			filters[i] = strings.TrimSuffix(f, "*") + "*.*"
		} else {
			filters[i] += ".*"
		}
	}

	return filters
}

// Add filters the keyspace and tables, if they match it adds a new unit to the
// Filter.
func (f *Filter) Add(keyspace string, tables []string) {
	// Add prefix
	prefix := keyspace + "."
	for i := 0; i < len(tables); i++ {
		tables[i] = prefix + tables[i]
	}

	// Filter
	filtered := f.inex.Filter(tables)

	// No data, skip the keyspace
	if len(filtered) == 0 {
		return
	}

	// Remove prefix
	for i := 0; i < len(filtered); i++ {
		filtered[i] = strings.TrimPrefix(filtered[i], prefix)
	}

	// Add unit
	u := Unit{
		Keyspace:  keyspace,
		Tables:    filtered,
		AllTables: len(filtered) == len(tables),
	}
	f.units = append(f.units, u)

	f.keyspaces = append(f.keyspaces, keyspace)
}

// Check returns true iff table matches filter.
func (f *Filter) Check(keyspace, table string) bool {
	key := keyspace + "." + table
	return len(f.inex.Filter([]string{key})) > 0
}

// Apply returns the resulting units. The units are sorted by position of a
// first match in the filters.
// If no units are found or a filter is invalid a validation error is returned.
// The validation error may be disabled by providing the force=true.
func (f *Filter) Apply(force bool) ([]Unit, error) {
	if len(f.units) == 0 && !force {
		return nil, service.ErrValidate(errors.Errorf("no keyspace matched criteria %s - available keyspaces are: %s", f.filters, f.keyspaces))
	}

	// Sort units by the presence
	sortUnits(f.units, f.inex)

	return f.units, nil
}

func sortUnits(units []Unit, inclExcl inexlist.InExList) {
	positions := make(map[string]int)
	for _, u := range units {
		min := inclExcl.Size()
		for _, t := range u.Tables {
			if p := inclExcl.FirstMatch(u.Keyspace + "." + t); p >= 0 && p < min {
				min = p
			}
		}
		positions[u.Keyspace] = min
	}

	sort.Slice(units, func(i, j int) bool {
		// order by position
		switch {
		case positions[units[i].Keyspace] < positions[units[j].Keyspace]:
			return true
		case positions[units[i].Keyspace] > positions[units[j].Keyspace]:
			return false
		default:
			// promote system keyspaces
			l := strings.HasPrefix(units[i].Keyspace, "system")
			r := strings.HasPrefix(units[j].Keyspace, "system")
			switch {
			case l && !r:
				return true
			case !l && r:
				return false
			default:
				// order by name
				return units[i].Keyspace < units[j].Keyspace
			}
		}
	})
}

// Filters returns the original filters used to create the instance.
func (f *Filter) Filters() []string {
	if f == nil {
		return nil
	}
	return f.filters
}
