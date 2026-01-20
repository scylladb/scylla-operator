package decimal

import (
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/inf.v0"

	"github.com/gocql/gocql/serialization/varint"
)

func EncInfDec(v inf.Dec) ([]byte, error) {
	sign := v.Sign()
	if sign == 0 {
		return []byte{0, 0, 0, 0, 0}, nil
	}
	return append(encScale(v.Scale()), varint.EncBigIntRS(v.UnscaledBig())...), nil
}

func EncInfDecR(v *inf.Dec) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encInfDecR(v), nil
}

// EncString encodes decimal string which should contains `scale` and `unscaled` strings separated by `;`.
func EncString(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	vs := strings.Split(v, ";")
	if len(vs) != 2 {
		return nil, fmt.Errorf("failed to marshal decimal: invalid decimal string %s", v)
	}
	scale, err := strconv.ParseInt(vs[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal decimal: invalid decimal scale string %s", vs[0])
	}
	unscaleData, err := encUnscaledString(vs[1])
	if err != nil {
		return nil, err
	}
	return append(encScale64(scale), unscaleData...), nil
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncString(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Type().Kind() {
	case reflect.String:
		return encReflectString(v)
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal decimal: unsupported value type (%T)(%[1]v), supported types: ~string, inf.Dec, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal decimal: unsupported value type (%T)(%[1]v), supported types: ~string, inf.Dec, unsetColumn", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encReflectString(v reflect.Value) ([]byte, error) {
	val := v.String()
	if val == "" {
		return nil, nil
	}
	vs := strings.Split(val, ";")
	if len(vs) != 2 {
		return nil, fmt.Errorf("failed to marshal decimal: invalid decimal string (%T)(%[1]v)", v.Interface())
	}
	scale, err := strconv.ParseInt(vs[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal decimal: invalid decimal scale string (%T)(%s)", v.Interface(), vs[0])
	}
	unscaledData, err := encUnscaledString(vs[1])
	if err != nil {
		return nil, err
	}
	return append(encScale64(scale), unscaledData...), nil
}

func encInfDecR(v *inf.Dec) []byte {
	sign := v.Sign()
	if sign == 0 {
		return []byte{0, 0, 0, 0, 0}
	}
	return append(encScale(v.Scale()), varint.EncBigIntRS(v.UnscaledBig())...)
}

func encScale(v inf.Scale) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func encScale64(v int64) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func encUnscaledString(v string) ([]byte, error) {
	switch {
	case len(v) == 0:
		return nil, nil
	case len(v) <= 18:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal decimal: invalid unscaled string %s, %s", v, err)
		}
		return varint.EncInt64Ext(n), nil
	case len(v) <= 20:
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return varint.EncInt64Ext(n), nil
		}

		t, ok := new(big.Int).SetString(v, 10)
		if !ok {
			return nil, fmt.Errorf("failed to marshal decimal: invalid unscaled string %s", v)
		}
		return varint.EncBigIntRS(t), nil
	default:
		t, ok := new(big.Int).SetString(v, 10)
		if !ok {
			return nil, fmt.Errorf("failed to marshal decimal: invalid unscaled string %s", v)
		}
		return varint.EncBigIntRS(t), nil
	}
}
