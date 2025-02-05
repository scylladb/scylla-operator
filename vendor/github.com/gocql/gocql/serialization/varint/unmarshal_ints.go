package varint

import (
	"fmt"
)

const (
	negInt16s8 = int16(-1) << 8

	negInt32s8  = int32(-1) << 8
	negInt32s16 = int32(-1) << 16
	negInt32s24 = int32(-1) << 24

	negInt64s8  = int64(-1) << 8
	negInt64s16 = int64(-1) << 16
	negInt64s24 = int64(-1) << 24
	negInt64s32 = int64(-1) << 32
	negInt64s40 = int64(-1) << 40
	negInt64s48 = int64(-1) << 48
	negInt64s56 = int64(-1) << 56

	negIntS8  = int(-1) << 8
	negIntS16 = int(-1) << 16
	negIntS24 = int(-1) << 24
	negIntS32 = int(-1) << 32
	negIntS40 = int(-1) << 40
	negIntS48 = int(-1) << 48
	negIntS56 = int(-1) << 56
)

func DecInt8(p []byte, v *int8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
	case 1:
		*v = dec1toInt8(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int8, the data value should be in the int8 range")
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
		val := dec1toInt8(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int8, the data value should be in the int8 range")
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
		return nil
	case 1:
		*v = dec1toInt16(p)
		return nil
	case 2:
		*v = dec2toInt16(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int16, the data value should be in the int16 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toInt16(p)
		*v = &val
		return nil
	case 2:
		val := dec2toInt16(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int16, the data value should be in the int16 range")
	}
	return errBrokenData(p)
}

func DecInt32(p []byte, v *int32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toInt32(p)
		return nil
	case 2:
		*v = dec2toInt32(p)
	case 3:
		*v = dec3toInt32(p)
	case 4:
		*v = dec4toInt32(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int32, the data value should be in the int32 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toInt32(p)
		*v = &val
		return nil
	case 2:
		val := dec2toInt32(p)
		*v = &val
	case 3:
		val := dec3toInt32(p)
		*v = &val
	case 4:
		val := dec4toInt32(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int32, the data value should be in the int32 range")
	}
	return errBrokenData(p)
}

func DecInt64(p []byte, v *int64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toInt64(p)
		return nil
	case 2:
		*v = dec2toInt64(p)
	case 3:
		*v = dec3toInt64(p)
	case 4:
		*v = dec4toInt64(p)
	case 5:
		*v = dec5toInt64(p)
	case 6:
		*v = dec6toInt64(p)
	case 7:
		*v = dec7toInt64(p)
	case 8:
		*v = dec8toInt64(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int64, the data value should be in the int64 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toInt64(p)
		*v = &val
		return nil
	case 2:
		val := dec2toInt64(p)
		*v = &val
	case 3:
		val := dec3toInt64(p)
		*v = &val
	case 4:
		val := dec4toInt64(p)
		*v = &val
	case 5:
		val := dec5toInt64(p)
		*v = &val
	case 6:
		val := dec6toInt64(p)
		*v = &val
	case 7:
		val := dec7toInt64(p)
		*v = &val
	case 8:
		val := dec8toInt64(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int64, the data value should be in the int64 range")
	}
	return errBrokenData(p)
}

func DecInt(p []byte, v *int) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toInt(p)
		return nil
	case 2:
		*v = dec2toInt(p)
	case 3:
		*v = dec3toInt(p)
	case 4:
		*v = dec4toInt(p)
	case 5:
		*v = dec5toInt(p)
	case 6:
		*v = dec6toInt(p)
	case 7:
		*v = dec7toInt(p)
	case 8:
		*v = dec8toInt(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int, the data value should be in the int range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toInt(p)
		*v = &val
		return nil
	case 2:
		val := dec2toInt(p)
		*v = &val
	case 3:
		val := dec3toInt(p)
		*v = &val
	case 4:
		val := dec4toInt(p)
		*v = &val
	case 5:
		val := dec5toInt(p)
		*v = &val
	case 6:
		val := dec6toInt(p)
		*v = &val
	case 7:
		val := dec7toInt(p)
		*v = &val
	case 8:
		val := dec8toInt(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into int, the data value should be in the int range")
	}
	return errBrokenData(p)
}

func dec1toInt8(p []byte) int8 {
	return int8(p[0])
}

func dec1toInt16(p []byte) int16 {
	if p[0] > 127 {
		return negInt16s8 | int16(p[0])
	}
	return int16(p[0])
}

func dec1toInt32(p []byte) int32 {
	if p[0] > 127 {
		return negInt32s8 | int32(p[0])
	}
	return int32(p[0])
}

func dec1toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s8 | int64(p[0])
	}
	return int64(p[0])
}

func dec1toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS8 | int(p[0])
	}
	return int(p[0])
}

func dec2toInt16(p []byte) int16 {
	return int16(p[0])<<8 | int16(p[1])
}

func dec2toInt32(p []byte) int32 {
	if p[0] > 127 {
		return negInt32s16 | int32(p[0])<<8 | int32(p[1])
	}
	return int32(p[0])<<8 | int32(p[1])
}

func dec2toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s16 | int64(p[0])<<8 | int64(p[1])
	}
	return int64(p[0])<<8 | int64(p[1])
}

