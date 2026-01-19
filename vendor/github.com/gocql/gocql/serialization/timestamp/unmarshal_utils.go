package timestamp

import (
	"fmt"
	"reflect"
	"time"
)

var (
	errWrongDataLen = fmt.Errorf("failed to unmarshal timestamp: the length of the data should be 0 or 8")
)

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal timestamp: can not unmarshal into nil reference (%T)(%[1]v))", v)
}

func DecInt64(p []byte, v *int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 8:
		*v = decInt64(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt64R(p []byte, v **int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int64)
		}
	case 8:
		val := decInt64(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecTime(p []byte, v *time.Time) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		// supposed to be zero timestamp `time.UnixMilli(0).UTC()`, but for backward compatibility mapped to zero time
		*v = time.Time{}
	case 8:
		*v = decTime(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecTimeR(p []byte, v **time.Time) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			// supposed to be zero timestamp `time.UnixMilli(0).UTC()`, but for backward compatibility mapped to zero time
			val := time.Time{}
			*v = &val
		}
	case 8:
		val := decTime(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal timestamp: can not unmarshal into nil reference (%T)(%[1]v))", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Int64:
		return decReflectInt64(p, v)
	default:
		return fmt.Errorf("failed to unmarshal timestamp: unsupported value type (%T)(%[1]v), supported types: ~int64, time.Time", v.Interface())
	}
}

func decReflectInt64(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 8:
		v.SetInt(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal timestamp: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int64:
		return decReflectIntsR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal timestamp: unsupported value type (%T)(%[1]v), supported types: ~int64, time.Time", v.Interface())
	}
}

func decReflectIntsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Elem().Set(reflect.Zero(v.Elem().Type()))
		} else {
			v.Elem().Set(reflect.New(v.Type().Elem().Elem()))
		}
	case 8:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetInt(decInt64(p))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decInt64(p []byte) int64 {
	return int64(p[0])<<56 | int64(p[1])<<48 | int64(p[2])<<40 | int64(p[3])<<32 | int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
}

func decTime(p []byte) time.Time {
	msec := decInt64(p)
	return time.Unix(msec/1e3, (msec%1e3)*1e6).UTC()
}
