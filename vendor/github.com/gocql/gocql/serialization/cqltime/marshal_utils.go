package cqltime

import (
	"fmt"
	"reflect"
	"time"
)

const (
	maxValInt64 int64         = 86399999999999
	minValInt64 int64         = 0
	maxValDur   time.Duration = 86399999999999
	minValDur   time.Duration = 0
)

var (
	errOutRangeInt64 = fmt.Errorf("failed to marshal time: the (int64) should be in the range 0 to 86399999999999")
	errOutRangeDur   = fmt.Errorf("failed to marshal time: the (time.Duration) should be in the range 0 to 86399999999999")
)

func EncInt64(v int64) ([]byte, error) {
	if v > maxValInt64 || v < minValInt64 {
		return nil, errOutRangeInt64
	}
	return encInt64(v), nil
}

func EncInt64R(v *int64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt64(*v)
}

func EncDuration(v time.Duration) ([]byte, error) {
	if v > maxValDur || v < minValDur {
		return nil, errOutRangeDur
	}
	return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}, nil
}

func EncDurationR(v *time.Duration) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncDuration(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Int64:
		val := v.Int()
		if val > maxValInt64 || val < minValInt64 {
			return nil, fmt.Errorf("failed to marshal time: the (%T) should be in the range 0 to 86399999999999", v.Interface())
		}
		return encInt64(val), nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal time: unsupported value type (%T)(%[1]v)", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal time: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encInt64(v int64) []byte {
	return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}
