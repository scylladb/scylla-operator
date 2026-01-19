package tinyint

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
)

const (
	negInt16 = int16(-1) << 8
	negInt32 = int32(-1) << 8
	negInt64 = int64(-1) << 8
	negInt   = int(-1) << 8
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal tinyint: the length of the data should less or equal then 1")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal tinyint: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecInt8(p []byte, v *int8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = int8(p[0])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt8R(p []byte, v **int8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int8)
		}
	case 1:
		val := int8(p[0])
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt16(p []byte, v *int16) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = decInt16(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt16R(p []byte, v **int16) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int16)
		}
	case 1:
		val := decInt16(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt32(p []byte, v *int32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = decInt32(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt32R(p []byte, v **int32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int32)
		}
	case 1:
		val := decInt32(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt64(p []byte, v *int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = decInt64(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt64R(p []byte, v **int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int64)
		}
	case 1:
		val := decInt64(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecInt(p []byte, v *int) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = decInt(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecIntR(p []byte, v **int) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int)
		}
	case 1:
		val := decInt(p)
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint8(p []byte, v *uint8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = p[0]
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint8R(p []byte, v **uint8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(uint8)
		}
	case 1:
		val := p[0]
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint16(p []byte, v *uint16) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = uint16(p[0])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint16R(p []byte, v **uint16) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(uint16)
		}
	case 1:
		val := uint16(p[0])
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint32(p []byte, v *uint32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = uint32(p[0])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint32R(p []byte, v **uint32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(uint32)
		}
	case 1:
		val := uint32(p[0])
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint64(p []byte, v *uint64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = uint64(p[0])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint64R(p []byte, v **uint64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(uint64)
		}
	case 1:
		val := uint64(p[0])
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUint(p []byte, v *uint) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = uint(p[0])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecUintR(p []byte, v **uint) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(uint)
		}
	case 1:
		val := uint(p[0])
		*v = &val
	default:
		return errWrongDataLen
	}
	return nil
}

func DecString(p []byte, v *string) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = ""
		} else {
			*v = "0"
		}
	case 1:
		*v = strconv.FormatInt(decInt64(p), 10)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecStringR(p []byte, v **string) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			val := "0"
			*v = &val
		}
	case 1:
		*v = new(string)
		**v = strconv.FormatInt(decInt64(p), 10)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecBigInt(p []byte, v *big.Int) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		v.SetInt64(0)
	case 1:
		v.SetInt64(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func DecBigIntR(p []byte, v **big.Int) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = big.NewInt(0)
		}
	case 1:
		*v = big.NewInt(decInt64(p))
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
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return decReflectInts(p, v)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return decReflectUints(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal tinyint: unsupported value type (%T)(%[1]v), supported types: %s", v.Interface(), supportedTypes)
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return decReflectIntsR(p, v)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return decReflectUintsR(p, v)
	case reflect.String:
		return decReflectStringR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal tinyint: unsupported value type (%T)(%[1]v), supported types: %s", v.Interface(), supportedTypes)
	}
}

func decReflectInts(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 1:
		v.SetInt(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUints(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 1:
		v.SetUint(uint64(p[0]))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectString(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.SetString("")
		} else {
			v.SetString("0")
		}
	case 1:
		v.SetString(strconv.FormatInt(decInt64(p), 10))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectIntsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 1:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetInt(decInt64(p))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUintsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 1:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetUint(uint64(p[0]))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectStringR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		var val reflect.Value
		if p == nil {
			val = reflect.Zero(v.Type().Elem())
		} else {
			val = reflect.New(v.Type().Elem().Elem())
			val.Elem().SetString("0")
		}
		v.Elem().Set(val)
	case 1:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetString(strconv.FormatInt(decInt64(p), 10))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectNullableR(p []byte, v reflect.Value) reflect.Value {
	if p == nil {
		return reflect.Zero(v.Elem().Type())
	}
	return reflect.New(v.Type().Elem().Elem())
}

func decInt16(p []byte) int16 {
	if p[0] > math.MaxInt8 {
		return negInt16 | int16(p[0])
	}
	return int16(p[0])
}

func decInt32(p []byte) int32 {
	if p[0] > math.MaxInt8 {
		return negInt32 | int32(p[0])
	}
	return int32(p[0])
}

func decInt64(p []byte) int64 {
	if p[0] > math.MaxInt8 {
		return negInt64 | int64(p[0])
	}
	return int64(p[0])
}

func decInt(p []byte) int {
	if p[0] > math.MaxInt8 {
		return negInt | int(p[0])
	}
	return int(p[0])
}
