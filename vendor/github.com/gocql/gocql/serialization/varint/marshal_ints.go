package varint

func EncInt8(v int8) ([]byte, error) {
	return encInt8(v), nil
}

func EncInt8R(v *int8) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encInt8(*v), nil
}

func EncInt16(v int16) ([]byte, error) {
	return encInt16(v), nil
}

func EncInt16R(v *int16) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encInt16(*v), nil
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
	return EncInt64Ext(v), nil
}

func EncInt64R(v *int64) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return EncInt64Ext(*v), nil
}

func EncInt(v int) ([]byte, error) {
	return encInt(v), nil
}

func EncIntR(v *int) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return encInt(*v), nil
}

func encInt8(v int8) []byte {
	return []byte{byte(v)}
}

func encInt16(v int16) []byte {
	if v <= maxInt8 && v >= minInt8 {
		return []byte{byte(v)}
	}
	return []byte{byte(v >> 8), byte(v)}
}

func encInt32(v int32) []byte {
	if v <= maxInt8 && v >= minInt8 {
		return []byte{byte(v)}
	}
	if v <= maxInt16 && v >= minInt16 {
		return []byte{byte(v >> 8), byte(v)}
	}
	if v <= maxInt24 && v >= minInt24 {
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	}
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func EncInt64Ext(v int64) []byte {
	if v <= maxInt8 && v >= minInt8 {
		return []byte{byte(v)}
	}
	if v <= maxInt16 && v >= minInt16 {
		return []byte{byte(v >> 8), byte(v)}
	}
	if v <= maxInt24 && v >= minInt24 {
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt32 && v >= minInt32 {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt40 && v >= minInt40 {
		return []byte{byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt48 && v >= minInt48 {
		return []byte{byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt56 && v >= minInt56 {
		return []byte{byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func encInt(v int) []byte {
	if v <= maxInt8 && v >= minInt8 {
		return []byte{byte(v)}
	}
	if v <= maxInt16 && v >= minInt16 {
		return []byte{byte(v >> 8), byte(v)}
	}
	if v <= maxInt24 && v >= minInt24 {
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt32 && v >= minInt32 {
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt40 && v >= minInt40 {
		return []byte{byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt48 && v >= minInt48 {
		return []byte{byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	if v <= maxInt56 && v >= minInt56 {
		return []byte{byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
	return []byte{byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}
