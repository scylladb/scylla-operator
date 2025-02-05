package boolean

import (
	"fmt"
	"reflect"
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal boolean: the length of the data should be 0 or 1")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal boolean: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecBool(p []byte, v *bool) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = false
	case 1:
		*v = decBool(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecBoolR(p []byte, v **bool) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(bool)
		}
	case 1:
		val := decBool(p)
		*v = &val
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
	case reflect.Bool:
		return decReflectBool(p, v)
	default:
		return fmt.Errorf("failed to unmarshal boolean: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Bool:
		return decReflectBoolR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal boolean: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectBool(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetBool(false)
	case 1:
		v.SetBool(decBool(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectBoolR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Elem().Set(reflect.Zero(v.Type().Elem()))
		} else {
			val := reflect.New(v.Type().Elem().Elem())
			v.Elem().Set(val)
		}
	case 1:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetBool(decBool(p))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decBool(p []byte) bool {
	return p[0] != 0
}
