package controllertools

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NonRetriable marks err as one that retrying won't fix: controller-runtime logs it and drops the request instead of
// requeuing it.
func NonRetriable(err error) error {
	return reconcile.TerminalError(err)
}

func NewNonRetriable(message string) error {
	return NonRetriable(errors.New(message))
}

func IsNonRetriable(err error) bool {
	return errors.Is(err, reconcile.TerminalError(nil))
}
