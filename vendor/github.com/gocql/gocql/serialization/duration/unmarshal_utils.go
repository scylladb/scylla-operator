package duration

import (
	"fmt"
	"math"
	"reflect"
	"time"
)

const (
	maxDays      = (math.MaxInt64 - math.MaxInt64%nanoDayPos) / nanoDayPos
	minDays      = -maxDays
	maxDaysNanos = maxDays * nanoDayPos
	minDaysNanos = minDays * nanoDayPos
	zeroDuration = "0s"
)

var (
	errWrongDataLen = fmt.Errorf("failed to unmarshal duration: the length of the data should be 0 or 3-19")
	errBrokenData   = fmt.Errorf("failed to unmarshal duration: the data is broken")
	errInvalidSign  = fmt.Errorf("failed to unmarshal duration: the data values of months, days and nanoseconds should have the same sign")
)

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal duration: can not unmarshal into nil reference (%T)(%[1]v))", v)
}

func DecInt64(p []byte, v *int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		*v = 0
	case l < 3:
		return errWrongDataLen
	default:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal duration: to unmarshal into (int64) the months value should be 0")
		}
		if p[1] == 0 {
			var ok bool
			if *v, ok = decNanos64(p); !ok {
				return errBrokenData
			}
		} else {
			d, n, ok := decDaysNanos64(p)
			if !ok {
				return errBrokenData
			}
			if !validSignDateNanos(d, n) {
				return errInvalidSign
			}
			if *v, ok = daysToNanos(d, n); !ok {
				return fmt.Errorf("failed to unmarshal duration: to unmarshal into (int64) the data value should be in int64 range")
			}
		}
	}
	return nil
}

func DecInt64R(p []byte, v **int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(int64)
		}
	case l < 3:
		return errWrongDataLen
	default:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal duration: to unmarshal into (*int64) the months value should be 0")
		}
		if p[1] == 0 {
			n, ok := decNanos64(p)
			if !ok {
				return errBrokenData
			}
			*v = &n
		} else {
			d, n, ok := decDaysNanos64(p)
			if !ok {
				return errBrokenData
			}
			if !validSignDateNanos(d, n) {
				return errInvalidSign
			}
			if n, ok = daysToNanos(d, n); !ok {
				return fmt.Errorf("failed to unmarshal duration: to unmarshal into (*int64) the data value should be in int64 range")
			}
			*v = &n
		}
	}
	return nil
}

func DecString(p []byte, v *string) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		if p == nil {
			*v = ""
		} else {
			*v = zeroDuration
		}
	case l < 3:
		return errWrongDataLen
	default:
		m, d, n, ok := decDuration(p)
		if !ok {
			return errBrokenData
		}
		if !validDuration(m, d, n) {
			return errInvalidSign
		}
		*v = decString(m, d, n)
	}
	return nil
}

func DecStringR(p []byte, v **string) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		if p == nil {
			*v = nil
		} else {
			val := zeroDuration
			*v = &val
		}
	case l < 3:
		return errWrongDataLen
	default:
		var val string
		m, d, n, ok := decDuration(p)
		if !ok {
			return errBrokenData
		}
		if !validDuration(m, d, n) {
			return errInvalidSign
		}
		val = decString(m, d, n)
		*v = &val
	}
	return nil
}

func DecDur(p []byte, v *time.Duration) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		*v = 0
	case l < 3:
		return errWrongDataLen
	default:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal duration: to unmarshal into (time.Duration) the months value should be 0")
		}
		if p[1] == 0 {
			var ok bool
			if *v, ok = decNanosDur(p); !ok {
				return errBrokenData
			}
		} else {
			d, n, ok := decDaysNanosDur(p)
			if !ok {
				return errBrokenData
			}
			if !validDateNanosDur(d, n) {
				return errInvalidSign
			}
			if n, ok = daysToNanosDur(d, n); !ok {
				return fmt.Errorf("failed to unmarshal duration: to unmarshal into (time.Duration) the data value should be in int64 range")
			}
			*v = n
		}
	}
	return nil
}

func DecDurR(p []byte, v **time.Duration) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(time.Duration)
		}
	case l < 3:
		return errWrongDataLen
	default:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal duration: to unmarshal into (*time.Duration) the months value should be 0")
		}
		if p[1] == 0 {
			n, ok := decNanosDur(p)
			if !ok {
				return errBrokenData
			}
			*v = &n
		} else {
			d, n, ok := decDaysNanosDur(p)
			if !ok {
				return errBrokenData
			}
			if !validDateNanosDur(d, n) {
				return errInvalidSign
			}
			if n, ok = daysToNanosDur(d, n); !ok {
				return fmt.Errorf("failed to unmarshal duration: to unmarshal into (*time.Duration) the data value should be in int64 range")
			}
			*v = &n
		}
	}
	return nil
}

