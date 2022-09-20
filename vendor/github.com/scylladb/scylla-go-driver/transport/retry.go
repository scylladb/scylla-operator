package transport

import (
	"github.com/scylladb/scylla-go-driver/frame"
	. "github.com/scylladb/scylla-go-driver/frame/response"
)

type RetryInfo struct {
	Error       error             // Failed query error.
	Idempotent  bool              // Is set to true only if we are sure that the query is idempotent.
	Consistency frame.Consistency // Failed query consistency.
}

type RetryDecision byte

const (
	RetrySameNode RetryDecision = iota
	RetryNextNode
	DontRetry
)

type RetryPolicy interface {
	NewRetryDecider() RetryDecider
}

// RetryDecider should be used for just one query that we want to retry.
// After that it should be discarded or reset.
type RetryDecider interface {
	Decide(RetryInfo) RetryDecision
	Reset()
}

type FallthroughRetryPolicy struct{}

func (*FallthroughRetryPolicy) NewRetryDecider() RetryDecider {
	return FallthroughRetryDecider{}
}

func NewFallthroughRetryPolicy() RetryPolicy {
	return &FallthroughRetryPolicy{}
}

type FallthroughRetryDecider struct{}

func (FallthroughRetryDecider) Decide(_ RetryInfo) RetryDecision {
	return DontRetry
}

func (FallthroughRetryDecider) Reset() {}

type DefaultRetryPolicy struct{}

func (*DefaultRetryPolicy) NewRetryDecider() RetryDecider {
	return &DefaultRetryDecider{
		wasUnavailable:  false,
		wasReadTimeout:  false,
		wasWriteTimeout: false,
	}
}

func NewDefaultRetryPolicy() RetryPolicy {
	return &DefaultRetryPolicy{}
}

type DefaultRetryDecider struct {
	// Those fields are set to true if specified error occurred during retry process.
	wasUnavailable  bool
	wasReadTimeout  bool
	wasWriteTimeout bool
}

func (d *DefaultRetryDecider) Decide(ri RetryInfo) RetryDecision {
	v, ok := ri.Error.(CodedError)
	if !ok {
		if ri.Idempotent {
			return RetryNextNode
		} else {
			return DontRetry
		}
	}
	switch v.ErrorCode() {
	// Basic errors - there are some problems on this node.
	// Retry on a different one if possible.
	case frame.ErrCodeOverloaded, frame.ErrCodeServer, frame.ErrCodeTruncate:
		if ri.Idempotent {
			return RetryNextNode
		} else {
			return DontRetry
		}
	// Unavailable - the current node believes that not enough nodes
	// are alive to satisfy specified consistency requirements.
	// Maybe this node has network problems - try a different one.
	// Perform at most one retry - it's unlikely that two nodes
	// have network problems at the same time.
	case frame.ErrCodeUnavailable:
		if !d.wasUnavailable {
			d.wasUnavailable = true
			return RetryNextNode
		} else {
			return DontRetry
		}
	// Is Bootstrapping - node can't execute the query, we should try another one.
	case frame.ErrCodeBootstrapping:
		return RetryNextNode
	// Read Timeout - coordinator didn't receive enough replies in time.
	// Retry at most once and only if there were actually enough replies
	// to satisfy consistency, but they were all just checksums (DataPresent == true).
	// This happens when the coordinator picked replicas that were overloaded/dying.
	// Retried request should have some useful response because the node will detect
	// that these replicas are dead.
	case frame.ErrCodeReadTimeout:
		err := v.(ReadTimeoutError)
		if !d.wasReadTimeout && err.Received >= err.BlockFor && err.DataPresent {
			d.wasReadTimeout = true
			return RetrySameNode
		} else {
			return DontRetry
		}
	// Write timeout - coordinator didn't receive enough replies in time.
	// Retry at most once and only for BatchLog write.
	// Coordinator probably didn't detect the nodes as dead.
	// By the time we retry they should be detected as dead.
	case frame.ErrCodeWriteTimeout:
		err := v.(WriteTimeoutError)
		if !d.wasWriteTimeout && ri.Idempotent && err.WriteType == frame.BatchLog {
			d.wasWriteTimeout = true
			return RetrySameNode
		} else {
			return DontRetry
		}
	default:
		return DontRetry
	}
}

func (d *DefaultRetryDecider) Reset() {
	d.wasUnavailable = false
	d.wasReadTimeout = false
	d.wasWriteTimeout = false
}
