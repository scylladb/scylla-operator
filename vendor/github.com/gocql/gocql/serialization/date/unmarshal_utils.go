package date

import (
	"fmt"
	"math"
	"reflect"
	"time"
)

const (
	negInt64       = int64(-1) << 32
	zeroDate       = "-5877641-06-23"
	zeroMS   int64 = -185542587187200000
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal date: the length of the data should be 0 or 4")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal date: can not unmarshal into nil reference (%T)(%[1]v))", v)
}

func DecInt32(p []byte, v *int32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 4:
		*v = decInt32(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt32R(p []byte, v **int32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int32)
		}
	case 4:
		val := decInt32(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt64(p []byte, v *int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = zeroMS
	case 4:
		*v = decMilliseconds(p)
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
			val := zeroMS
			*v = &val
		}
	case 4:
		val := decMilliseconds(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint32(p []byte, v *uint32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 4:
		*v = decUint32(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint32R(p []byte, v **uint32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(uint32)
		}
	case 4:
		val := decUint32(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecString(p []byte, v *string) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = ""
		} else {
			*v = zeroDate
		}
	case 4:
		*v = decString(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecStringR(p []byte, v **string) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			val := zeroDate
			*v = &val
		}
	case 4:
		val := decString(p)
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
		*v = minDate
	case 4:
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
			val := minDate
			*v = &val
		}
	case 4:
		val := decTime(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal date: can not unmarshal into nil reference (%T)(%[1]v))", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Int32:
		return decReflectInt32(p, v)
	case reflect.Int64:
		return decReflectInt64(p, v)
	case reflect.Uint32:
		return decReflectUint32(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal date: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectInt32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 4:
		v.SetInt(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInt64(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(zeroMS)
	case 4:
		v.SetInt(decMilliseconds(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 4:
		v.SetUint(decUint64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectString(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.SetString("")
		} else {
			v.SetString(zeroDate)
		}
	case 4:
		v.SetString(decString(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal date: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int32:
		return decReflectInt32R(p, v)
	case reflect.Int64:
		return decReflectInt64R(p, v)
	case reflect.Uint32:
		return decReflectUint32R(p, v)
	case reflect.String:
		return decReflectStringR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal date: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectInt32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(decInt64(p))
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInt64R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		var val reflect.Value
		if p == nil {
			val = reflect.Zero(v.Type().Elem())
		} else {
			val = reflect.New(v.Type().Elem().Elem())
			val.Elem().SetInt(zeroMS)
			v.Elem().Set(val)
		}
		v.Elem().Set(val)
	case 4:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetInt(decMilliseconds(p))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(decUint64(p))
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectStringR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		var val reflect.Value
		if p == nil {
			val = reflect.Zero(v.Type().Elem())
		} else {
			val = reflect.New(v.Type().Elem().Elem())
			val.Elem().SetString(zeroDate)
		}
		v.Elem().Set(val)
	case 4:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetString(decString(p))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectNullableR(p []byte, v reflect.Value) reflect.Value {
	if p == nil {
		return reflect.Zero(v.Elem().Type())
	}
	return reflect.New(v.Type().Elem().Elem())
}

func decInt32(p []byte) int32 {
	return int32(p[0])<<24 | int32(p[1])<<16 | int32(p[2])<<8 | int32(p[3])
}

func decInt64(p []byte) int64 {
	if p[0] > math.MaxInt8 {
		return negInt64 | int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3])
	}
	return int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3])
}

func decMilliseconds(p []byte) int64 {
	return (int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3]) - centerEpoch) * millisecondsInADay
}

func decUint32(p []byte) uint32 {
	return uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
}

func decUint64(p []byte) uint64 {
	return uint64(p[0])<<24 | uint64(p[1])<<16 | uint64(p[2])<<8 | uint64(p[3])
}

func decString(p []byte) string {
	return decTime(p).Format("2006-01-02")
}

func decTime(p []byte) time.Time {
	return time.UnixMilli(decMilliseconds(p)).UTC()
}
