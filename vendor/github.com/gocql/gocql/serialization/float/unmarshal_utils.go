package float

import (
	"fmt"
	"reflect"
	"unsafe"
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal float: the length of the data should be 0 or 4")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal float: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecFloat32(p []byte, v *float32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 4:
		*v = decFloat32(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecFloat32R(p []byte, v **float32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(float32)
		}
	case 4:
		*v = decFloat32R(p)
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
	case reflect.Float32:
		return decReflectFloat32(p, v)
	default:
		return fmt.Errorf("failed to unmarshal float: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Float32:
		return decReflectFloat32R(p, v)
	default:
		return fmt.Errorf("failed to unmarshal float: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectFloat32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetFloat(0)
	case 4:
		v.SetFloat(float64(decFloat32(p)))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectFloat32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 4:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetFloat(float64(decFloat32(p)))
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

func decFloat32(p []byte) float32 {
	return uint32ToFloat(decUint32(p))
}

func decFloat32R(p []byte) *float32 {
	return uint32ToFloatR(decUint32(p))
}

func uint32ToFloat(v uint32) float32 {
	return *(*float32)(unsafe.Pointer(&v))
}

func uint32ToFloatR(v uint32) *float32 {
	return (*float32)(unsafe.Pointer(&v))
}

func decUint32(p []byte) uint32 {
	return uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
}