func DecDuration(p []byte, v *Duration) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		*v = Duration{}
	case l < 3:
		return errWrongDataLen
	default:
		var ok bool
		v.Months, v.Days, v.Nanoseconds, ok = decVints(p)
		if !ok {
			return errBrokenData
		}
		if !v.Valid() {
			return errInvalidSign
		}
	}
	return nil
}

func DecDurationR(p []byte, v **Duration) error {
	if v == nil {
		return errNilReference(v)
	}
	switch l := len(p); {
	case l == 0:
		if p == nil {
			*v = nil
		} else {
			*v = new(Duration)
		}
	case l < 3:
		return errWrongDataLen
	default:
		var ok bool
		var val Duration
		val.Months, val.Days, val.Nanoseconds, ok = decVints(p)
		if !ok {
			return errBrokenData
		}
		if !val.Valid() {
			return errInvalidSign
		}
		*v = &val
	}
	return nil
}

func DecReflect(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal duration: can not unmarshal into nil reference (%T)(%[1]v))", v.Interface())
	}

	switch v = v.Elem(); v.Kind() {
	case reflect.Int64:
		return decReflectInt64(p, v)
	case reflect.String:
		return decReflectString(p, v)
	default:
		return fmt.Errorf("failed to unmarshal duration: unsupported value type (%T)(%[1]v), supported types: ~int64, ~string, time.Duration, gocql.Duration", v.Interface())
	}
}

func decReflectInt64(p []byte, v reflect.Value) error {
	switch l := len(p); {
	case l == 0:
		v.SetInt(0)
	case l < 3:
		return errWrongDataLen
	default:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal duration: to unmarshal into (%T) the months value should be 0", v.Interface())
		}
		if p[1] == 0 {
			n, ok := decNanos64(p)
			if !ok {
				return errBrokenData
			}
			v.SetInt(n)
		} else {
			d, n, ok := decDaysNanos64(p)
			if !ok {
				return errBrokenData
			}
			if !validSignDateNanos(d, n) {
				return errInvalidSign
			}
			if n, ok = daysToNanos(d, n); !ok {
				return fmt.Errorf("failed to unmarshal duration: to unmarshal into (%T) the data value should be in int64 range", v.Interface())
			}
			v.SetInt(n)
		}
	}
	return nil
}

func decReflectString(p []byte, v reflect.Value) error {
	switch l := len(p); {
	case l == 0:
		if p == nil {
			v.SetString("")
		} else {
			v.SetString(zeroDuration)
		}
	case l < 3:
		return errWrongDataLen
	default:
		m, d, n, ok := decDuration(p)
		if !ok {
			return errBrokenData
		}
		if !validDuration(m, d, n) {
			return errInvalidSign
		}
		v.SetString(decString(m, d, n))
	}
	return nil
}

func DecReflectR(p []byte, v reflect.Value) error {
	if v.IsNil() {
		return fmt.Errorf("failed to unmarshal duration: can not unmarshal into nil reference (%T)(%[1]v)", v.Interface())
	}

	switch v.Type().Elem().Elem().Kind() {
	case reflect.Int64:
		return decReflectInt64R(p, v)
	case reflect.String:
		return decReflectStringR(p, v)
	default:
		return fmt.Errorf("failed to unmarshal duration: unsupported value type (%T)(%[1]v), supported types: ~int64, ~string, time.Duration, gocql.Duration", v.Interface())
	}
}

func decReflectInt64R(p []byte, v reflect.Value) error {
	switch l := len(p); {
	case l == 0:
		var val reflect.Value
		if p == nil {
			val = reflect.Zero(v.Type().Elem())
		} else {
			val = reflect.New(v.Type().Elem().Elem())
		}
		v.Elem().Set(val)
	case l < 3:
		return errWrongDataLen
	default:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal duration: to unmarshal into (%T) the months value should be 0", v.Interface())
		}
		val := reflect.New(v.Type().Elem().Elem())
		if p[1] == 0 {
			n, ok := decNanos64(p)
			if !ok {
				return errBrokenData
			}
			val.Elem().SetInt(n)
		} else {
			d, n, ok := decDaysNanos64(p)
			if !ok {
				return errBrokenData
			}
			if !validSignDateNanos(d, n) {
				return errInvalidSign
			}
			if n, ok = daysToNanos(d, n); !ok {
				return fmt.Errorf("failed to unmarshal duration: to unmarshal into (%T) the data value should be in int64 range", v.Interface())
			}
			val.Elem().SetInt(n)
		}
		v.Elem().Set(val)
	}
	return nil
}

