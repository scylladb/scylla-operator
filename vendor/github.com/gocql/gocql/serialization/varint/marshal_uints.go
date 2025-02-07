package varint

func EncUint8(v uint8) ([]byte, error) {
	return encUint8(v), nil
}

func EncUint8R(v *uint8) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encUint8(*v), nil
}

func EncUint16(v uint16) ([]byte, error) {
	return encUint16(v), nil
}

func EncUint16R(v *uint16) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encUint16(*v), nil
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

func EncUint64(v uint64) ([]byte, error) {
	return encUint64(v), nil
}

func EncUint64R(v *uint64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encUint64(*v), nil
}

func EncUint(v uint) ([]byte, error) {
	return encUint(v), nil
}

func EncUintR(v *uint) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encUint(*v), nil
}

func encUint8(v uint8) []byte {
	if v > maxInt8 {
		return []byte{0, v}
	}
	return []byte{v}
}

func encUint16(v uint16) []byte {
	switch {
	case byte(v>>15) != 0:
		return []byte{0, byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}

func encUint32(v uint32) []byte {
	switch {
	case byte(v>>31) != 0:
		return []byte{0, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>23) != 0:
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>15) != 0:
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}

func encUint64(v uint64) []byte {
	switch {
	case byte(v>>63) != 0:
		return []byte{0, byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>55) != 0:
		return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>47) != 0:
		return []byte{byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>39) != 0:
		return []byte{byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>31) != 0:
		return []byte{byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>23) != 0:
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>15) != 0:
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}

func encUint(v uint) []byte {
	switch {
	case byte(v>>63) != 0:
		return []byte{0, byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>55) != 0:
		return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>47) != 0:
		return []byte{byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>39) != 0:
		return []byte{byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>31) != 0:
		return []byte{byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>23) != 0:
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>15) != 0:
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	case byte(v>>7) != 0:
		return []byte{byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v)}
	}
}
