package timeuuid

import (
	"fmt"
	"reflect"
	"time"
)

const (
	hexString = "0123456789abcdef"
	zeroUUID  = "00000000-0000-0000-0000-000000000000"
)

var (
	offsets  = [...]int{0, 2, 4, 6, 9, 11, 14, 16, 19, 21, 24, 26, 28, 30, 32, 34}
	timeBase = time.Date(1582, time.October, 15, 0, 0, 0, 0, time.UTC).Unix()

	errWrongDataLen = fmt.Errorf("failed to unmarshal timeuuid: the length of the data should be 0 or 16")
)

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal timeuuid: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecArray(p []byte, v *[16]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = [16]byte{}
	case 16:
		copy(v[:], p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecArrayR(p []byte, v **[16]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new([16]byte)
		}
	case 16:
		*v = &[16]byte{}
		copy((*v)[:], p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecSlice(p []byte, v *[]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = make([]byte, 0)
		}
	case 16:
		*v = make([]byte, 16)
		copy(*v, p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecSliceR(p []byte, v **[]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			tmp := make([]byte, 0)
			*v = &tmp
		}
	case 16:
		*v = &[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		copy(**v, p)
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
			*v = zeroUUID
		}
	case 16:
		*v = decString(p)
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
			tmp := zeroUUID
			*v = &tmp
		}
	case 16:
		tmp := decString(p)
		*v = &tmp
	default:
		return errWrongDataLen
	}
	return nil
}

func DecTime(p []byte, v *time.Time) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = time.Time{}
	case 16:
		*v = decTime(p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecTimeR(p []byte, v **time.Time) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(time.Time)
		}
	case 16:
		val := decTime(p)
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
	case reflect.Array:
		if v.Type().Elem().Kind() != reflect.Uint8 || v.Len() != 16 {
			return fmt.Errorf("failed to unmarshal timeuuid: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return decReflectArray(p, v)
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to unmarshal timeuuid: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return decReflectBytes(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal timeuuid: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	ev := v.Elem()
	switch evt := ev.Type().Elem(); evt.Kind() {
	case reflect.Array:
		if evt.Elem().Kind() != reflect.Uint8 || ev.Len() != 16 {
			return fmt.Errorf("failed to marshal timeuuid: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return decReflectArrayR(p, ev)
	case reflect.Slice:
		if evt.Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to marshal timeuuid: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return decReflectBytesR(p, ev)
	case reflect.String:
		return decReflectStringR(p, ev)
	default:
		return fmt.Errorf("failed to unmarshal timeuuid: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectArray(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetZero()
	case 16:
		val := reflect.New(v.Type())
		copy((*[16]byte)(val.UnsafePointer())[:], p)
		v.Set(val.Elem())
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectBytes(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.SetBytes(nil)
		} else {
			v.SetBytes(make([]byte, 0))
		}
	case 16:
		tmp := make([]byte, 16)
		copy(tmp, p)
		v.SetBytes(tmp)
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
			v.SetString(zeroUUID)
		}
	case 16:
		v.SetString(decString(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectArrayR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Set(reflect.Zero(v.Type()))
		} else {
			val := reflect.New(v.Type().Elem())
			v.Set(val)
		}
	case 16:
		val := reflect.New(v.Type().Elem())
		copy((*[16]byte)(val.UnsafePointer())[:], p)
		v.Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectBytesR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Set(reflect.Zero(v.Type()))
		} else {
			val := reflect.New(v.Type().Elem())
			val.Elem().SetBytes(make([]byte, 0))
			v.Set(val)
		}
	case 16:
		tmp := make([]byte, 16)
		copy(tmp, p)
		val := reflect.New(v.Type().Elem())
		val.Elem().SetBytes(tmp)
		v.Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectStringR(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Set(reflect.Zero(v.Type()))
		} else {
			val := reflect.New(v.Type().Elem())
			val.Elem().SetString(zeroUUID)
			v.Set(val)
		}
	case 16:
		val := reflect.New(v.Type().Elem())
		val.Elem().SetString(decString(p))
		v.Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decString(p []byte) string {
	r := make([]byte, 36)
	for i, b := range p {
		r[offsets[i]] = hexString[b>>4]
		r[offsets[i]+1] = hexString[b&0xF]
	}
	r[8] = '-'
	r[13] = '-'
	r[18] = '-'
	r[23] = '-'
	return string(r)
}

func decTime(u []byte) time.Time {
	ts := decTimestamp(u)
	sec := ts / 1e7
	nsec := (ts % 1e7) * 100
	return time.Unix(sec+timeBase, nsec).UTC()
}

func decTimestamp(u []byte) int64 {
	return int64(uint64(u[0])<<24|uint64(u[1])<<16|
		uint64(u[2])<<8|uint64(u[3])) +
		int64(uint64(u[4])<<40|uint64(u[5])<<32) +
		int64(uint64(u[6]&0x0F)<<56|uint64(u[7])<<48)
}
