package controllertools

import (
	"errors"
)

type NonRetriableError struct {
	error
}

func NonRetriable(err error) error {
	return NonRetriableError{err}
}

func NewNonRetriable(message string) error {
	return NonRetriableError{errors.New(message)}
}

func IsNonRetriable(err error) bool {
	return errors.Is(err, NonRetriableError{})
}
