package varint

import (
	"fmt"
	"math/big"
	"reflect"
	"strconv"
)

const supportedTypes = "~int8, ~int16, ~int32, ~int64, ~int, ~uint8, ~uint16, ~uint32, ~uint64, ~uint, ~string, big.Int"

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Type().Kind() {
	case reflect.Int8:
		return EncInt8(int8(v.Int()))
	case reflect.Int16:
		return EncInt16(int16(v.Int()))
	case reflect.Int32:
		return EncInt32(int32(v.Int()))
	case reflect.Int, reflect.Int64:
		return EncInt64(v.Int())
	case reflect.Uint8:
		return EncUint8(uint8(v.Uint()))
	case reflect.Uint16:
		return EncUint16(uint16(v.Uint()))
	case reflect.Uint32:
		return EncUint32(uint32(v.Uint()))
	case reflect.Uint, reflect.Uint64:
		return EncUint64(v.Uint())
	case reflect.String:
		return encReflectString(v)
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal varint: unsupported value type (%T)(%[1]v), supported types: %s, unsetColumn", v.Interface(), supportedTypes)

	default:
		return nil, fmt.Errorf("failed to marshal varint: unsupported value type (%T)(%[1]v), supported types: %s, unsetColumn", v.Interface(), supportedTypes)
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
	switch {
	case len(val) == 0:
		return nil, nil
	case len(val) <= 18:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal varint: can not marshal (%T)(%[1]v), %s", v.Interface(), err)
		}
		return EncInt64Ext(n), nil
	case len(val) <= 20:
		n, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return EncInt64Ext(n), nil
		}

		t, ok := new(big.Int).SetString(val, 10)
		if !ok {
			return nil, fmt.Errorf("failed to marshal varint: can not marshal (%T)(%[1]v)", v.Interface())
		}
		return EncBigIntRS(t), nil
	default:
		t, ok := new(big.Int).SetString(val, 10)
		if !ok {
			return nil, fmt.Errorf("failed to marshal varint: can not marshal (%T)(%[1]v)", v.Interface())
		}
		return EncBigIntRS(t), nil
	}
}
