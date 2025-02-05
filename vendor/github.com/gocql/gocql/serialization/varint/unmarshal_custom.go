package varint

import (
	"fmt"
	"reflect"
	"strconv"
)

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal varint: can not unmarshal into nil reference (%T)(%#[1]v)", v.Interface())
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
		return fmt.Errorf("failed to unmarshal varint: unsupported value type (%T)(%#[1]v)", v.Interface())
	}
}

func decReflectInt8(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
	case 1:
		v.SetInt(dec1toInt64(p))
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int8 range", v.Interface())
	}
	return nil
}

func decReflectInt16(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
		return nil
	case 1:
		v.SetInt(dec1toInt64(p))
		return nil
	case 2:
		v.SetInt(dec2toInt64(p))
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int16 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectInt32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
		return nil
	case 1:
		v.SetInt(dec1toInt64(p))
		return nil
	case 2:
		v.SetInt(dec2toInt64(p))
	case 3:
		v.SetInt(dec3toInt64(p))
	case 4:
		v.SetInt(dec4toInt64(p))
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint32 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectInts(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetInt(0)
		return nil
	case 1:
		v.SetInt(dec1toInt64(p))
		return nil
	case 2:
		v.SetInt(dec2toInt64(p))
	case 3:
		v.SetInt(dec3toInt64(p))
	case 4:
		v.SetInt(dec4toInt64(p))
	case 5:
		v.SetInt(dec5toInt64(p))
	case 6:
		v.SetInt(dec6toInt64(p))
	case 7:
		v.SetInt(dec7toInt64(p))
	case 8:
		v.SetInt(dec8toInt64(p))
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int64 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUint8(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
		return nil
	case 1:
		v.SetUint(dec1toUint64(p))
		return nil
	case 2:
		if p[0] == 0 {
			v.SetUint(dec2toUint64(p))
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint8 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint8 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUint16(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
		return nil
	case 1:
		v.SetUint(dec1toUint64(p))
		return nil
	case 2:
		v.SetUint(dec2toUint64(p))
	case 3:
		if p[0] == 0 {
			v.SetUint(dec3toUint64(p))
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint16 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint16 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUint32(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
		return nil
	case 1:
		v.SetUint(dec1toUint64(p))
		return nil
	case 2:
		v.SetUint(dec2toUint64(p))
	case 3:
		v.SetUint(dec3toUint64(p))
	case 4:
		v.SetUint(dec4toUint64(p))
	case 5:
		if p[0] == 0 {
			v.SetUint(dec5toUint64(p))
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint32 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint32 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUints(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetUint(0)
		return nil
	case 1:
		v.SetUint(dec1toUint64(p))
		return nil
	case 2:
		v.SetUint(dec2toUint64(p))
	case 3:
		v.SetUint(dec3toUint64(p))
	case 4:
		v.SetUint(dec4toUint64(p))
	case 5:
		v.SetUint(dec5toUint64(p))
	case 6:
		v.SetUint(dec6toUint64(p))
	case 7:
		v.SetUint(dec7toUint64(p))
	case 8:
		v.SetUint(dec8toUint64(p))
	case 9:
		if p[0] == 0 {
			v.SetUint(dec9toUint64(p))
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint64 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint64 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectString(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.SetString("")
		} else {
			v.SetString("0")
		}
		return nil
	case 1:
		v.SetString(strconv.FormatInt(dec1toInt64(p), 10))
		return nil
	case 2:
		v.SetString(strconv.FormatInt(dec2toInt64(p), 10))
	case 3:
		v.SetString(strconv.FormatInt(dec3toInt64(p), 10))
	case 4:
		v.SetString(strconv.FormatInt(dec4toInt64(p), 10))
	case 5:
		v.SetString(strconv.FormatInt(dec5toInt64(p), 10))
	case 6:
		v.SetString(strconv.FormatInt(dec6toInt64(p), 10))
	case 7:
		v.SetString(strconv.FormatInt(dec7toInt64(p), 10))
	case 8:
		v.SetString(strconv.FormatInt(dec8toInt64(p), 10))
	default:
		v.SetString(Dec2BigInt(p).String())
	}
	return errBrokenData(p)
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
		return fmt.Errorf("failed to unmarshal bigint: unsupported value type (%T)(%[1]v)", v.Interface())
	}
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
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec1toInt64(p))
		v.Elem().Set(newVal)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int8 range", v.Interface())
	}
	return nil
}

func decReflectInt16R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec1toInt64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec2toInt64(p))
		v.Elem().Set(newVal)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int16 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectInt32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec1toInt64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec2toInt64(p))
		v.Elem().Set(newVal)
	case 3:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec3toInt64(p))
		v.Elem().Set(newVal)
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec4toInt64(p))
		v.Elem().Set(newVal)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int32 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectIntsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec1toInt64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec2toInt64(p))
		v.Elem().Set(newVal)
	case 3:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec3toInt64(p))
		v.Elem().Set(newVal)
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec4toInt64(p))
		v.Elem().Set(newVal)
	case 5:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec5toInt64(p))
		v.Elem().Set(newVal)
	case 6:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec6toInt64(p))
		v.Elem().Set(newVal)
	case 7:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec7toInt64(p))
		v.Elem().Set(newVal)
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetInt(dec8toInt64(p))
		v.Elem().Set(newVal)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the int64 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUint8R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec1toUint64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		if p[0] == 0 {
			newVal := reflect.New(v.Type().Elem().Elem())
			newVal.Elem().SetUint(dec2toUint64(p))
			v.Elem().Set(newVal)
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint8 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint8 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUint16R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec1toUint64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec2toUint64(p))
		v.Elem().Set(newVal)
	case 3:
		if p[0] == 0 {
			newVal := reflect.New(v.Type().Elem().Elem())
			newVal.Elem().SetUint(dec3toUint64(p))
			v.Elem().Set(newVal)
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint16 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint16 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUint32R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec1toUint64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec2toUint64(p))
		v.Elem().Set(newVal)
	case 3:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec3toUint64(p))
		v.Elem().Set(newVal)
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec4toUint64(p))
		v.Elem().Set(newVal)
	case 5:
		if p[0] == 0 {
			newVal := reflect.New(v.Type().Elem().Elem())
			newVal.Elem().SetUint(dec5toUint64(p))
			v.Elem().Set(newVal)
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint32 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint32 range", v.Interface())
	}
	return errBrokenData(p)
}

