package decimal

import (
	"fmt"
	"reflect"
	"strconv"

	"gopkg.in/inf.v0"

	"github.com/gocql/gocql/serialization/varint"
)

var errWrongDataLen = fmt.Errorf("failed to unmarshal decimal: the length of the data should be 0 or more than 5")

func errBrokenData(p []byte) error {
	if p[4] == 0 && p[5] <= 127 || p[4] == 255 && p[5] > 127 {
		return fmt.Errorf("failed to unmarshal decimal: the data is broken")
	}
	return nil
}

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal decimal: can not unmarshal into nil reference(%T)(%[1]v)", v)
}

func DecInfDec(p []byte, v *inf.Dec) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		v.SetScale(0).SetUnscaled(0)
		return nil
	case 1, 2, 3, 4:
		return errWrongDataLen
	case 5:
		v.SetScale(decScale(p)).SetUnscaled(dec1toInt64(p))
		return nil
	case 6:
		v.SetScale(decScale(p)).SetUnscaled(dec2toInt64(p))
	case 7:
		v.SetScale(decScale(p)).SetUnscaled(dec3toInt64(p))
	case 8:
		v.SetScale(decScale(p)).SetUnscaled(dec4toInt64(p))
	case 9:
		v.SetScale(decScale(p)).SetUnscaled(dec5toInt64(p))
	case 10:
		v.SetScale(decScale(p)).SetUnscaled(dec6toInt64(p))
	case 11:
		v.SetScale(decScale(p)).SetUnscaled(dec7toInt64(p))
	case 12:
		v.SetScale(decScale(p)).SetUnscaled(dec8toInt64(p))
	default:
		v.SetScale(decScale(p)).SetUnscaledBig(varint.Dec2BigInt(p[4:]))
	}
	return errBrokenData(p)
}

func DecInfDecR(p []byte, v **inf.Dec) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		if p == nil {
			*v = nil
		} else {
			*v = inf.NewDec(0, 0)
		}
		return nil
	case 1, 2, 3, 4:
		return errWrongDataLen
	case 5:
		*v = inf.NewDec(dec1toInt64(p), decScale(p))
		return nil
	case 6:
		*v = inf.NewDec(dec2toInt64(p), decScale(p))
	case 7:
		*v = inf.NewDec(dec3toInt64(p), decScale(p))
	case 8:
		*v = inf.NewDec(dec4toInt64(p), decScale(p))
	case 9:
		*v = inf.NewDec(dec5toInt64(p), decScale(p))
	case 10:
		*v = inf.NewDec(dec6toInt64(p), decScale(p))
	case 11:
		*v = inf.NewDec(dec7toInt64(p), decScale(p))
	case 12:
		*v = inf.NewDec(dec8toInt64(p), decScale(p))
	default:
		*v = inf.NewDecBig(varint.Dec2BigInt(p[4:]), decScale(p))
	}
	return errBrokenData(p)
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
			*v = "0;0"
		}
		return nil
	case 1, 2, 3, 4:
		return errWrongDataLen
	case 5:
		*v = decString5(p)
		return nil
	case 6:
		*v = decString6(p)
	case 7:
		*v = decString7(p)
	case 8:
		*v = decString8(p)
	case 9:
		*v = decString9(p)
	case 10:
		*v = decString10(p)
	case 11:
		*v = decString11(p)
	case 12:
		*v = decString12(p)
	default:
		*v = decString(p)
	}
	return errBrokenData(p)
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
			tmp := "0;0"
			*v = &tmp
		}
		return nil
	case 1, 2, 3, 4:
		return errWrongDataLen
	case 5:
		tmp := decString5(p)
		*v = &tmp
		return nil
	case 6:
		tmp := decString6(p)
		*v = &tmp
	case 7:
		tmp := decString7(p)
		*v = &tmp
	case 8:
		tmp := decString8(p)
		*v = &tmp
	case 9:
		tmp := decString9(p)
		*v = &tmp
	case 10:
		tmp := decString10(p)
		*v = &tmp
	case 11:
		tmp := decString11(p)
		*v = &tmp
	case 12:
		tmp := decString12(p)
		*v = &tmp
	default:
		tmp := decString(p)
		*v = &tmp
	}
	return errBrokenData(p)
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal decimal: can not unmarshal into nil reference (%T)(%#[1]v)", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal decimal: unsupported value type (%T)(%#[1]v), supported types: ~string, inf.Dec", v.Interface())
	}
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal decimal: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.String:
		return decReflectStringR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal decimal: unsupported value type (%T)(%[1]v), supported types: ~string, inf.Dec", v.Interface())
	}
}

func decReflectString(p []byte, v reflect.Value) error {
	switch len(p) {
	case 0:
		if p == nil {
			v.SetString("")
		} else {
			v.SetString("0;0")
		}
		return nil
	case 1, 2, 3, 4:
		return errWrongDataLen
	case 5:
		v.SetString(decString5(p))
		return nil
	case 6:
		v.SetString(decString6(p))
	case 7:
		v.SetString(decString7(p))
	case 8:
		v.SetString(decString8(p))
	case 9:
		v.SetString(decString9(p))
	case 10:
		v.SetString(decString10(p))
	case 11:
		v.SetString(decString11(p))
	case 12:
		v.SetString(decString12(p))
	default:
		v.SetString(decString(p))
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
			val.Elem().SetString("0;0")
		}
		v.Elem().Set(val)
		return nil
	case 1, 2, 3, 4:
		return errWrongDataLen
	case 5:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString5(p))
		v.Elem().Set(newVal)
		return nil
	case 6:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString6(p))
		v.Elem().Set(newVal)
	case 7:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString7(p))
		v.Elem().Set(newVal)
	case 8:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString8(p))
		v.Elem().Set(newVal)
	case 9:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString9(p))
		v.Elem().Set(newVal)
	case 10:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString10(p))
		v.Elem().Set(newVal)
	case 11:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString11(p))
		v.Elem().Set(newVal)
	case 12:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString12(p))
		v.Elem().Set(newVal)
	default:
		newVal := reflect.New(v.Type().Elem().Elem())
		newVal.Elem().SetString(decString(p))
		v.Elem().Set(newVal)
	}
	return errBrokenData(p)
}

func decString5(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec1toInt64(p), 10)
}

func decString6(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec2toInt64(p), 10)
}

func decString7(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec3toInt64(p), 10)
}
func decString8(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec4toInt64(p), 10)
}
func decString9(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec5toInt64(p), 10)
}
func decString10(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec6toInt64(p), 10)
}
func decString11(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec7toInt64(p), 10)
}
func decString12(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + strconv.FormatInt(dec8toInt64(p), 10)
}

func decString(p []byte) string {
	return strconv.FormatInt(decScaleInt64(p), 10) + ";" + varint.Dec2BigInt(p[4:]).String()
}
