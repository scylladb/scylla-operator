package uuid

import (
	"fmt"
	"reflect"
	"strings"
)

func EncArray(v [16]byte) ([]byte, error) {
	return v[:], nil
}

func EncArrayR(v *[16]byte) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return v[:], nil
}

func EncSlice(v []byte) ([]byte, error) {
	switch len(v) {
	case 0:
		if v == nil {
			return nil, nil
		}
		return make([]byte, 0), nil
	case 16:
		return v, nil
	default:
		return nil, fmt.Errorf("failed to marshal uuid: the ([]byte) length should be 0 or 16")
	}
}

func EncSliceR(v *[]byte) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncSlice(*v)
}

func EncString(v string) ([]byte, error) {
	return encString(v)
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encString(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Array:
		if v.Type().Elem().Kind() != reflect.Uint8 || v.Len() != 16 {
			return nil, fmt.Errorf("failed to marshal uuid: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
		}
		nv := reflect.New(v.Type())
		nv.Elem().Set(v)
		return nv.Elem().Bytes(), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("failed to marshal uuid: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
		}
		return encReflectBytes(v)
	case reflect.String:
		return encReflectString(v)
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal uuid: timeuuid value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal uuid: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	switch ev := v.Elem(); ev.Kind() {
	case reflect.Array:
		if ev.Type().Elem().Kind() != reflect.Uint8 || ev.Len() != 16 {
			return nil, fmt.Errorf("failed to marshal uuid: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
		}
		return v.Elem().Bytes(), nil
	case reflect.Slice:
		if ev.Type().Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("failed to marshal uuid: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
		}
		return encReflectBytes(ev)
	case reflect.String:
		return encReflectString(ev)
	default:
		return nil, fmt.Errorf("failed to marshal uuid: unsupported value type (%T)(%[1]v), supported types: ~[]byte, ~[16]byte, ~string, unsetColumn", v.Interface())
	}
}

func encReflectBytes(rv reflect.Value) ([]byte, error) {
	switch rv.Len() {
	case 0:
		if rv.IsNil() {
			return nil, nil
		}
		return make([]byte, 0), nil
	case 16:
		return rv.Bytes(), nil
	default:
		return nil, fmt.Errorf("failed to marshal uuid: the (%T) length should be 0 or 16", rv.Interface())
	}
}

// encReflectString encodes uuid strings via reflect package.
// The following code was taken from the `Parse` function of the "github.com/google/uuid" package.
func encReflectString(v reflect.Value) ([]byte, error) {
	s := v.String()
	switch len(s) {
	case 45: // urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		if !strings.EqualFold(s[:9], "urn:uuid:") {
			return nil, fmt.Errorf("failed to marshal uuid: the (%T) have invalid urn prefix: %q", v.Interface(), s[:9])
		}
		s = s[9:]
	case 38: // {xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}
		s = s[1:]
	case 36: // xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	case 32: // xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
		var ok bool
		data := make([]byte, 16)
		for i := range data {
			data[i], ok = xtob(s[i*2], s[i*2+1])
			if !ok {
				return nil, fmt.Errorf("failed to marshal uuid: the (%T) have invalid UUID format: %q", v.Interface(), s)
			}
		}
		return data, nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to marshal uuid: the (%T) length can be 0,32,36,38,45", v.Interface())
	}

	// s is now at least 36 bytes long
	// it must be of the form  xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return nil, fmt.Errorf("failed to marshal uuid: the (%T) have invalid UUID format: %q", v.Interface(), s)
	}
	data := make([]byte, 16)
	for i, x := range [16]int{
		0, 2, 4, 6,
		9, 11,
		14, 16,
		19, 21,
		24, 26, 28, 30, 32, 34,
	} {
		b, ok := xtob(s[x], s[x+1])
		if !ok {
			return nil, fmt.Errorf("failed to marshal uuid: the (%T) have invalid UUID format: %q", v.Interface(), b)
		}
		data[i] = b
	}
	return data, nil
}

// encString encodes uuid strings.
// The following code was taken from the `Parse` function of the "github.com/google/uuid" package.
func encString(s string) ([]byte, error) {
	switch len(s) {
	case 45: // urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		if !strings.EqualFold(s[:9], "urn:uuid:") {
			return nil, fmt.Errorf("failed to marshal uuid: (string) have invalid urn prefix: %q", s[:9])
		}
		s = s[9:]
	case 38: // {xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}
		s = s[1:]
	case 36: // xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	case 32: // xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
		var ok bool
		data := make([]byte, 16)
		for i := range data {
			data[i], ok = xtob(s[i*2], s[i*2+1])
			if !ok {
				return nil, fmt.Errorf("failed to marshal uuid: the (string) have invalid UUID format: %q", s)
			}
		}
		return data, nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to marshal uuid: the (string) length can be 0,32,36,38,45")
	}

	// s is now at least 36 bytes long
	// it must be of the form  xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return nil, fmt.Errorf("failed to marshal uuid: the (string) have invalid UUID format: %q", s)
	}
	data := make([]byte, 16)
	for i, x := range [16]int{
		0, 2, 4, 6,
		9, 11,
		14, 16,
		19, 21,
		24, 26, 28, 30, 32, 34,
	} {
		b, ok := xtob(s[x], s[x+1])
		if !ok {
			return nil, fmt.Errorf("failed to marshal uuid: the (string) have invalid UUID format: %q", b)
		}
		data[i] = b
	}
	return data, nil
}

// xtob converts hex characters x1 and x2 into a byte.
// The following code was taken from the "github.com/google/uuid" package.
func xtob(x1, x2 byte) (byte, bool) {
	b1 := xvalues[x1]
	b2 := xvalues[x2]
	return (b1 << 4) | b2, b1 != 255 && b2 != 255
}

// xvalues returns the value of a byte as a hexadecimal digit or 255.
// The following code was taken from the "github.com/google/uuid" package.
var xvalues = [256]byte{
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 255, 255, 255, 255, 255, 255,
	255, 10, 11, 12, 13, 14, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 10, 11, 12, 13, 14, 15, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
}
