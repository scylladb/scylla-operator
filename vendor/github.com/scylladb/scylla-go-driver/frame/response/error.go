package response

import (
	"fmt"

	"github.com/scylladb/scylla-go-driver/frame"
)

// ScyllaError is embedded in all error frames.
// ScyllaError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1046
type ScyllaError struct {
	Code    frame.ErrorCode
	Message string
}

type CodedError interface {
	error
	ErrorCode() frame.ErrorCode
}

func (e ScyllaError) Error() string {
	return fmt.Sprintf("[Scylla error code=%#x message=%q]", e.Code, e.Message)
}

func (e ScyllaError) String() string {
	return fmt.Sprintf("[Scylla error code=%#x message=%q]", e.Code, e.Message)
}

func (e ScyllaError) ErrorCode() frame.ErrorCode {
	return e.Code
}

func ParseScyllaError(b *frame.Buffer) ScyllaError {
	return ScyllaError{
		Code:    b.ReadErrorCode(),
		Message: b.ReadString(),
	}
}

func ParseError(b *frame.Buffer) CodedError {
	err := ParseScyllaError(b)
	switch err.Code {
	case frame.ErrCodeUnavailable:
		return ParseUnavailableError(b, err)
	case frame.ErrCodeWriteTimeout:
		return ParseWriteTimeoutError(b, err)
	case frame.ErrCodeReadTimeout:
		return ParseReadTimeoutError(b, err)
	case frame.ErrCodeReadFailure:
		return ParseReadFailureError(b, err)
	case frame.ErrCodeFunctionFailure:
		return ParseFuncFailureError(b, err)
	case frame.ErrCodeWriteFailure:
		return ParseWriteFailureError(b, err)
	case frame.ErrCodeAlreadyExists:
		return ParseAlreadyExistsError(b, err)
	case frame.ErrCodeUnprepared:
		return ParseUnpreparedError(b, err)
	default:
		return err
	}
}

// UnavailableError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1060
type UnavailableError struct {
	ScyllaError
	Consistency frame.Consistency
	Required    frame.Int
	Alive       frame.Int
}

func ParseUnavailableError(b *frame.Buffer, err ScyllaError) UnavailableError {
	return UnavailableError{
		ScyllaError: err,
		Consistency: b.ReadConsistency(),
		Required:    b.ReadInt(),
		Alive:       b.ReadInt(),
	}
}

// WriteTimeoutError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1076
type WriteTimeoutError struct {
	ScyllaError
	Consistency frame.Consistency
	Received    frame.Int
	BlockFor    frame.Int
	WriteType   frame.WriteType
}

func ParseWriteTimeoutError(b *frame.Buffer, err ScyllaError) WriteTimeoutError {
	return WriteTimeoutError{
		ScyllaError: err,
		Consistency: b.ReadConsistency(),
		Received:    b.ReadInt(),
		BlockFor:    b.ReadInt(),
		WriteType:   b.ReadWriteType(),
	}
}

// ReadTimeoutError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1108
type ReadTimeoutError struct {
	ScyllaError
	Consistency frame.Consistency
	Received    frame.Int
	BlockFor    frame.Int
	DataPresent bool
}

func ParseReadTimeoutError(b *frame.Buffer, err ScyllaError) ReadTimeoutError {
	return ReadTimeoutError{
		ScyllaError: err,
		Consistency: b.ReadConsistency(),
		Received:    b.ReadInt(),
		BlockFor:    b.ReadInt(),
		DataPresent: b.ReadByte() != 0,
	}
}

// ReadFailureError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1125
type ReadFailureError struct {
	ScyllaError
	Consistency frame.Consistency
	Received    frame.Int
	BlockFor    frame.Int
	NumFailures frame.Int
	DataPresent bool
}

func ParseReadFailureError(b *frame.Buffer, err ScyllaError) ReadFailureError {
	return ReadFailureError{
		ScyllaError: err,
		Consistency: b.ReadConsistency(),
		Received:    b.ReadInt(),
		BlockFor:    b.ReadInt(),
		NumFailures: b.ReadInt(),
		DataPresent: b.ReadByte() != 0,
	}
}

// FuncFailureError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1140
type FuncFailureError struct {
	ScyllaError
	Keyspace string
	Function string
	ArgTypes frame.StringList
}

func ParseFuncFailureError(b *frame.Buffer, err ScyllaError) FuncFailureError {
	return FuncFailureError{
		ScyllaError: err,
		Keyspace:    b.ReadString(),
		Function:    b.ReadString(),
		ArgTypes:    b.ReadStringList(),
	}
}

// WriteFailureError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1147
type WriteFailureError struct {
	ScyllaError
	Consistency frame.Consistency
	Received    frame.Int
	BlockFor    frame.Int
	NumFailures frame.Int
	WriteType   frame.WriteType
}

func ParseWriteFailureError(b *frame.Buffer, err ScyllaError) WriteFailureError {
	return WriteFailureError{
		ScyllaError: err,
		Consistency: b.ReadConsistency(),
		Received:    b.ReadInt(),
		BlockFor:    b.ReadInt(),
		NumFailures: b.ReadInt(),
		WriteType:   b.ReadWriteType(),
	}
}

// AlreadyExistsError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1187
type AlreadyExistsError struct {
	ScyllaError
	Keyspace string
	Table    string
}

func ParseAlreadyExistsError(b *frame.Buffer, err ScyllaError) AlreadyExistsError {
	return AlreadyExistsError{
		ScyllaError: err,
		Keyspace:    b.ReadString(),
		Table:       b.ReadString(),
	}
}

// UnpreparedError spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L1197
type UnpreparedError struct {
	ScyllaError
	UnknownID frame.ShortBytes
}

func ParseUnpreparedError(b *frame.Buffer, err ScyllaError) UnpreparedError {
	return UnpreparedError{
		ScyllaError: err,
		UnknownID:   b.ReadShortBytes(),
	}
}