func decReflectStringR(p []byte, v reflect.Value) error {
	switch l := len(p); {
	case l == 0:
		var val reflect.Value
		if p == nil {
			val = reflect.Zero(v.Type().Elem())
		} else {
			val = reflect.New(v.Type().Elem().Elem())
			val.Elem().SetString(zeroDuration)
		}
		v.Elem().Set(val)
	case l < 3:
		return errWrongDataLen
	default:
		m, d, n, ok := decDuration(p)
		if !ok {
			return errBrokenData
		}
		if !validDuration(m, d, n) {
			return errInvalidSign
		}
		val := reflect.New(v.Type().Elem().Elem())
		val.Elem().SetString(decString(m, d, n))
		v.Elem().Set(val)
	}
	return nil
}

func validSignDateNanos(d int64, n int64) bool {
	if d >= 0 && n >= 0 {
		return true
	}
	if d <= 0 && n <= 0 {
		return true
	}
	return false
}

func daysToNanos(d int64, n int64) (int64, bool) {
	if d > maxDays || d < minDays {
		return 0, false
	}
	d *= nanoDayPos
	if (d > 0 && math.MaxInt64-d < n) || (d < 0 && math.MinInt64-d > n) {
		return 0, false
	}
	return n + d, true
}

func daysToNanosDur(d time.Duration, n time.Duration) (time.Duration, bool) {
	if d > maxDays || d < minDays {
		return 0, false
	}
	d *= nanoDayPos
	if (d > 0 && math.MaxInt64-d < n) || (d < 0 && math.MinInt64-d > n) {
		return 0, false
	}
	return n + d, true
}

func validDateNanosDur(d time.Duration, n time.Duration) bool {
	if d >= 0 && n >= 0 {
		return true
	}
	if d <= 0 && n <= 0 {
		return true
	}
	return false
}

func decDuration(p []byte) (m int32, d int32, n int64, ok bool) {
	if p[0] != 0 {
		m, d, n, ok = decVints(p)
	} else if p[1] != 0 {
		d, n, ok = decDaysNanos(p)
	} else {
		n, ok = decNanos64(p)
	}
	return
}

func decVints(p []byte) (int32, int32, int64, bool) {
	m, read := decVint32(p, 0)
	if read == 0 {
		return 0, 0, 0, false
	}
	d, read := decVint32(p, read)
	if read == 0 {
		return 0, 0, 0, false
	}
	n, read := decVint64(p, read)
	if read == 0 {
		return 0, 0, 0, false
	}
	return decZigZag32(m), decZigZag32(d), decZigZag64(n), true
}

func decDaysNanos(p []byte) (int32, int64, bool) {
	d, read := decVint32(p, 1)
	if read == 0 {
		return 0, 0, false
	}
	n, read := decVint64(p, read)
	if read == 0 {
		return 0, 0, false
	}
	return decZigZag32(d), decZigZag64(n), true
}

func decDaysNanos64(p []byte) (int64, int64, bool) {
	d, read := decVint3264(p, 1)
	if read == 0 {
		return 0, 0, false
	}
	n, read := decVint64(p, read)
	if read == 0 {
		return 0, 0, false
	}
	return decZigZag64(d), decZigZag64(n), true
}

func decNanos64(p []byte) (int64, bool) {
	n, read := decVint64(p, 2)
	if read == 0 {
		return 0, false
	}
	return decZigZag64(n), true
}

func decNanosDur(p []byte) (time.Duration, bool) {
	n, read := decVint64(p, 2)
	if read == 0 {
		return 0, false
	}
	return decZigZagDur(n), true
}

func decDaysNanosDur(p []byte) (time.Duration, time.Duration, bool) {
	d, read := decVint3264(p, 1)
	if read == 0 {
		return 0, 0, false
	}
	n, read := decVint64(p, read)
	if read == 0 {
		return 0, 0, false
	}
	return decZigZagDur(d), decZigZagDur(n), true
}

func decVint64(p []byte, s int) (uint64, int) {
	vintLen := decVintLen(p[s:])
	if vintLen+s != len(p) {
		return 0, 0
	}
	switch vintLen {
	case 9:
		return dec9Vint64(p[s:]), s + 9
	case 8:
		return dec8Vint64(p[s:]), s + 8
	case 7:
		return dec7Vint64(p[s:]), s + 7
	case 6:
		return dec6Vint64(p[s:]), s + 6
	case 5:
		return dec5Vint64(p[s:]), s + 5
	case 4:
		return dec4Vint64(p[s:]), s + 4
	case 3:
		return dec3Vint64(p[s:]), s + 3
	case 2:
		return dec2Vint64(p[s:]), s + 2
	case 1:
		return dec1Vint64(p[s:]), s + 1
	case 0:
		return 0, s + 1
	default:
		return 0, 0
	}
}

