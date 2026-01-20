package timestamp

import (
	"fmt"
	"reflect"
	"time"
)

var (
	maxTimestamp = time.Date(292278994, 8, 17, 7, 12, 55, 807*1000000, time.UTC)
	minTimestamp = time.Date(-292275055, 5, 16, 16, 47, 4, 192*1000000, time.UTC)
)

func EncInt64(v int64) ([]byte, error) {
	return encInt64(v), nil
}

func EncInt64R(v *int64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt64(*v)
}

func EncTime(v time.Time) ([]byte, error) {
	if v.After(maxTimestamp) || v.Before(minTimestamp) {
		return nil, fmt.Errorf("failed to marshal timestamp: the (%T)(%s) value should be in the range from -292275055-05-16T16:47:04.192Z to 292278994-08-17T07:12:55.807", v, v.Format(time.RFC3339Nano))
	}
	// It supposed to be v.UTC().UnixMilli(), for backward compatibility map `time.Time{}` to nil value
	if v.IsZero() {
		return make([]byte, 0), nil
	}
	ms := v.UTC().UnixMilli()
	return []byte{byte(ms >> 56), byte(ms >> 48), byte(ms >> 40), byte(ms >> 32), byte(ms >> 24), byte(ms >> 16), byte(ms >> 8), byte(ms)}, nil
}

func EncTimeR(v *time.Time) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncTime(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Int64:
		return encInt64(v.Int()), nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal timestamp: unsupported value type (%T)(%[1]v), supported types: ~int64, time.Time, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal timestamp: unsupported value type (%T)(%[1]v), supported types: ~int64, time.Time, unsetColumn", v.Interface())
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
