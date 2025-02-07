package inet

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"unsafe"
)

var (
	errWrongDataLen = fmt.Errorf("failed to unmarshal inet: the length of the data can be 0,4,16")

	digits = getDigits()
)

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal inet: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecBytes(p []byte, v *[]byte) error {
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
	case 4:
		*v = make([]byte, 4)
		copy(*v, p)
	case 16:
		*v = make([]byte, 16)
		copy(*v, p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecBytesR(p []byte, v **[]byte) error {
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
	case 4:
		*v = &[]byte{0, 0, 0, 0}
		copy(**v, p)
	case 16:
		*v = &[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		copy(**v, p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecNetIP(p []byte, v *net.IP) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = make(net.IP, 0)
		}
	case 4:
		*v = make(net.IP, 4)
		copy(*v, p)
	case 16:
		*v = make(net.IP, 16)
		copy(*v, p)
		if v4 := v.To4(); v4 != nil {
			*v = v4
		}
	default:
		return errWrongDataLen
	}
	return nil
}

func DecNetIPr(p []byte, v **net.IP) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			tmp := make(net.IP, 0)
			*v = &tmp
		}
	case 4:
		*v = &net.IP{0, 0, 0, 0}
		copy(**v, p)
	case 16:
		*v = &net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		copy(**v, p)
		if v4 := (*v).To4(); v4 != nil {
			**v = v4
		}
	default:
		return errWrongDataLen
	}
	return nil
}

func DecArray4(p []byte, v *[4]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = [4]byte{}
	case 4:
		*v = [4]byte{}
		copy(v[:], p)
	case 16:
		if !isFist10Zeros(p) {
			return fmt.Errorf("failed to unmarshal inet: can not unmarshal ipV6 into [4]byte")
		}
		*v = [4]byte{}
		copy(v[:], p[12:16])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecArray4R(p []byte, v **[4]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = &[4]byte{}
		}
	case 4:
		*v = &[4]byte{}
		copy((*v)[:], p)
	case 16:
		if !isFist10Zeros(p) {
			return fmt.Errorf("failed to unmarshal inet: can not unmarshal ipV6 into [4]byte")
		}
		*v = &[4]byte{}
		copy((*v)[:], p[12:16])
	default:
		return errWrongDataLen
	}
	return nil
}

func DecArray16(p []byte, v *[16]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = [16]byte{}
	case 4, 16:
		*v = [16]byte{}
		copy(v[:], p)
	default:
		return errWrongDataLen
	}
	return nil
}

func DecArray16R(p []byte, v **[16]byte) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = &[16]byte{}
		}
	case 4, 16:
		*v = &[16]byte{}
		copy((*v)[:], p)
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
			*v = "0.0.0.0"
		}
	case 4:
		*v = decString4(p)
	case 16:
		*v = decString16(p)
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
			tmp := "0.0.0.0"
			*v = &tmp
		}
	case 4:
		tmp := decString4(p)
		*v = &tmp
	case 16:
		tmp := decString16(p)
		*v = &tmp
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
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		switch v.Len() {
		case 4:
			return decReflectArray4(p, v)
		case 16:
			return decReflectArray16(p, v)
		default:
			return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return decReflectBytes(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return errNilReference(v)
	}

	ev := v.Elem()
	switch evt := ev.Type().Elem(); evt.Kind() {
	case reflect.Array:
		if evt.Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		switch ev.Len() {
		case 4:
			return decReflectArray4R(p, ev)
		case 16:
			return decReflectArray16R(p, ev)
		default:
			return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
	case reflect.Slice:
		if evt.Elem().Kind() != reflect.Uint8 {
			return fmt.Errorf("failed to marshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
		}
		return decReflectBytesR(p, ev)
	case reflect.String:
		return decReflectStringR(p, ev)
	default:
		return fmt.Errorf("failed to unmarshal inet: unsupported value type (%T)(%[1]v)", v.Interface())
	}
}

func decReflectArray4(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetZero()
	case 4:
		val := reflect.New(v.Type())
		copy((*[4]byte)(val.UnsafePointer())[:], p)
		v.Set(val.Elem())
	case 16:
		if !isFist10Zeros(p) {
			return fmt.Errorf("failed to unmarshal inet: can not unmarshal ipV6 into (%T)", v.Interface())
		}
		val := reflect.New(v.Type())
		copy((*[4]byte)(val.UnsafePointer())[:], p[12:16])
		v.Set(val.Elem())
	default:
		return fmt.Errorf("failed to unmarshal inet: to unmarshal into (%T) the length of the data can be 0,4,16", v.Interface())
	}
	return nil
}

func decReflectArray16(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		v.SetZero()
	case 4, 16:
		val := reflect.New(v.Type())
		copy((*[16]byte)(val.UnsafePointer())[:], p)
		v.Set(val.Elem())
	default:
		return fmt.Errorf("failed to unmarshal inet: to unmarshal into (%T) the length of the data can be 0,4,16", v.Interface())
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
	case 4:
		tmp := make([]byte, 4)
		copy(tmp, p)
		v.SetBytes(tmp)
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
			v.SetString("0.0.0.0")
		}
	case 4:
		v.SetString(decString4(p))
	case 16:
		v.SetString(decString16(p))
	default:
		return errWrongDataLen
	}
	return nil
}

