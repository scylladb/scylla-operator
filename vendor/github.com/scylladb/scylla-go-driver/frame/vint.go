package frame

import (
	"fmt"
	"math/bits"
)

// https://github.com/apache/cassandra/blob/afa7dfb5a48ecb56abc2d8bbb1de0fc8f1ca77b9/doc/native_protocol_v5.spec#L393-L409

// decodeVInt decodes [vint] into an int64 value and returns the length on the wire it has read.
func decodeVInt(data []byte) (value int64, length int, err error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("decode vint: not enough bytes")
	}
	additionalBytes := bits.LeadingZeros8(^data[0])
	additional := data[1:]
	if len(additional) < additionalBytes {
		return 0, 0, fmt.Errorf("decode vint: not enough bytes")
	}
	var uvalue uint64
	// copy the first byte, clearing the leading 1 bits
	uvalue = uint64(data[0]) & (uint64(0xff) >> additionalBytes)
	// copy the additional bytes
	for i := 0; i < additionalBytes; i++ {
		uvalue = (uvalue << 8) | (uint64(additional[i]))
	}
	length = 1 + additionalBytes
	value = int64((uvalue >> 1) ^ -(uvalue & 1))
	return
}

// appendVInt encodes value as [vint] and appends it to appendTo.
func appendVInt(appendTo []byte, value int64) []byte {
	if value == 0 {
		return append(appendTo, 0)
	}
	uvalue := uint64((value >> 63) ^ (value << 1))
	var data [9]byte
	i := 8
	for i > 0 && uvalue > 0 {
		data[i] = uint8(uvalue & 0xff)
		i--
		uvalue >>= 8
	}
	lz := bits.LeadingZeros8(data[i+1])
	additionalBytes := 8 - i
	if lz > additionalBytes-1 {
		// enough space to write the prefix into the last-written byte
		additionalBytes--
		i++
	}
	data[i] |= ^(uint8(0xff) >> additionalBytes)
	return append(appendTo, data[i:]...)
}
