package varint

import (
	"fmt"
)

func DecUint8(p []byte, v *uint8) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toUint8(p)
		return nil
	case 2:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint8, the data value should be in the uint8 range")
		}
		*v = dec2toUint8(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint8, the data value should be in the uint8 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toUint8(p)
		*v = &val
		return nil
	case 2:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint8, the data value should be in the uint8 range")
		}
		val := dec2toUint8(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint8, the data value should be in the uint8 range")
	}
	return errBrokenData(p)
}

func DecUint16(p []byte, v *uint16) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toUint16(p)
		return nil
	case 2:
		*v = dec2toUint16(p)
	case 3:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint16, the data value should be in the uint16 range")
		}
		*v = dec3toUint16(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint16, the data value should be in the uint16 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toUint16(p)
		*v = &val
		return nil
	case 2:
		val := dec2toUint16(p)
		*v = &val
	case 3:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint16, the data value should be in the uint16 range")
		}
		val := dec3toUint16(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint16, the data value should be in the uint16 range")
	}
	return errBrokenData(p)
}

func DecUint32(p []byte, v *uint32) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toUint32(p)
		return nil
	case 2:
		*v = dec2toUint32(p)
	case 3:
		*v = dec3toUint32(p)
	case 4:
		*v = dec4toUint32(p)
	case 5:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint32, the data value should be in the uint32 range")
		}
		*v = dec5toUint32(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint32, the data value should be in the uint32 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toUint32(p)
		*v = &val
		return nil
	case 2:
		val := dec2toUint32(p)
		*v = &val
	case 3:
		val := dec3toUint32(p)
		*v = &val
	case 4:
		val := dec4toUint32(p)
		*v = &val
	case 5:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint32, the data value should be in the uint32 range")
		}
		val := dec5toUint32(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint32, the data value should be in the uint32 range")
	}
	return errBrokenData(p)
}

func DecUint64(p []byte, v *uint64) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toUint64(p)
		return nil
	case 2:
		*v = dec2toUint64(p)
	case 3:
		*v = dec3toUint64(p)
	case 4:
		*v = dec4toUint64(p)
	case 5:
		*v = dec5toUint64(p)
	case 6:
		*v = dec6toUint64(p)
	case 7:
		*v = dec7toUint64(p)
	case 8:
		*v = dec8toUint64(p)
	case 9:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint64, the data value should be in the uint64 range")
		}
		*v = dec9toUint64(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint64, the data value should be in the uint64 range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toUint64(p)
		*v = &val
		return nil
	case 2:
		val := dec2toUint64(p)
		*v = &val
	case 3:
		val := dec3toUint64(p)
		*v = &val
	case 4:
		val := dec4toUint64(p)
		*v = &val
	case 5:
		val := dec5toUint64(p)
		*v = &val
	case 6:
		val := dec6toUint64(p)
		*v = &val
	case 7:
		val := dec7toUint64(p)
		*v = &val
	case 8:
		val := dec8toUint64(p)
		*v = &val
	case 9:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint64, the data value should be in the uint64 range")
		}
		val := dec9toUint64(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint64, the data value should be in the uint64 range")
	}
	return errBrokenData(p)
}

func DecUint(p []byte, v *uint) error {
	if v == nil {
		return errNilReference(v)
	}
	switch len(p) {
	case 0:
		*v = 0
		return nil
	case 1:
		*v = dec1toUint(p)
		return nil
	case 2:
		*v = dec2toUint(p)
	case 3:
		*v = dec3toUint(p)
	case 4:
		*v = dec4toUint(p)
	case 5:
		*v = dec5toUint(p)
	case 6:
		*v = dec6toUint(p)
	case 7:
		*v = dec7toUint(p)
	case 8:
		*v = dec8toUint(p)
	case 9:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint, the data value should be in the uint range")
		}
		*v = dec9toUint(p)
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint, the data value should be in the uint range")
	}
	return errBrokenData(p)
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
		return nil
	case 1:
		val := dec1toUint(p)
		*v = &val
		return nil
	case 2:
		val := dec2toUint(p)
		*v = &val
	case 3:
		val := dec3toUint(p)
		*v = &val
	case 4:
		val := dec4toUint(p)
		*v = &val
	case 5:
		val := dec5toUint(p)
		*v = &val
	case 6:
		val := dec6toUint(p)
		*v = &val
	case 7:
		val := dec7toUint(p)
		*v = &val
	case 8:
		val := dec8toUint(p)
		*v = &val
	case 9:
		if p[0] != 0 {
			return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint, the data value should be in the uint range")
		}
		val := dec9toUint(p)
		*v = &val
	default:
		return fmt.Errorf("failed to unmarshal varint: to unmarshal into uint, the data value should be in the uint range")
	}
	return errBrokenData(p)
}

