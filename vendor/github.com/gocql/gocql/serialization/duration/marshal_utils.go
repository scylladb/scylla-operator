package duration

import (
	"fmt"
	"reflect"
	"time"
)

const (
	vintPrefix1 byte = 128
	vintPrefix2 byte = 192
	vintPrefix3 byte = 224
	vintPrefix4 byte = 240
	vintPrefix5 byte = 248
	vintPrefix6 byte = 252
	vintPrefix7 byte = 254
	vintPrefix8 byte = 255

	nanoDayPos = 24 * 60 * 60 * 1000 * 1000 * 1000
	nanoDayNeg = -nanoDayPos
)

func EncInt64(v int64) ([]byte, error) {
	return encInt64(v), nil
}

func EncInt64R(v *int64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encInt64(*v), nil
}

func EncDur(v time.Duration) ([]byte, error) {
	return encDur(v), nil
}

func EncDurR(v *time.Duration) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encDur(*v), nil
}

func EncString(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	data, err := encString(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal duration: the parse error of the (string)(%s): %v", v, err)
	}
	return data, nil
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncString(*v)
}

func EncDuration(v Duration) ([]byte, error) {
	if !v.Valid() {
		return nil, fmt.Errorf("failed to marshal duration: the (Duration) values of months (%d), days (%d) and nanoseconds (%d) should have the same sign", v.Months, v.Days, v.Nanoseconds)
	}
	return encVintMonthsDaysNanos(v.Months, v.Days, v.Nanoseconds), nil
}

func EncDurationR(v *Duration) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	if !v.Valid() {
		return nil, fmt.Errorf("failed to marshal duration: the (*Duration) values of the months (%d), days (%d) and nanoseconds (%d) should have same sign", v.Months, v.Days, v.Nanoseconds)
	}
	return encVintMonthsDaysNanos(v.Months, v.Days, v.Nanoseconds), nil
}

func EncReflect(v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Int64:
		return encInt64(v.Int()), nil
	case reflect.String:
		val := v.String()
		if val == "" {
			return nil, nil
		}
		data, err := encString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal duration: the (%T)(%[1]v) have invalid format, %v", v, err)
		}
		return data, nil
	case reflect.Struct:
		if v.Type().String() == "gocql.unsetColumn" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to marshal duration: unsupported value type (%T)(%[1]v), supported types: ~int64, ~string, time.Duration, gocql.Duration, unsetColumn", v.Interface())
	default:
		return nil, fmt.Errorf("failed to marshal duration: unsupported value type (%T)(%[1]v), supported types: ~int64, ~string, time.Duration, gocql.Duration, unsetColumn", v.Interface())
	}
}

func EncReflectR(v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return nil, nil
	}
	return EncReflect(v.Elem())
}

func encDur(v time.Duration) []byte {
	if v < nanoDayPos && v > nanoDayNeg {
		return encNanos(encIntZigZagDur(v))
	}
	n := v % nanoDayPos
	return encDaysNanos(encIntZigZag32(int32((v-n)/nanoDayPos)), encIntZigZagDur(n))
}

func encInt64(v int64) []byte {
	if v < nanoDayPos && v > nanoDayNeg {
		return encNanos(encIntZigZag64(v))
	}
	n := v % nanoDayPos
	return encDaysNanos(encIntZigZag32(int32((v-n)/nanoDayPos)), encIntZigZag64(n))
}

func encZigZagUint64Pos(v uint64) uint64 {
	return v << 1
}

func encIntZigZag32(v int32) uint32 {
	return uint32((v >> 31) ^ (v << 1))
}

func encIntZigZag64(v int64) uint64 {
	return uint64((v >> 63) ^ (v << 1))
}

func encIntZigZagDur(v time.Duration) uint64 {
	return uint64((v >> 63) ^ (v << 1))
}

