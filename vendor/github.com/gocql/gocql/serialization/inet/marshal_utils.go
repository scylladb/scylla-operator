package inet

import (
	"fmt"
	"net"
	"reflect"
)

func EncBytes(v []byte) ([]byte, error) {
	switch len(v) {
	case 0:
		if v == nil {
			return nil, nil
		}
		return make([]byte, 0), nil
	case 4:
		tmp := make([]byte, 4)
		copy(tmp, v)
		return tmp, nil
	case 16:
		tmp := make([]byte, 16)
		copy(tmp, v)
		return tmp, nil
	default:
		return nil, fmt.Errorf("failed to marshal inet: the ([]byte) length can be 0,4,16")
	}
}

func EncBytesR(v *[]byte) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncBytes(*v)
}

func EncNetIP(v net.IP) ([]byte, error) {
	switch len(v) {
	case 0:
		if v == nil {
			return nil, nil
		}
		return make([]byte, 0), nil
	case 4, 16:
		t := v.To4()
		if t == nil {
			return v.To16(), nil
		}
		return t, nil
	default:
		return nil, fmt.Errorf("failed to marshal inet: the (net.IP) length can be 0,4,16")
	}
}

func EncNetIPr(v *net.IP) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncNetIP(*v)
}

func EncArray16(v [16]byte) ([]byte, error) {
	tmp := make([]byte, 16)
	copy(tmp, v[:])
	return tmp, nil
}

func EncArray16R(v *[16]byte) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncArray16(*v)
}

func EncArray4(v [4]byte) ([]byte, error) {
	tmp := make([]byte, 4)
	copy(tmp, v[:])
	return tmp, nil
}

func EncArray4R(v *[4]byte) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncArray4(*v)
}

func EncString(v string) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	b := net.ParseIP(v)
	if b != nil {
		t := b.To4()
		if t == nil {
			return b.To16(), nil
		}
		return t, nil
	}
	return nil, fmt.Errorf("failed to marshal inet: invalid IP string %s", v)
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncString(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Array:
		if l := v.Len(); v.Type().Elem().Kind() != reflect.Uint8 || (l != 16 && l != 4) {
			return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		nv := reflect.New(v.Type())
		nv.Elem().Set(v)
		return nv.Elem().Bytes(), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return encReflectBytes(v)
	case reflect.String:
		return encReflectString(v)
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	switch ev := v.Elem(); ev.Kind() {
	case reflect.Array:
		if l := v.Len(); ev.Type().Elem().Kind() != reflect.Uint8 || (l != 16 && l != 4) {
			return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return v.Elem().Bytes(), nil
	case reflect.Slice:
		if ev.Type().Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return encReflectBytes(ev)
	case reflect.String:
		return encReflectString(ev)
	default:
		return nil, fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func encReflectString(v reflect.Value) ([]byte, error) {
	val := v.String()
	if len(val) == 0 {
		return nil, nil
	}
	b := net.ParseIP(val)
	if b != nil {
		t := b.To4()
		if t == nil {
			return b.To16(), nil
		}
		return t, nil
	}
	return nil, fmt.Errorf("failed to marshal inet: invalid IP string (%T)(%[1]v)", v.Interface())
}

func encReflectBytes(v reflect.Value) ([]byte, error) {
	val := v.Bytes()
	switch len(val) {
	case 0:
		if val == nil {
			return nil, nil
		}
		return make([]byte, 0), nil
	case 4:
		tmp := make([]byte, 4)
		copy(tmp, val)
		return tmp, nil
	case 16:
		tmp := make([]byte, 16)
		copy(tmp, val)
		return tmp, nil
	default:
		return nil, fmt.Errorf("failed to marshal inet: the (%T) length can be 0,4,16", v.Interface())
	}
}
