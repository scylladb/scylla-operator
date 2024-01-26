// Copyright (C) 2017 ScyllaDB

package service

import (
	"github.com/gocql/gocql"
	"github.com/pkg/errors"
)

// Common errors.
var (
	ErrNotFound = gocql.ErrNotFound
	ErrNilPtr   = errors.New("nil")
)

// errValidate is a validation error caused by inner error.
type errValidate struct { // nolint: errname
	err error
}

// ErrValidate marks error as a validation error, if err is not nil.
func ErrValidate(err error) error {
	if err == nil {
		return nil
	}
	return errValidate{err: err}
}

// IsErrValidate checks if given error is a validation error.
func IsErrValidate(err error) bool {
	_, ok := err.(errValidate) // nolint: errorlint
	return ok
}

// Error implements error.
func (e errValidate) Error() string {
	return e.err.Error()
}
