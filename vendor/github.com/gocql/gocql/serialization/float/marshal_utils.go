package float

import (
	"fmt"
	"reflect"
	"unsafe"
)

func EncFloat32(v float32) ([]byte, error) {
	return encFloat32(v), nil
}

func EncFloat32R(v *float32) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encFloat32R(v), nil
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Float32:
		return encFloat32(float32(v.Float())), nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal float: unsupported value type (%T)(%[1]v), supported types: ~float32, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal float: unsupported value type (%T)(%[1]v), supported types: ~float32, unsetColumn", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encFloat32(v float32) []byte {
	return encUint32(floatToUint(v))
}

func encFloat32R(v *float32) []byte {
	return encUint32(floatToUintR(v))
}

func encUint32(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func floatToUint(v float32) uint32 {
	return *(*uint32)(unsafe.Pointer(&v))
}

func floatToUintR(v *float32) uint32 {
	return *(*uint32)(unsafe.Pointer(v))
}