func decReflectUintsR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.Elem().Set(decReflectNullableR(p, v))
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec1toUint64(p))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec2toUint64(p))
		v.Elem().Set(newVal)
	case 3:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec3toUint64(p))
		v.Elem().Set(newVal)
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec4toUint64(p))
		v.Elem().Set(newVal)
	case 5:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec5toUint64(p))
		v.Elem().Set(newVal)
	case 6:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec6toUint64(p))
		v.Elem().Set(newVal)
	case 7:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec7toUint64(p))
		v.Elem().Set(newVal)
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetUint(dec8toUint64(p))
		v.Elem().Set(newVal)
	case 9:
		if p[0] == 0 {
			newVal := reflect.New(v.Type().Elem().Elem())
			newVal.Elem().SetUint(dec9toUint64(p))
			v.Elem().Set(newVal)
		} else {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint64 range", v.Interface())
		}
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into %T, the data value should be in the uint64 range", v.Interface())
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec1toInt64(p), 10))
		v.Elem().Set(newVal)
		return nil
	case 2:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec2toInt64(p), 10))
		v.Elem().Set(newVal)
	case 3:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec3toInt64(p), 10))
		v.Elem().Set(newVal)
	case 4:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec4toInt64(p), 10))
		v.Elem().Set(newVal)
	case 5:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec5toInt64(p), 10))
		v.Elem().Set(newVal)
	case 6:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec6toInt64(p), 10))
		v.Elem().Set(newVal)
	case 7:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec7toInt64(p), 10))
		v.Elem().Set(newVal)
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(strconv.FormatInt(dec8toInt64(p), 10))
		v.Elem().Set(newVal)
	default:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(Dec2BigInt(p).String())
		v.Elem().Set(newVal)
	}
	return errBrokenData(p)
}
