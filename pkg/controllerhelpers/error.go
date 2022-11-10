package controllerhelpers

import (
	"strings"
)

type RequeueError struct {
	reasons []string
}

var _ error = &RequeueError{}

func NewRequeueError(reasons ...string) *RequeueError {
	return &RequeueError{
		reasons: reasons,
	}
}

func (e *RequeueError) Error() string {
	return strings.Join(e.reasons, ", ")
}

func (e *RequeueError) Is(err error) bool {
	_, ok := err.(*RequeueError)
	return ok
}