func decVint32(p []byte, s int) (uint32, int) {
	vintLen := decVintLen(p[s:])
	if vintLen+s >= len(p) {
		return 0, 0
	}
	switch vintLen {
	case 5:
		if p[s] != vintPrefix4 {
			return 0, 0
		}
		return dec5Vint32(p[s:]), s + 5
	case 4:
		return dec4Vint32(p[s:]), s + 4
	case 3:
		return dec3Vint32(p[s:]), s + 3
	case 2:
		return dec2Vint32(p[s:]), s + 2
	case 1:
		return dec1Vint32(p[s:]), s + 1
	case 0:
		return 0, s + 1
	default:
		return 0, 0
	}
}

func decVint3264(p []byte, s int) (uint64, int) {
	vintLen := decVintLen(p[s:])
	if vintLen+s >= len(p) {
		return 0, 0
	}
	switch vintLen {
	case 5:
		if p[s] != vintPrefix4 {
			return 0, 0
		}
		return dec5Vint64(p[s:]), s + 5
	case 4:
		return dec4Vint64(p[s:]), s + 4
	case 3:
		return dec3Vint64(p[s:]), s + 3
	case 2:
		return dec2Vint64(p[s:]), s + 2
	case 1:
		return dec1Vint64(p[s:]), s + 1
	case 0:
		return 0, s + 1
	default:
		return 0, 0
	}
}

func decVintLen(p []byte) int {
	switch {
	case p[0] == 255:
		return 9
	case p[0]>>1 == 127:
		return 8
	case p[0]>>2 == 63:
		return 7
	case p[0]>>3 == 31:
		return 6
	case p[0]>>4 == 15:
		return 5
	case p[0]>>5 == 7:
		return 4
	case p[0]>>6 == 3:
		return 3
	case p[0]>>7 == 1:
		return 2
	default:
		return 1
	}
}

func decZigZag32(n uint32) int32 {
	return int32((n >> 1) ^ -(n & 1))
}

func decZigZag64(n uint64) int64 {
	return int64((n >> 1) ^ -(n & 1))
}

func decZigZagDur(n uint64) time.Duration {
	return time.Duration((n >> 1) ^ -(n & 1))
}

func dec5Vint32(p []byte) uint32 {
	return uint32(p[1])<<24 | uint32(p[2])<<16 | uint32(p[3])<<8 | uint32(p[4])
}

func dec4Vint32(p []byte) uint32 {
	return uint32(p[0]&^vintPrefix3)<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
}

func dec3Vint32(p []byte) uint32 {
	return uint32(p[0]&^vintPrefix2)<<16 | uint32(p[1])<<8 | uint32(p[2])
}

func dec2Vint32(p []byte) uint32 {
	return uint32(p[0]&^vintPrefix1)<<8 | uint32(p[1])
}

func dec1Vint32(p []byte) uint32 {
	return uint32(p[0])
}

func dec9Vint64(p []byte) uint64 {
	return uint64(p[1])<<56 | uint64(p[2])<<48 | uint64(p[3])<<40 | uint64(p[4])<<32 | uint64(p[5])<<24 | uint64(p[6])<<16 | uint64(p[7])<<8 | uint64(p[8])
}

func dec8Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix7)<<56 | uint64(p[1])<<48 | uint64(p[2])<<40 | uint64(p[3])<<32 | uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7])
}

func dec7Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix6)<<48 | uint64(p[1])<<40 | uint64(p[2])<<32 | uint64(p[3])<<24 | uint64(p[4])<<16 | uint64(p[5])<<8 | uint64(p[6])
}

func dec6Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix5)<<40 | uint64(p[1])<<32 | uint64(p[2])<<24 | uint64(p[3])<<16 | uint64(p[4])<<8 | uint64(p[5])
}

func dec5Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix4)<<32 | uint64(p[1])<<24 | uint64(p[2])<<16 | uint64(p[3])<<8 | uint64(p[4])
}

func dec4Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix3)<<24 | uint64(p[1])<<16 | uint64(p[2])<<8 | uint64(p[3])
}

func dec3Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix2)<<16 | uint64(p[1])<<8 | uint64(p[2])
}

func dec2Vint64(p []byte) uint64 {
	return uint64(p[0]&^vintPrefix1)<<8 | uint64(p[1])
}

func dec1Vint64(p []byte) uint64 {
	return uint64(p[0])
}
