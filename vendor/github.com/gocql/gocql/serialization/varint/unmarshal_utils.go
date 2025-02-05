package varint

import (
	"fmt"
	"math/big"
	"strconv"
)

func errBrokenData(p []byte) error {
	if p[0] == 0 && p[1] <= 127 || p[0] == 255 && p[1] > 127 {
		return fmt.Errorf("failed to unmarshal varint: the data is broken")
	}
	return nil
}

func errNilReference(v interface{}) error {
	return fmt.Errorf("failed to unmarshal varint: can not unmarshal into nil reference %#v)", v)
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
			*v = "0"
		}
		return nil
	case 1:
		*v = strconv.FormatInt(dec1toInt64(p), 10)
		return nil
	case 2:
		*v = strconv.FormatInt(dec2toInt64(p), 10)
	case 3:
		*v = strconv.FormatInt(dec3toInt64(p), 10)
	case 4:
		*v = strconv.FormatInt(dec4toInt64(p), 10)
	case 5:
		*v = strconv.FormatInt(dec5toInt64(p), 10)
	case 6:
		*v = strconv.FormatInt(dec6toInt64(p), 10)
	case 7:
		*v = strconv.FormatInt(dec7toInt64(p), 10)
	case 8:
		*v = strconv.FormatInt(dec8toInt64(p), 10)
	default:
		*v = Dec2BigInt(p).String()
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
			val := "0"
			*v = &val
		}
		return nil
	case 1:
		val := strconv.FormatInt(dec1toInt64(p), 10)
		*v = &val
		return nil
	case 2:
		val := strconv.FormatInt(dec2toInt64(p), 10)
		*v = &val
	case 3:
		val := strconv.FormatInt(dec3toInt64(p), 10)
		*v = &val
	case 4:
		val := strconv.FormatInt(dec4toInt64(p), 10)
		*v = &val
	case 5:
		val := strconv.FormatInt(dec5toInt64(p), 10)
		*v = &val
	case 6:
		val := strconv.FormatInt(dec6toInt64(p), 10)
		*v = &val
	case 7:
		val := strconv.FormatInt(dec7toInt64(p), 10)
		*v = &val
	case 8:
		val := strconv.FormatInt(dec8toInt64(p), 10)
		*v = &val
	default:
		val := Dec2BigInt(p).String()
		*v = &val
	}
	return errBrokenData(p)
}

func DecBigInt(p []byte, v *big.Int) error {
	switch len(p) {
	case 0:
		v.SetInt64(0)
		return nil
	case 1:
		v.SetInt64(dec1toInt64(p))
		return nil
	case 2:
		v.SetInt64(dec2toInt64(p))
	case 3:
		v.SetInt64(dec3toInt64(p))
	case 4:
		v.SetInt64(dec4toInt64(p))
	case 5:
		v.SetInt64(dec5toInt64(p))
	case 6:
		v.SetInt64(dec6toInt64(p))
	case 7:
		v.SetInt64(dec7toInt64(p))
	case 8:
		v.SetInt64(dec8toInt64(p))
	default:
		dec2ToBigInt(p, v)
	}
	return errBrokenData(p)
}

func DecBigIntR(p []byte, v **big.Int) error {
	if p != nil {
		*v = big.NewInt(0)
		return DecBigInt(p, *v)
	}
	*v = nil
	return nil
}

// Dec2BigInt decode p to big.Int. Use for cases with len(p)>=2.
// This function shared to use in unmarshal `decimal`.
func Dec2BigInt(p []byte) *big.Int {
	// Positive range processing
	if p[0] <= 127 {
		return new(big.Int).SetBytes(p)
	}
	// negative range processing
	data := make([]byte, len(p))
	copy(data, p)

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

	return new(big.Int).Neg(new(big.Int).SetBytes(data))
}

func dec2ToBigInt(p []byte, v *big.Int) {
	if p[0] <= 127 {
		// Positive range processing
		v.SetBytes(p)
	} else {
		// negative range processing
		data := make([]byte, len(p))
		copy(data, p)

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
		v.Set(new(big.Int).Neg(new(big.Int).SetBytes(data)))
	}
}
