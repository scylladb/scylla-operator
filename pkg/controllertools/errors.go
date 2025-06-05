package controllertools

import (
	"errors"
)

type NonRetriableError struct {
	error
}

func (e NonRetriableError) Unwrap() error {
	return e.error
}

func NonRetriable(err error) error {
	return NonRetriableError{err}
}

func NewNonRetriable(message string) error {
	return NonRetriableError{errors.New(message)}
}

func IsNonRetriable(err error) bool {
	return errors.As(err, &NonRetriableError{})
}