func dec2toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS16 | int(p[0])<<8 | int(p[1])
	}
	return int(p[0])<<8 | int(p[1])
}

func dec3toInt32(p []byte) int32 {
	if p[0] > 127 {
		return negInt32s24 | int32(p[0])<<16 | int32(p[1])<<8 | int32(p[2])
	}
	return int32(p[0])<<16 | int32(p[1])<<8 | int32(p[2])
}

func dec3toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s24 | int64(p[0])<<16 | int64(p[1])<<8 | int64(p[2])
	}
	return int64(p[0])<<16 | int64(p[1])<<8 | int64(p[2])
}

func dec3toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS24 | int(p[0])<<16 | int(p[1])<<8 | int(p[2])
	}
	return int(p[0])<<16 | int(p[1])<<8 | int(p[2])
}

func dec4toInt32(p []byte) int32 {
	return int32(p[0])<<24 | int32(p[1])<<16 | int32(p[2])<<8 | int32(p[3])
}

func dec4toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s32 | int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3])
	}
	return int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3])
}

func dec4toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS32 | int(p[0])<<24 | int(p[1])<<16 | int(p[2])<<8 | int(p[3])
	}
	return int(p[0])<<24 | int(p[1])<<16 | int(p[2])<<8 | int(p[3])
}

func dec5toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s40 | int64(p[0])<<32 | int64(p[1])<<24 | int64(p[2])<<16 | int64(p[3])<<8 | int64(p[4])
	}
	return int64(p[0])<<32 | int64(p[1])<<24 | int64(p[2])<<16 | int64(p[3])<<8 | int64(p[4])
}

func dec5toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS40 | int(p[0])<<32 | int(p[1])<<24 | int(p[2])<<16 | int(p[3])<<8 | int(p[4])
	}
	return int(p[0])<<32 | int(p[1])<<24 | int(p[2])<<16 | int(p[3])<<8 | int(p[4])
}

func dec6toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s48 | int64(p[0])<<40 | int64(p[1])<<32 | int64(p[2])<<24 | int64(p[3])<<16 | int64(p[4])<<8 | int64(p[5])
	}
	return int64(p[0])<<40 | int64(p[1])<<32 | int64(p[2])<<24 | int64(p[3])<<16 | int64(p[4])<<8 | int64(p[5])
}

func dec6toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS48 | int(p[0])<<40 | int(p[1])<<32 | int(p[2])<<24 | int(p[3])<<16 | int(p[4])<<8 | int(p[5])
	}
	return int(p[0])<<40 | int(p[1])<<32 | int(p[2])<<24 | int(p[3])<<16 | int(p[4])<<8 | int(p[5])
}

func dec7toInt64(p []byte) int64 {
	if p[0] > 127 {
		return negInt64s56 | int64(p[0])<<48 | int64(p[1])<<40 | int64(p[2])<<32 | int64(p[3])<<24 | int64(p[4])<<16 | int64(p[5])<<8 | int64(p[6])
	}
	return int64(p[0])<<48 | int64(p[1])<<40 | int64(p[2])<<32 | int64(p[3])<<24 | int64(p[4])<<16 | int64(p[5])<<8 | int64(p[6])
}

func dec7toInt(p []byte) int {
	if p[0] > 127 {
		return negIntS56 | int(p[0])<<48 | int(p[1])<<40 | int(p[2])<<32 | int(p[3])<<24 | int(p[4])<<16 | int(p[5])<<8 | int(p[6])
	}
	return int(p[0])<<48 | int(p[1])<<40 | int(p[2])<<32 | int(p[3])<<24 | int(p[4])<<16 | int(p[5])<<8 | int(p[6])
}

func dec8toInt64(p []byte) int64 {
	return int64(p[0])<<56 | int64(p[1])<<48 | int64(p[2])<<40 | int64(p[3])<<32 | int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
}

func dec8toInt(p []byte) int {
	return int(p[0])<<56 | int(p[1])<<48 | int(p[2])<<40 | int(p[3])<<32 | int(p[4])<<24 | int(p[5])<<16 | int(p[6])<<8 | int(p[7])
}
