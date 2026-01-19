package double

import (
	"fmt"
	"reflect"
	"unsafe"
)

func EncFloat64(v float64) ([]byte, error) {
	return encFloat64(v), nil
}

func EncFloat64R(v *float64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encFloat64R(v), nil
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Float64:
		return encFloat64(v.Float()), nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal double: unsupported value type (%T)(%[1]v), supported types: ~float64, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal double: unsupported value type (%T)(%[1]v), supported types: ~float64, unsetColumn", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encFloat64(v float64) []byte {
	return encUint64(floatToUint(v))
}

func encFloat64R(v *float64) []byte {
	return encUint64(floatToUintR(v))
}

func encUint64(v uint64) []byte {
	return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func floatToUint(v float64) uint64 {
	return *(*uint64)(unsafe.Pointer(&v))
}

func floatToUintR(v *float64) uint64 {
	return *(*uint64)(unsafe.Pointer(v))
}
