// Copyright (c) 2026 ScyllaDB.

package controllermanager

import (
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// errorCollector aggregates the errors of many calls, so that a caller can make every call up front and check once.
type errorCollector struct {
	errs []error
}

// Err returns the collected errors as one, or nil when there are none.
func (c *errorCollector) Err() error {
	return apimachineryutilerrors.NewAggregate(c.errs)
}

// collectError returns the value of f and adds its error, if any, to c.
func collectError[T any](c *errorCollector, f func() (T, error)) T {
	v, err := f()
	if err != nil {
		c.errs = append(c.errs, err)
	}

	return v
}
