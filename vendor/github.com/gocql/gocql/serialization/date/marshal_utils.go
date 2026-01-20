package date

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	millisecondsInADay int64 = 24 * 60 * 60 * 1000
	centerEpoch        int64 = 1 << 31
	maxYear            int   = 5881580
	minYear            int   = -5877641
	maxMilliseconds    int64 = 185542587100800000
	minMilliseconds    int64 = -185542587187200000
)

var (
	maxDate = time.Date(5881580, 07, 11, 0, 0, 0, 0, time.UTC)
	minDate = time.Date(-5877641, 06, 23, 0, 0, 0, 0, time.UTC)
)

func errWrongStringFormat(v interface{}) error {
	return fmt.Errorf(`failed to marshal date: the (%T)(%[1]v) should have fromat "2006-01-02"`, v)
}

func EncInt32(v int32) ([]byte, error) {
	return encInt32(v), nil
}

func EncInt32R(v *int32) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encInt32(*v), nil
}

func EncInt64(v int64) ([]byte, error) {
	if v > maxMilliseconds || v < minMilliseconds {
		return nil, fmt.Errorf("failed to marshal date: the (int64)(%v) value out of range", v)
	}
	return encInt64(days(v)), nil
}

func EncInt64R(v *int64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt64(*v)
}

func EncUint32(v uint32) ([]byte, error) {
	return encUint32(v), nil
}

func EncUint32R(v *uint32) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encUint32(*v), nil
}

func EncTime(v time.Time) ([]byte, error) {
	if v.After(maxDate) || v.Before(minDate) {
		return nil, fmt.Errorf("failed to marshal date: the (%T)(%s) value should be in the range from -5877641-06-23 to 5881580-07-11", v, v.Format("2006-01-02"))
	}
	return encTime(v), nil
}

func EncTimeR(v *time.Time) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncTime(*v)
}

func EncString(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	var err error
	var y, m, d int
	var t time.Time
	switch ps := strings.Split(v, "-"); len(ps) {
	case 3:
		if y, err = strconv.Atoi(ps[0]); err != nil {
			return nil, errWrongStringFormat(v)
		}
		if m, err = strconv.Atoi(ps[1]); err != nil {
			return nil, errWrongStringFormat(v)
		}
		if d, err = strconv.Atoi(ps[2]); err != nil {
			return nil, errWrongStringFormat(v)
		}
	case 4:
		if y, err = strconv.Atoi(ps[1]); err != nil || ps[0] != "" {
			return nil, errWrongStringFormat(v)
		}
		y = -y
		if m, err = strconv.Atoi(ps[2]); err != nil {
			return nil, errWrongStringFormat(v)
		}
		if d, err = strconv.Atoi(ps[3]); err != nil {
			return nil, errWrongStringFormat(v)
		}
	default:
		return nil, errWrongStringFormat(v)
	}
	if y > maxYear || y < minYear {
		return nil, fmt.Errorf("failed to marshal date: the (%T)(%[1]v) value should be in the range from -5877641-06-23 to 5881580-07-11", v)
	}
	t = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.After(maxDate) || t.Before(minDate) {
		return nil, fmt.Errorf("failed to marshal date: the (%T)(%[1]v) value should be in the range from -5877641-06-23 to 5881580-07-11", v)
	}
	return encTime(t), nil
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncString(*v)
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Int32:
		return encInt64(v.Int()), nil
	case reflect.Int64:
		val := v.Int()
		if val > maxMilliseconds || val < minMilliseconds {
			return nil, fmt.Errorf("failed to marshal date: the value (%T)(%[1]v) out of range", v.Interface())
		}
		return encInt64(days(val)), nil
	case reflect.Uint32:
		val := v.Uint()
		return []byte{byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)}, nil
	case reflect.String:
		return encReflectString(v)
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal date: unsupported value type (%T)(%[1]v), supported types: ~int32, ~int64, ~uint32,  ~string, time.Time, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal date: unsupported value type (%T)(%[1]v), supported types: ~int32, ~int64, ~uint32,  ~string, time.Time, unsetColumn", v.Interface())
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
	var err error
	var y, m, d int
	var t time.Time
	ps := strings.Split(val, "-")
	switch len(ps) {
	case 3:
		if y, err = strconv.Atoi(ps[0]); err != nil {
			return nil, errWrongStringFormat(v.Interface())
		}
		if m, err = strconv.Atoi(ps[1]); err != nil {
			return nil, errWrongStringFormat(v.Interface())
		}
		if d, err = strconv.Atoi(ps[2]); err != nil {
			return nil, errWrongStringFormat(v.Interface())
		}
	case 4:
		if y, err = strconv.Atoi(ps[1]); err != nil {
			return nil, errWrongStringFormat(v.Interface())
		}
		y = -y
		if m, err = strconv.Atoi(ps[2]); err != nil {
			return nil, errWrongStringFormat(v.Interface())
		}
		if d, err = strconv.Atoi(ps[3]); err != nil {
			return nil, errWrongStringFormat(v.Interface())
		}
	default:
		return nil, errWrongStringFormat(v.Interface())
	}
	if y > maxYear || y < minYear {
		return nil, fmt.Errorf("failed to marshal date: the (%T)(%[1]v) value should be in the range from -5877641-06-23 to 5881580-07-11", v.Interface())
	}
	t = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.After(maxDate) || t.Before(minDate) {
		return nil, fmt.Errorf("failed to marshal date: the (%T)(%[1]v) value should be in the range from -5877641-06-23 to 5881580-07-11", v.Interface())
	}
	return encTime(t), nil
}

func encInt64(v int64) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func encInt32(v int32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func encUint32(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func encTime(v time.Time) []byte {
	d := days(v.UnixMilli())
	return []byte{byte(d >> 24), byte(d >> 16), byte(d >> 8), byte(d)}
}

func days(v int64) int64 {
	return v/millisecondsInADay + centerEpoch
}