func decReflectArray4R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Set(reflect.Zero(v.Type()))
		} else {
			val := reflect.New(v.Type().Elem())
			v.Set(val)
		}
	case 4:
		val := reflect.New(v.Type().Elem())
		copy((*[4]byte)(val.UnsafePointer())[:], p)
		v.Set(val)
	case 16:
		if !isFist10Zeros(p) {
			return fmt.Errorf("failed to unmarshal inet: can not unmarshal ipV6 into (%T)", v.Interface())
		}
		val := reflect.New(v.Type().Elem())
		copy((*[4]byte)(val.UnsafePointer())[:], p[12:16])
		v.Set(val)
	default:
		return fmt.Errorf("failed to unmarshal inet: to unmarshal into (%T) the length of the data can be 0,4,16", v.Interface())
	}
	return nil
}

func decReflectArray16R(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.Set(reflect.Zero(v.Type()))
		} else {
			val := reflect.New(v.Type().Elem())
			v.Set(val)
		}
	case 4, 16:
		val := reflect.New(v.Type().Elem())
		copy((*[16]byte)(val.UnsafePointer())[:], p)
		v.Set(val)
	default:
		return fmt.Errorf("failed to unmarshal inet: to unmarshal into (%T) the length of the data can be 0,4,16", v.Interface())
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
	case 4:
		tmp := make([]byte, 4)
		copy(tmp, p)
		val := reflect.New(v.Type().Elem())
		val.Elem().SetBytes(tmp)
		v.Set(val)
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
			val.Elem().SetString("0.0.0.0")
			v.Set(val)
		}
	case 4:
		val := reflect.New(v.Type().Elem())
		val.Elem().SetString(decString4(p))
		v.Set(val)
	case 16:
		val := reflect.New(v.Type().Elem())
		val.Elem().SetString(decString16(p))
		v.Set(val)
	default:
		return errWrongDataLen
	}
	return nil
}

func decString4(p []byte) string {
	out := make([]byte, 0, 15)
	for _, x := range p {
		out = append(out, digits[x]...)
	}
	return string(out[:len(out)-1])
}

func decString16(p []byte) string {
	if isV4MappedToV6(p) {
		return decString4(p[12:16])
	}
	return netip.AddrFrom16(*(*[16]byte)(unsafe.Pointer(&p[0]))).String()
}

func getDigits() []string {
	out := make([]string, 256)
	for i := range out {
		out[i] = fmt.Sprintf("%d.", i)
	}
	return out
}

func isV4MappedToV6(p []byte) bool {
	return p[0] == 0 && p[1] == 0 && p[2] == 0 && p[3] == 0 && p[4] == 0 &&
		p[5] == 0 && p[6] == 0 && p[7] == 0 && p[8] == 0 && p[9] == 0 && p[10] == 255 && p[11] == 255
}

func isFist10Zeros(p []byte) bool {
	return p[0] == 0 && p[1] == 0 && p[2] == 0 && p[3] == 0 && p[4] == 0 &&
		p[5] == 0 && p[6] == 0 && p[7] == 0 && p[8] == 0 && p[9] == 0
}
