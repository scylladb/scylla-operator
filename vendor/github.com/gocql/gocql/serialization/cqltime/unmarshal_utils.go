package cqltime

import (
	"fmt"
	"reflect"
	"time"
)

var (
	errWrongDataLen      = fmt.Errorf("failed to unmarshal time: the length of the data should be 0 or 8")
	errDataOutRangeInt64 = fmt.Errorf("failed to unmarshal time: (int64) the data should be in the range 0 to 86399999999999")
	errDataOutRangeDur   = fmt.Errorf("failed to unmarshal time: (time.Duration) the data should be in the range 0 to 86399999999999")
)

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal time: can not unmarshal into nil reference (%T)(%[1]v))", v)
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
		if *v > maxValInt64 || *v < minValInt64 {
			return errDataOutRangeInt64
		}
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
		if val > maxValInt64 || val < minValInt64 {
			return errDataOutRangeInt64
		}
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecDuration(p []byte, v *time.Duration) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 8:
		*v = decDur(p)
		if *v > maxValDur || *v < minValDur {
			return errDataOutRangeDur
		}
	default:
		return errWrongDataLen
	}
	return nil
}

func DecDurationR(p []byte, v **time.Duration) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(time.Duration)
		}
	case 8:
		val := decDur(p)
		if val > maxValDur || val < minValDur {
			return errDataOutRangeDur
		}
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal time: can not unmarshal into nil reference (%T)(%[1]v))", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Int64, reflect.Int:
		return decReflectInt64(p, v)
	default:
		return fmt.Errorf("failed to unmarshal time: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectInt64(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 8:
		val := decInt64(p)
		if val > maxValInt64 || val < minValInt64 {
			return fmt.Errorf("failed to unmarshal time: (%T) the data should be in the range 0 to 86399999999999", v.Interface())
		}
		v.SetInt(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal time: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int64, reflect.Int:
		return decReflectIntsR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal time: unsupported value type (%T)(%[1]v)", v.Interface())
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
		vv := decInt64(p)
		if vv > maxValInt64 || vv < minValInt64 {
			return fmt.Errorf("failed to unmarshal time: (%T) the data should be in the range 0 to 86399999999999", v.Interface())
		}
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetInt(vv)
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decInt64(p []byte) int64 {
	return int64(p[0])<<56 | int64(p[1])<<48 | int64(p[2])<<40 | int64(p[3])<<32 | int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
}

func decDur(p []byte) time.Duration {
	return time.Duration(p[0])<<56 | time.Duration(p[1])<<48 | time.Duration(p[2])<<40 | time.Duration(p[3])<<32 | time.Duration(p[4])<<24 | time.Duration(p[5])<<16 | time.Duration(p[6])<<8 | time.Duration(p[7])
}
