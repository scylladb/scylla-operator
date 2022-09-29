package errors

import (
	"errors"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type aggregate struct {
	errList []error
	sep     string
}

var _ utilerrors.Aggregate = &aggregate{}

func NewAggregate(errList []error, sep string) error {
	var errs []error

	for _, err := range errList {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	if len(sep) == 0 {
		sep = "\n"
	}

	return &aggregate{
		errList: errs,
		sep:     sep,
	}
}

func NewMultilineAggregate(errList []error) error {
	return NewAggregate(errList, "\n")
}

func (agg *aggregate) Error() string {
	msgs := make([]string, 0, len(agg.errList))

	for _, err := range agg.errList {
		msgs = append(msgs, err.Error())
	}

	return strings.Join(msgs, "\n")
}

func (agg *aggregate) Errors() []error {
	return agg.errList
}

func (agg *aggregate) Is(target error) bool {
	return agg.visit(func(err error) bool {
		return errors.Is(err, target)
	})
}

func (agg *aggregate) visit(f func(err error) bool) bool {
	for _, err := range agg.errList {
		switch err := err.(type) {
		case *aggregate:
			match := err.visit(f)
			if match {
				return match
			}

		case utilerrors.Aggregate:
			for _, nestedErr := range err.Errors() {
				match := f(nestedErr)
				if match {
					return match
				}
			}

		default:
			match := f(err)
			if match {
				return match
			}
		}
	}

	return false
}
