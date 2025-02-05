package varint

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

const (
	maxInt8  = 1<<7 - 1
	maxInt16 = 1<<15 - 1
	maxInt24 = 1<<23 - 1
	maxInt32 = 1<<31 - 1
	maxInt40 = 1<<39 - 1
	maxInt48 = 1<<47 - 1
	maxInt56 = 1<<55 - 1
	maxInt64 = 1<<63 - 1

	minInt8  = -1 << 7
	minInt16 = -1 << 15
	minInt24 = -1 << 23
	minInt32 = -1 << 31
	minInt40 = -1 << 39
	minInt48 = -1 << 47
	minInt56 = -1 << 55
)

func EncBigInt(v big.Int) ([]byte, error) {
	return encBigInt(v), nil
}

func EncBigIntR(v *big.Int) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncBigIntRS(v), nil
}

func EncString(v string) ([]byte, error) {
	switch {
	case len(v) == 0:
		return nil, nil
	case len(v) <= 18:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal varint: can not marshal %#v, %s", v, err)
		}
		return EncInt64Ext(n), nil
	case len(v) <= 20:
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return EncInt64Ext(n), nil
		}

		t, ok := new(big.Int).SetString(v, 10)
		if !ok {
			return nil, fmt.Errorf("failed to marshal varint: can not marshal %#v", v)
		}
		return EncBigIntRS(t), nil
	default:
		t, ok := new(big.Int).SetString(v, 10)
		if !ok {
			return nil, fmt.Errorf("failed to marshal varint: can not marshal %#v", v)
		}
		return EncBigIntRS(t), nil
	}
}

func EncStringR(v *string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncString(*v)
}

func encBigInt(v big.Int) []byte {
	switch v.Sign() {
	case 1:
		data := v.Bytes()
		if data[0] > math.MaxInt8 {
			data = append([]byte{0}, data...)
		}
		return data
	case -1:
		data := v.Bytes()
		add := true
		for i := len(data) - 1; i >= 0; i-- {
			if !add {
				data[i] = 255 - data[i]
			} else {
				data[i] = 255 - data[i] + 1
				if data[i] != 0 {
					add = false
				}
			}
		}
		if data[0] < 128 {
			data = append([]byte{255}, data...)
		}
		return data
	default:
		return []byte{0}
	}
}

// EncBigIntRS encode big.Int to []byte.
// This function shared to use in marshal `decimal`.
func EncBigIntRS(v *big.Int) []byte {
	switch v.Sign() {
	case 1:
		data := v.Bytes()
		if data[0] > math.MaxInt8 {
			data = append([]byte{0}, data...)
		}
		return data
	case -1:
		data := v.Bytes()
		add := true
		for i := len(data) - 1; i >= 0; i-- {
			if !add {
				data[i] = 255 - data[i]
			} else {
				data[i] = 255 - data[i] + 1
				if data[i] != 0 {
					add = false
				}
			}
		}
		if data[0] < 128 {
			data = append([]byte{255}, data...)
		}
		return data
	default:
		return []byte{0}
	}
}
