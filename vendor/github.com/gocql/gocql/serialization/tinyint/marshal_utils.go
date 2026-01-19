package tinyint

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
)

const supportedTypes = "~int8, ~int16, ~int32, ~int64, ~int, ~uint8, ~uint16, ~uint32, ~uint64, ~uint, ~string, big.Int"

var (
	maxBigInt = big.NewInt(math.MaxInt8)
	minBigInt = big.NewInt(math.MinInt8)
)

func EncInt8(v int8) ([]byte, error) {
	return []byte{byte(v)}, nil
}

func EncInt8R(v *int8) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt8(*v)
}

func EncInt16(v int16) ([]byte, error) {
	if v > math.MaxInt8 || v < math.MinInt8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncInt16R(v *int16) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt16(*v)
}

func EncInt32(v int32) ([]byte, error) {
	if v > math.MaxInt8 || v < math.MinInt8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncInt32R(v *int32) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt32(*v)
}

func EncInt64(v int64) ([]byte, error) {
	if v > math.MaxInt8 || v < math.MinInt8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncInt64R(v *int64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt64(*v)
}

func EncInt(v int) ([]byte, error) {
	if v > math.MaxInt8 || v < math.MinInt8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncIntR(v *int) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt(*v)
}

func EncUint8(v uint8) ([]byte, error) {
	return []byte{v}, nil
}

func EncUint8R(v *uint8) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncUint8(*v)
}

func EncUint16(v uint16) ([]byte, error) {
	if v > math.MaxUint8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncUint16R(v *uint16) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncUint16(*v)
}

func EncUint32(v uint32) ([]byte, error) {
	if v > math.MaxUint8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncUint32R(v *uint32) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncUint32(*v)
}

func EncUint64(v uint64) ([]byte, error) {
	if v > math.MaxUint8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncUint64R(v *uint64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncUint64(*v)
}

func EncUint(v uint) ([]byte, error) {
	if v > math.MaxUint8 {
		return nil, fmt.Errorf("failed to marshal tinyint: value %#v out of range", v)
	}
	return []byte{byte(v)}, nil
}

func EncUintR(v *uint) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncUint(*v)
}

func EncBigInt(v big.Int) ([]byte, error) {
	if v.Cmp(maxBigInt) == 1 || v.Cmp(minBigInt) == -1 {
		return nil, fmt.Errorf("failed to marshal tinyint: value (%T)(%s) out of range", v, v.String())
	}
	return []byte{byte(v.Int64())}, nil
}

func EncBigIntR(v *big.Int) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncBigInt(*v)
}

func EncString(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}

	n, err := strconv.ParseInt(v, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tinyint: can not marshal (%T)(%[1]v) %s", v, err)
	}
	return []byte{byte(n)}, nil
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncString(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Int8:
		return []byte{byte(v.Int())}, nil
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16:
		val := v.Int()
		if val > math.MaxInt8 || val < math.MinInt8 {
			return nil, fmt.Errorf("failed to marshal tinyint: value (%T)(%[1]v) out of range", v.Interface())
		}
		return []byte{byte(val)}, nil
	case reflect.Uint8:
		return []byte{byte(v.Uint())}, nil
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16:
		val := v.Uint()
		if val > math.MaxUint8 {
			return nil, fmt.Errorf("failed to marshal tinyint: value (%T)(%[1]v) out of range", v.Interface())
		}
		return []byte{byte(val)}, nil
	case reflect.String:
		val := v.String()
		if val == "" {
			return nil, nil
		}

		n, err := strconv.ParseInt(val, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tinyint: can not marshal (%T)(%[1]v) %s", v.Interface(), err)
		}
		return []byte{byte(n)}, nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal tinyint: unsupported value type (%T)(%[1]v), supported types: %s, unsetColumn", v.Interface(), supportedTypes)
	default:
		return nil, fmt.Errorf("failed to marshal tinyint: unsupported value type (%T)(%[1]v), supported types: %s, unsetColumn", v.Interface(), supportedTypes)
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}
