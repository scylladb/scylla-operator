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

func IsNonRetriable(err error) bool {
	return errors.Is(err, NonRetriableError{})
}