func encVint32(v uint32) []byte {
	switch {
	case byte(v>>28) != 0:
		return []byte{vintPrefix4, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>21) != 0:
		return []byte{vintPrefix3 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>14) != 0:
		return []byte{vintPrefix2 | byte(v>>16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{vintPrefix1 | byte(v>>8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}

func encVint64as32(v uint64) []byte {
	switch {
	case byte(v>>28) != 0:
		return []byte{vintPrefix4, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>21) != 0:
		return []byte{vintPrefix3 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>14) != 0:
		return []byte{vintPrefix2 | byte(v>>16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{vintPrefix1 | byte(v>>8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}

func encVint64(v uint64) []byte {
	switch {
	case byte(v>>56) != 0:
		return []byte{vintPrefix8, byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>49) != 0:
		return []byte{vintPrefix7 | byte(v>>56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>42) != 0:
		return []byte{vintPrefix6 | byte(v>>48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>35) != 0:
		return []byte{vintPrefix5 | byte(v>>40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>28) != 0:
		return []byte{vintPrefix4 | byte(v>>32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>21) != 0:
		return []byte{vintPrefix3 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>14) != 0:
		return []byte{vintPrefix2 | byte(v>>16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{vintPrefix1 | byte(v>>8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}

func encVintMonthsDaysNanos(m, d int32, n int64) []byte {
	if m == 0 {
		if d == 0 {
			return encNanos(encIntZigZag64(n))
		}
		return append(encDays(encIntZigZag32(d)), encVint64(encIntZigZag64(n))...)
	}
	return append(append(encVint32(encIntZigZag32(m)), encVint32(encIntZigZag32(d))...), encVint64(encIntZigZag64(n))...)
}

func encVintMonthsDaysNanosPos(m, d, n uint64) []byte {
	if m == 0 {
		if d == 0 {
			return encNanos(encZigZagUint64Pos(n))
		}
		return append(encDays64(encZigZagUint64Pos(d)), encVint64(encZigZagUint64Pos(n))...)
	}
	return append(append(encVint64as32(encZigZagUint64Pos(m)), encVint64as32(encZigZagUint64Pos(d))...), encVint64(encZigZagUint64Pos(n))...)
}

func encVintMonthsDaysNanosNeg(m, d, n uint64) []byte {
	if m == 0 {
		if d == 0 {
			return encNanos(encIntZigZag64(int64(-n)))
		}
		return append(encDays(encIntZigZag32(int32(-d))), encVint64(encIntZigZag64(int64(-n)))...)
	}
	return append(append(encVint32(encIntZigZag32(int32(-m))), encVint32(encIntZigZag32(int32(-d)))...), encVint64(encIntZigZag64(int64(-n)))...)
}

func encDaysNanos(d uint32, n uint64) []byte {
	return append(encDays(d), encVint64(n)...)
}

func encDays(v uint32) []byte {
	switch {
	case byte(v>>28) != 0:
		return []byte{0, vintPrefix4, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>21) != 0:
		return []byte{0, vintPrefix3 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>14) != 0:
		return []byte{0, vintPrefix2 | byte(v>>16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{0, vintPrefix1 | byte(v>>8), byte(v)}
	default:
		return []byte{0, byte(v)}
	}
}

func encDays64(v uint64) []byte {
	switch {
	case byte(v>>28) != 0:
		return []byte{0, vintPrefix4, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>21) != 0:
		return []byte{0, vintPrefix3 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>14) != 0:
		return []byte{0, vintPrefix2 | byte(v>>16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{0, vintPrefix1 | byte(v>>8), byte(v)}
	default:
		return []byte{0, byte(v)}
	}
}

func encNanos(v uint64) []byte {
	switch {
	case byte(v>>56) != 0:
		return []byte{0, 0, vintPrefix8, byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>49) != 0:
		return []byte{0, 0, vintPrefix7 | byte(v>>56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>42) != 0:
		return []byte{0, 0, vintPrefix6 | byte(v>>48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>35) != 0:
		return []byte{0, 0, vintPrefix5 | byte(v>>40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>28) != 0:
		return []byte{0, 0, vintPrefix4 | byte(v>>32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>21) != 0:
		return []byte{0, 0, vintPrefix3 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>14) != 0:
		return []byte{0, 0, vintPrefix2 | byte(v>>16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{0, 0, vintPrefix1 | byte(v>>8), byte(v)}
	default:
		return []byte{0, 0, byte(v)}
	}
}
