package smallint

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
)

const (
	negInt32 = int32(-1) << 16
	negInt64 = int64(-1) << 16
	negInt   = int(-1) << 16
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal smallint: the length of the data should be 0 or 2")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal smallint: can not unmarshal into nil reference (%T)(%[1]v))", v)
}

func DecInt8(p []byte, v *int8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 2:
		val := decInt16(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into int8, the data should be in the int8 range")
		}
		*v = int8(val)
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
	case 2:
		val := decInt16(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into int8, the data should be in the int8 range")
		}
		tmp := int8(val)
		*v = &tmp
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
	case 2:
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
	case 2:
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
	case 2:
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
	case 2:
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
	case 2:
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
	case 2:
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
	case 2:
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
	case 2:
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
	case 2:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into uint8, the data should be in the uint8 range")
		}
		*v = p[1]
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
	case 2:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into uint8, the data should be in the uint8 range")
		}
		val := p[1]
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
	case 2:
		*v = decUint16(p)
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
	case 2:
		val := decUint16(p)
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
	case 2:
		*v = decUint32(p)
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
	case 2:
		val := decUint32(p)
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
	case 2:
		*v = decUint64(p)
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
	case 2:
		val := decUint64(p)
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
	case 2:
		*v = decUint(p)
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
	case 2:
		val := decUint(p)
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
	case 2:
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
	case 2:
		val := strconv.FormatInt(decInt64(p), 10)
		*v = &val
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
	case 2:
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
	case 2:
		*v = big.NewInt(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal smallint: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Int8:
		return decReflectInt8(p, v)
	case reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return decReflectInts(p, v)
	case reflect.Uint8:
		return decReflectUint8(p, v)
	case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return decReflectUints(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal smallint: unsupported value type (%T)(%[1]v), supported types: %s", v.Interface(), supportedTypes)
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal smallint: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int8:
		return decReflectInt8R(p, v)
	case reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return decReflectIntsR(p, v)
	case reflect.Uint8:
		return decReflectUint8R(p, v)
	case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return decReflectUintsR(p, v)
	case reflect.String:
		return decReflectStringR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal smallint: unsupported value type (%T)(%[1]v), supported types: %s", v.Interface(), supportedTypes)
	}
}

func decReflectInt8(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 2:
		val := decInt64(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into (%T), the data should be in the int8 range", v.Interface())
		}
		v.SetInt(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInts(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 2:
		v.SetInt(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint8(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 2:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into (%T), the data should be in the uint8 range", v.Interface())
		}
		v.SetUint(uint64(p[1]))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUints(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 2:
		v.SetUint(decUint64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectString(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p != nil {
			v.SetString("0")
		} else {
			v.SetString("")
		}
	case 2:
		v.SetString(strconv.FormatInt(decInt64(p), 10))
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

func decReflectInt8R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 2:
		val := decInt64(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into (%T), the data should be in the int8 range", v.Interface())
		}
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(val)
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectIntsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 2:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetInt(decInt64(p))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint8R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 2:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal smallint: to unmarshal into (%T), the data should be in the uint8 range", v.Interface())
		}
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(uint64(p[1]))
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUintsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 2:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetUint(decUint64(p))
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
	case 2:
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetString(strconv.FormatInt(decInt64(p), 10))
		v.Elem().Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decInt16(p []byte) int16 {
	return int16(p[0])<<8 | int16(p[1])
}

func decInt32(p []byte) int32 {
	if p[0] > math.MaxInt8 {
		return negInt32 | int32(p[0])<<8 | int32(p[1])
	}
	return int32(p[0])<<8 | int32(p[1])
}

func decInt64(p []byte) int64 {
	if p[0] > math.MaxInt8 {
		return negInt64 | int64(p[0])<<8 | int64(p[1])
	}
	return int64(p[0])<<8 | int64(p[1])
}

func decInt(p []byte) int {
	if p[0] > math.MaxInt8 {
		return negInt | int(p[0])<<8 | int(p[1])
	}
	return int(p[0])<<8 | int(p[1])
}

func decUint16(p []byte) uint16 {
	return uint16(p[0])<<8 | uint16(p[1])
}

func decUint32(p []byte) uint32 {
	return uint32(p[0])<<8 | uint32(p[1])
}

func decUint64(p []byte) uint64 {
	return uint64(p[0])<<8 | uint64(p[1])
}

func decUint(p []byte) uint {
	return uint(p[0])<<8 | uint(p[1])
}
