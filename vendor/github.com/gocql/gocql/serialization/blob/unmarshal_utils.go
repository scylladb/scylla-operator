package blob

import (
	"fmt"
	"reflect"
)

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal blob: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecString(p []byte, v *string) error {
	if v == nil {
		return errNilReference(v)
	}
	*v = decString(p)
	return nil
}

func DecStringR(p []byte, v **string) error {
	if v == nil {
		return errNilReference(v)
	}
	*v = decStringR(p)
	return nil
}

func DecBytes(p []byte, v *[]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	*v = decBytes(p)
	return nil
}

func DecBytesR(p []byte, v **[]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	*v = decBytesR(p)
	return nil
}

func DecInterface(p []byte, v *interface{}) error {
	if v == nil {
		return errNilReference(v)
	}
	*v = decBytes(p)
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.String:
		v.SetString(decString(p))
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to marshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte", v.Interface())
		}
		v.SetBytes(decBytes(p))
	case reflect.Interface:
		v.Set(reflect.ValueOf(decBytes(p)))
	default:
		return fmt.Errorf("failed to unmarshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte", v.Interface())
	}
	return nil
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch ev := v.Type().Elem().Elem(); ev.Kind() {
	case reflect.String:
		return decReflectStringR(p, v)
	case reflect.Slice:
		if ev.Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to marshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte", v.Interface())
		}
		return decReflectBytesR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal blob: unsupported value type (%T)(%[1]v), supported types: ~string, ~[]byte", v.Interface())
	}
}

func decReflectStringR(p []byte, v reflect.Value) error {
	if len(p) == 0 {
		if p == nil {
			v.Elem().Set(reflect.Zero(v.Type().Elem()))
		} else {
			v.Elem().Set(reflect.New(v.Type().Elem().Elem()))
		}
		return nil
	}
	val := reflect.New(v.Type().Elem().Elem())
	val.Elem().SetString(string(p))
	v.Elem().Set(val)
	return nil
}

func decReflectBytesR(p []byte, v reflect.Value) error {
	if len(p) == 0 {
		if p == nil {
			v.Elem().Set(reflect.Zero(v.Elem().Type()))
		} else {
			val := reflect.New(v.Type().Elem().Elem())
			val.Elem().SetBytes(make([]byte, 0))
			v.Elem().Set(val)
		}
		return nil
	}
	tmp := make([]byte, len(p))
	copy(tmp, p)

	val := reflect.New(v.Type().Elem().Elem())
	val.Elem().SetBytes(tmp)
	v.Elem().Set(val)
	return nil
}

func decString(p []byte) string {
	if len(p) == 0 {
		return ""
	}
	return string(p)
}

func decStringR(p []byte) *string {
	if len(p) == 0 {
		if p == nil {
			return nil
		}
		return new(string)
	}
	tmp := string(p)
	return &tmp
}

func decBytes(p []byte) []byte {
	if len(p) == 0 {
		if p == nil {
			return nil
		}
		return make([]byte, 0)
	}
	tmp := make([]byte, len(p))
	copy(tmp, p)
	return tmp
}

func decBytesR(p []byte) *[]byte {
	if len(p) == 0 {
		if p == nil {
			return nil
		}
		tmp := make([]byte, 0)
		return &tmp
	}
	tmp := make([]byte, len(p))
	copy(tmp, p)
	return &tmp
}
