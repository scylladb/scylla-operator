package blob

import (
	"fmt"
	"reflect"
)

func EncString(v string) ([]byte, error) {
	return encString(v), nil
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encString(*v), nil
}

func EncBytes(v []byte) ([]byte, error) {
	return v, nil
}

func EncBytesR(v *[]byte) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return *v, nil
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.String:
		return encString(v.String()), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("failed to marshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte, unsetColumn", v.Interface())
		}
		return EncBytes(v.Bytes())
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte, unsetColumn", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encString(v string) []byte {
	if v == "" {
		return make([]byte, 0)
	}
	return []byte(v)
}
