package bigint

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal bigint: the length of the data should be 0 or 8")

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal bigint: can not unmarshal into nil reference (%T)(%[1]v))", v)
}

func DecInt8(p []byte, v *int8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 8:
		val := decInt64(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into int8, the data should be in the int8 range")
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
	case 8:
		val := decInt64(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into int8, the data should be in the int8 range")
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
	case 8:
		val := decInt64(p)
		if val > math.MaxInt16 || val < math.MinInt16 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into int16, the data should be in the int16 range")
		}
		*v = int16(val)
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
	case 8:
		val := decInt64(p)
		if val > math.MaxInt16 || val < math.MinInt16 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into int16, the data should be in the int16 range")
		}
		tmp := int16(val)
		*v = &tmp
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
	case 8:
		val := decInt64(p)
		if val > math.MaxInt32 || val < math.MinInt32 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into int32, the data should be in the int32 range")
		}
		*v = int32(val)
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
	case 8:
		val := decInt64(p)
		if val > math.MaxInt32 || val < math.MinInt32 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into int32, the data should be in the int32 range")
		}
		tmp := int32(val)
		*v = &tmp
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
	case 8:
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
	case 8:
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
	case 8:
		*v = int(p[0])<<56 | int(p[1])<<48 | int(p[2])<<40 | int(p[3])<<32 | int(p[4])<<24 | int(p[5])<<16 | int(p[6])<<8 | int(p[7])
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
	case 8:
		val := int(p[0])<<56 | int(p[1])<<48 | int(p[2])<<40 | int(p[3])<<32 | int(p[4])<<24 | int(p[5])<<16 | int(p[6])<<8 | int(p[7])
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 || p[6] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into uint8, the data should be in the uint8 range")
		}
		*v = p[7]
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 || p[6] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into uint8, the data should be in the uint8 range")
		}
		val := p[7]
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into uint16, the data should be in the uint16 range")
		}
		*v = uint16(p[6])<<8 | uint16(p[7])
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into uint16, the data should be in the uint16 range")
		}
		val := uint16(p[6])<<8 | uint16(p[7])
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into uint32, the data should be in the uint32 range")
		}
		*v = uint32(p[4])<<24 | uint32(p[5])<<16 | uint32(p[6])<<8 | uint32(p[7])
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into uint32, the data should be in the uint32 range")
		}
		val := uint32(p[4])<<24 | uint32(p[5])<<16 | uint32(p[6])<<8 | uint32(p[7])
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
	case 8:
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
	case 8:
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
	case 8:
		*v = uint(p[0])<<56 | uint(p[1])<<48 | uint(p[2])<<40 | uint(p[3])<<32 | uint(p[4])<<24 | uint(p[5])<<16 | uint(p[6])<<8 | uint(p[7])
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
	case 8:
		val := uint(p[0])<<56 | uint(p[1])<<48 | uint(p[2])<<40 | uint(p[3])<<32 | uint(p[4])<<24 | uint(p[5])<<16 | uint(p[6])<<8 | uint(p[7])
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
	case 8:
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
	case 8:
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
	case 8:
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
			*v = new(big.Int)
		}
	case 8:
		*v = big.NewInt(decInt64(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal bigint: can not unmarshal into nil reference (%T)(%[1]v))", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Int8:
		return decReflectInt8(p, v)
	case reflect.Int16:
		return decReflectInt16(p, v)
	case reflect.Int32:
		return decReflectInt32(p, v)
	case reflect.Int64, reflect.Int:
		return decReflectInts(p, v)
	case reflect.Uint8:
		return decReflectUint8(p, v)
	case reflect.Uint16:
		return decReflectUint16(p, v)
	case reflect.Uint32:
		return decReflectUint32(p, v)
	case reflect.Uint64, reflect.Uint:
		return decReflectUints(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal bigint: unsupported value type (%T)(%[1]v), supported types: %s", v.Interface(), supportedTypes)
	}
}

func decReflectInt8(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 8:
		val := decInt64(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the int8 range", v.Interface())
		}
		v.SetInt(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInt16(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 8:
		val := decInt64(p)
		if val > math.MaxInt16 || val < math.MinInt16 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the int16 range", v.Interface())
		}
		v.SetInt(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInt32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 8:
		val := decInt64(p)
		if val > math.MaxInt32 || val < math.MinInt32 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the int32 range", v.Interface())
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
	case 8:
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
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 || p[6] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the uint8 range", v.Interface())
		}
		v.SetUint(uint64(p[7]))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint16(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the uint16 range", v.Interface())
		}
		v.SetUint(uint64(p[6])<<8 | uint64(p[7]))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 8:
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the uint32 range", v.Interface())
		}
		v.SetUint(uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7]))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUints(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
	case 8:
		v.SetUint(decUint64(p))
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
	case 8:
		v.SetString(strconv.FormatInt(decInt64(p), 10))
	default:
		return errWrongDataLen
	}
	return nil
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal bigint: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int8:
		return decReflectInt8R(p, v)
	case reflect.Int16:
		return decReflectInt16R(p, v)
	case reflect.Int32:
		return decReflectInt32R(p, v)
	case reflect.Int64, reflect.Int:
		return decReflectIntsR(p, v)
	case reflect.Uint8:
		return decReflectUint8R(p, v)
	case reflect.Uint16:
		return decReflectUint16R(p, v)
	case reflect.Uint32:
		return decReflectUint32R(p, v)
	case reflect.Uint64, reflect.Uint:
		return decReflectUintsR(p, v)
	case reflect.String:
		return decReflectStringR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal bigint: unsupported value type (%T)(%[1]v), supported types: %s", v.Interface(), supportedTypes)
	}
}

func decReflectInt8R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 8:
		val := decInt64(p)
		if val > math.MaxInt8 || val < math.MinInt8 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the int8 range", v.Interface())
		}
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(val)
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInt16R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 8:
		val := decInt64(p)
		if val > math.MaxInt16 || val < math.MinInt16 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the int16 range", v.Interface())
		}
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(val)
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectInt32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 8:
		val := decInt64(p)
		if val > math.MaxInt32 || val < math.MinInt32 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the int32 range", v.Interface())
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
	case 8:
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
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 || p[6] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the uint8 range", v.Interface())
		}
		newVal.Elem().SetUint(uint64(p[7]))
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint16R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 || p[4] != 0 || p[5] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the uint16 range", v.Interface())
		}
		newVal.Elem().SetUint(uint64(p[6])<<8 | uint64(p[7]))
		v.Elem().Set(newVal)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectUint32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		if p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0 {
			return fmt.Errorf("failed to unmarshal bigint: to unmarshal into %T, the data should be in the uint32 range", v.Interface())
		}
		newVal.Elem().SetUint(uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7]))
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
	case 8:
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
	case 8:
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

func decInt64(p []byte) int64 {
	return int64(p[0])<<56 | int64(p[1])<<48 | int64(p[2])<<40 | int64(p[3])<<32 | int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
}

func decUint64(p []byte) uint64 {
	return uint64(p[0])<<56 | uint64(p[1])<<48 | uint64(p[2])<<40 | uint64(p[3])<<32 | uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7])
}
