package boolean

import (
	"fmt"
	"reflect"
)

func EncBool(v bool) ([]byte, error) {
	return encBool(v), nil
}

func EncBoolR(v *bool) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encBool(*v), nil
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Bool:
		return encBool(v.Bool()), nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal boolean: unsupported value type (%T)(%[1]v)", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal boolean: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encBool(v bool) []byte {
	if v {
		return []byte{1}
	}
	return []byte{0}
}