func dec1toUint8(p []byte) uint8 {
	return p[0]
}

func dec1toUint16(p []byte) uint16 {
	return uint16(p[0])
}

func dec1toUint32(p []byte) uint32 {
	return uint32(p[0])
}

func dec1toUint64(p []byte) uint64 {
	return uint64(p[0])
}

func dec1toUint(p []byte) uint {
	return uint(p[0])
}

func dec2toUint8(p []byte) uint8 {
	return p[1]
}

func dec2toUint16(p []byte) uint16 {
	return uint16(p[0])<<8 | uint16(p[1])
}

func dec2toUint32(p []byte) uint32 {
	return uint32(p[0])<<8 | uint32(p[1])
}

func dec2toUint64(p []byte) uint64 {
	return uint64(p[0])<<8 | uint64(p[1])
}

func dec2toUint(p []byte) uint {
	return uint(p[0])<<8 | uint(p[1])
}

func dec3toUint16(p []byte) uint16 {
	return uint16(p[1])<<8 | uint16(p[2])
}

func dec3toUint32(p []byte) uint32 {
	return uint32(p[0])<<16 | uint32(p[1])<<8 | uint32(p[2])
}

func dec3toUint64(p []byte) uint64 {
	return uint64(p[0])<<16 | uint64(p[1])<<8 | uint64(p[2])
}

func dec3toUint(p []byte) uint {
	return uint(p[0])<<16 | uint(p[1])<<8 | uint(p[2])
}

func dec4toUint32(p []byte) uint32 {
	return uint32(p[0])<<24 | uint32(p[1])<<16 | uint32(p[2])<<8 | uint32(p[3])
}

func dec4toUint64(p []byte) uint64 {
	return uint64(p[0])<<24 | uint64(p[1])<<16 | uint64(p[2])<<8 | uint64(p[3])
}

func dec4toUint(p []byte) uint {
	return uint(p[0])<<24 | uint(p[1])<<16 | uint(p[2])<<8 | uint(p[3])
}

func dec5toUint32(p []byte) uint32 {
	return uint32(p[1])<<24 | uint32(p[2])<<16 | uint32(p[3])<<8 | uint32(p[4])
}

func dec5toUint64(p []byte) uint64 {
	return uint64(p[0])<<32 | uint64(p[1])<<24 | uint64(p[2])<<16 | uint64(p[3])<<8 | uint64(p[4])
}

func dec5toUint(p []byte) uint {
	return uint(p[0])<<32 | uint(p[1])<<24 | uint(p[2])<<16 | uint(p[3])<<8 | uint(p[4])
}

func dec6toUint64(p []byte) uint64 {
	return uint64(p[0])<<40 | uint64(p[1])<<32 | uint64(p[2])<<24 | uint64(p[3])<<16 | uint64(p[4])<<8 | uint64(p[5])
}

func dec6toUint(p []byte) uint {
	return uint(p[0])<<40 | uint(p[1])<<32 | uint(p[2])<<24 | uint(p[3])<<16 | uint(p[4])<<8 | uint(p[5])
}

func dec7toUint64(p []byte) uint64 {
	return uint64(p[0])<<48 | uint64(p[1])<<40 | uint64(p[2])<<32 | uint64(p[3])<<24 | uint64(p[4])<<16 | uint64(p[5])<<8 | uint64(p[6])
}

func dec7toUint(p []byte) uint {
	return uint(p[0])<<48 | uint(p[1])<<40 | uint(p[2])<<32 | uint(p[3])<<24 | uint(p[4])<<16 | uint(p[5])<<8 | uint(p[6])
}

func dec8toUint64(p []byte) uint64 {
	return uint64(p[0])<<56 | uint64(p[1])<<48 | uint64(p[2])<<40 | uint64(p[3])<<32 | uint64(p[4])<<24 | uint64(p[5])<<16 | uint64(p[6])<<8 | uint64(p[7])
}

func dec8toUint(p []byte) uint {
	return uint(p[0])<<56 | uint(p[1])<<48 | uint(p[2])<<40 | uint(p[3])<<32 | uint(p[4])<<24 | uint(p[5])<<16 | uint(p[6])<<8 | uint(p[7])
}

func dec9toUint64(p []byte) uint64 {
	return uint64(p[1])<<56 | uint64(p[2])<<48 | uint64(p[3])<<40 | uint64(p[4])<<32 | uint64(p[5])<<24 | uint64(p[6])<<16 | uint64(p[7])<<8 | uint64(p[8])
}

func dec9toUint(p []byte) uint {
	return uint(p[1])<<56 | uint(p[2])<<48 | uint(p[3])<<40 | uint(p[4])<<32 | uint(p[5])<<24 | uint(p[6])<<16 | uint(p[7])<<8 | uint(p[8])
}
