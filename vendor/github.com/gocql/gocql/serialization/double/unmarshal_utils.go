package double

import (
	"fmt"
	"reflect"
	"unsafe"
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal double: the length of the data should be 0 or 8")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal double: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecFloat64(p []byte, v *float64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 8:
		*v = decFloat64(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecFloat64R(p []byte, v **float64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(float64)
		}
	case 8:
		*v = decFloat64R(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Float64:
		return decReflectFloat32(p, v)
	default:
		return fmt.Errorf("failed to unmarshal double: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Float64:
		return decReflectFloat32R(p, v)
	default:
		return fmt.Errorf("failed to unmarshal double: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectFloat32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetFloat(0)
	case 8:
		v.SetFloat(decFloat64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectFloat32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 8:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetFloat(decFloat64(p))
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

func decFloat64(p []byte) float64 {
	return uint64ToFloat(decUint64(p))
}

func decFloat64R(p []byte) *float64 {
	return uint64ToFloatR(decUint64(p))
}

func uint64ToFloat(v uint64) float64 {
	return *(*float64)(unsafe.Pointer(&v))
}

func uint64ToFloatR(v uint64) *float64 {
	return (*float64)(unsafe.Pointer(&v))
}

func decUint64(p []byte) uint64 {
	return uint64(p[0])<<56 | uint64(p[1])<<48 | uint64(p[2])<<40 | uint64(p[3])<<32 | uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7])
}
