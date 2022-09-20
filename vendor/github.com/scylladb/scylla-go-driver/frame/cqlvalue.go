package frame

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"unicode"
	"unicode/utf8"
)

type CqlValue struct {
	Type  *Option
	Value Bytes
}

func (c CqlValue) AsASCII() (string, error) {
	if c.Type.ID != ASCIIID {
		return "", fmt.Errorf("%v is not of ASCII type", c)
	}

	for _, v := range c.Value {
		if v > unicode.MaxASCII {
			return "", fmt.Errorf("%v contains non-ascii characters", c)
		}
	}

	return string(c.Value), nil
}

func (c CqlValue) AsBoolean() (bool, error) {
	if c.Type.ID != BooleanID {
		return false, fmt.Errorf("%v is not of Boolean type", c)
	}

	return c.Value[0] != 0, nil
}

func (c CqlValue) AsBlob() ([]byte, error) {
	if c.Type.ID != BlobID {
		return nil, fmt.Errorf("%v is not of Blob type", c)
	}

	v := make([]byte, len(c.Value))
	copy(v, c.Value)
	return v, nil
}

func (c CqlValue) AsUUID() ([16]byte, error) {
	if c.Type.ID != UUIDID {
		return [16]byte{}, fmt.Errorf("%v is not of UUID type", c)
	}

	if len(c.Value) != 16 {
		return [16]byte{}, fmt.Errorf("expected 16 bytes, got %d", len(c.Value))
	}

	var v [16]byte
	copy(v[:], c.Value)
	return v, nil
}

func (c CqlValue) AsTimeUUID() ([16]byte, error) {
	if c.Type.ID != TimeUUIDID {
		return [16]byte{}, fmt.Errorf("%v is not of TimeUUID type", c)
	}

	if len(c.Value) != 16 {
		return [16]byte{}, fmt.Errorf("expected 16 bytes, got %d", len(c.Value))
	}

	var v [16]byte
	copy(v[:], c.Value)
	if uuidVersion(v) != 1 {
		return [16]byte{}, fmt.Errorf("%v is not a version 1 UUID", v)
	}

	return v, nil
}

func uuidVersion(u [16]byte) int {
	// See section 4.1.3 of RFC 4122.
	return int(u[6] & 0xF0 >> 4)
}

func (c CqlValue) AsInt32() (int32, error) {
	if c.Type.ID != IntID {
		return 0, fmt.Errorf("%v is not of Int type", c)
	}

	if len(c.Value) != 4 {
		return 0, fmt.Errorf("expected 4 bytes, got %d", len(c.Value))
	}

	return int32(c.Value[0])<<24 |
		int32(c.Value[1])<<16 |
		int32(c.Value[2])<<8 |
		int32(c.Value[3]), nil
}

func (c CqlValue) AsInt16() (int16, error) {
	if c.Type.ID != SmallIntID {
		return 0, fmt.Errorf("%v is not of SmallInt type", c)
	}

	if len(c.Value) != 2 {
		return 0, fmt.Errorf("expected 2 bytes, got %d", len(c.Value))
	}

	return int16(c.Value[0])<<8 | int16(c.Value[1]), nil
}

func (c CqlValue) AsInt8() (int8, error) {
	if c.Type.ID != TinyIntID {
		return 0, fmt.Errorf("%v is not of TinyInt type", c)
	}

	if len(c.Value) != 1 {
		return 0, fmt.Errorf("expected 1 byte, got %d", len(c.Value))
	}

	return int8(c.Value[0]), nil
}

func (c CqlValue) AsInt64() (int64, error) {
	if c.Type.ID != BigIntID {
		return 0, fmt.Errorf("%v is not of BigInt type", c)
	}

	if len(c.Value) != 8 {
		return 0, fmt.Errorf("expected 8 bytes, got %d", len(c.Value))
	}

	return int64(c.Value[0])<<56 |
		int64(c.Value[1])<<48 |
		int64(c.Value[2])<<40 |
		int64(c.Value[3])<<32 |
		int64(c.Value[4])<<24 |
		int64(c.Value[5])<<16 |
		int64(c.Value[6])<<8 |
		int64(c.Value[7]), nil
}

func (c CqlValue) AsText() (string, error) {
	if c.Type.ID != VarcharID {
		return "", fmt.Errorf("%v is not of Text/Varchar type", c)
	}

	if !utf8.Valid(c.Value) {
		return "", fmt.Errorf("%v contains non-utf8 characters", c)
	}

	return string(c.Value), nil
}

func (c CqlValue) AsIP() (net.IP, error) {
	if c.Type.ID != InetID {
		return nil, fmt.Errorf("%v is not of Inet type", c)
	}

	if len(c.Value) != 4 && len(c.Value) != 16 {
		return nil, fmt.Errorf("invalid ip length")
	}

	return c.Value, nil
}

func (c CqlValue) AsFloat32() (float32, error) {
	if c.Type.ID != FloatID {
		return 0, fmt.Errorf("%v is not of Float type", c)
	}

	if len(c.Value) != 4 {
		return 0, fmt.Errorf("expected 4 bytes, got %d", len(c.Value))
	}

	return math.Float32frombits(uint32(c.Value[0])<<24 |
		uint32(c.Value[1])<<16 |
		uint32(c.Value[2])<<8 |
		uint32(c.Value[3])), nil
}

func (c CqlValue) AsFloat64() (float64, error) {
	if c.Type.ID != DoubleID {
		return 0, fmt.Errorf("%v is not of Double type", c)
	}

	if len(c.Value) != 8 {
		return 0, fmt.Errorf("expected 8 bytes, got %d", len(c.Value))
	}

	return math.Float64frombits(uint64(c.Value[0])<<56 |
		uint64(c.Value[1])<<48 |
		uint64(c.Value[2])<<40 |
		uint64(c.Value[3])<<32 |
		uint64(c.Value[4])<<24 |
		uint64(c.Value[5])<<16 |
		uint64(c.Value[6])<<8 |
		uint64(c.Value[7])), nil
}

func (c CqlValue) AsStringSlice() ([]string, error) {
	if c.Type.ID != SetID && c.Type.ID != ListID {
		return nil, fmt.Errorf("%v can't be interpreted as a slice", c)
	}

	var elemID OptionID
	if c.Type.ID == SetID {
		elemID = c.Type.Set.Element.ID
	} else {
		elemID = c.Type.List.Element.ID
	}

	if elemID != VarcharID && elemID != ASCIIID {
		return nil, fmt.Errorf("%v can't be interpreted as []string", c)
	}

	raw := c.Value
	if len(raw) < 4 {
		return nil, fmt.Errorf("expected at least 4 bytes, got %d", len(raw))
	}

	res := make([]string, int32(binary.BigEndian.Uint32(raw)))
	raw = raw[4:]

	for i := range res {
		if len(raw) < 4 {
			return nil, fmt.Errorf("expected at least 4 bytes, got %d", len(raw))
		}
		size := binary.BigEndian.Uint32(raw)
		res[i] = string(raw[4 : size+4])
		raw = raw[size+4:]
	}

	return res, nil
}

func (c CqlValue) AsStringMap() (map[string]string, error) {
	if c.Type.ID != MapID {
		return nil, fmt.Errorf("%v is not a map", c)
	}
	if c.Type.Map.Key.ID != VarcharID && c.Type.Map.Key.ID != ASCIIID {
		return nil, fmt.Errorf("map keys can't be interpreted as strings")
	}
	if c.Type.Map.Value.ID != VarcharID && c.Type.Map.Value.ID != ASCIIID {
		return nil, fmt.Errorf("map values can't be interpreted as strings")
	}

	raw := c.Value
	n := int(binary.BigEndian.Uint32(raw))
	raw = raw[4:]

	res := make(map[string]string, n)
	for i := 0; i < n; i++ {
		keyN := binary.BigEndian.Uint32(raw)
		key := string(raw[4 : keyN+4])
		raw = raw[keyN+4:]

		valueN := binary.BigEndian.Uint32(raw)
		value := string(raw[4 : valueN+4])
		raw = raw[valueN+4:]

		res[key] = value
	}
	return res, nil
}

func (c CqlValue) AsDuration() (Duration, error) {
	var err error
	if c.Type.ID != DurationID {
		return Duration{}, fmt.Errorf("%v is not a duration", c)
	}
	raw := c.Value
	months, n, err := decodeVInt(raw)
	if err != nil {
		return Duration{}, err
	}
	if months < math.MinInt32 || months > math.MaxInt32 {
		return Duration{}, fmt.Errorf("months out of range")
	}
	raw = raw[n:]
	days, n, err := decodeVInt(raw)
	if err != nil {
		return Duration{}, err
	}
	if days < math.MinInt32 || days > math.MaxInt32 {
		return Duration{}, fmt.Errorf("days out of range")
	}
	raw = raw[n:]
	nanoseconds, n, err := decodeVInt(raw)
	if err != nil {
		return Duration{}, err
	}
	raw = raw[n:]
	if len(raw) > 0 {
		return Duration{}, fmt.Errorf("extra data after duration value")
	}
	d := Duration{
		Months:      int32(months),
		Days:        int32(days),
		Nanoseconds: nanoseconds,
	}
	err = d.validate()
	if err != nil {
		return Duration{}, err
	}
	return d, nil
}

func CqlFromASCII(s string) (CqlValue, error) {
	for _, v := range s {
		if v > unicode.MaxASCII {
			return CqlValue{}, fmt.Errorf("string contains non-ascii characters")
		}
	}

	return CqlValue{
		Type:  &Option{ID: ASCIIID},
		Value: Bytes(s),
	}, nil
}

func CqlFromBlob(b []byte) CqlValue {
	return CqlValue{
		Type:  &Option{ID: BlobID},
		Value: b,
	}
}

func CqlFromBoolean(b bool) CqlValue {
	v := CqlValue{
		Type: &Option{ID: BooleanID},
	}

	if b {
		v.Value = []byte{1}
	} else {
		v.Value = []byte{0}
	}
	return v
}

func CqlFromInt64(v int64) CqlValue {
	return CqlValue{
		Type: &Option{ID: BigIntID},
		Value: Bytes{byte(v >> 56),
			byte(v >> 48),
			byte(v >> 40),
			byte(v >> 32),
			byte(v >> 24),
			byte(v >> 16),
			byte(v >> 8),
			byte(v)},
	}
}

func CqlFromInt32(v int32) CqlValue {
	return CqlValue{
		Type: &Option{ID: IntID},
		Value: Bytes{byte(v >> 24),
			byte(v >> 16),
			byte(v >> 8),
			byte(v)},
	}
}

func CqlFromInt16(v int16) CqlValue {
	return CqlValue{
		Type: &Option{ID: SmallIntID},
		Value: Bytes{
			byte(v >> 8),
			byte(v)},
	}
}

func CqlFromInt8(v int8) CqlValue {
	return CqlValue{
		Type:  &Option{ID: TinyIntID},
		Value: Bytes{byte(v)},
	}
}

func CqlFromText(s string) (CqlValue, error) {
	if !utf8.ValidString(s) {
		return CqlValue{}, fmt.Errorf("%s contains non-utf8 characters", s)
	}

	return CqlValue{
		Type:  &Option{ID: VarcharID},
		Value: Bytes(s),
	}, nil
}

func CqlFromUUID(b [16]byte) CqlValue {
	c := CqlValue{
		Type:  &Option{ID: UUIDID},
		Value: make(Bytes, 16),
	}
	copy(c.Value, b[:])
	return c
}

func CqlFromTimeUUID(b [16]byte) (CqlValue, error) {
	// See: https://github.com/apache/cassandra/blob/7b91e4cc18e77fa5862864fcc1150fd1eb86a01a/doc/native_protocol_v4.spec#L954
	if uuidVersion(b) != 1 {
		return CqlValue{}, fmt.Errorf("%v is not a version 1 uuid", b)
	}

	c := CqlValue{
		Type:  &Option{ID: TimeUUIDID},
		Value: make(Bytes, 16),
	}
	copy(c.Value, b[:])
	return c, nil
}

func CqlFromIP(ip net.IP) (CqlValue, error) {
	if len(ip) != 4 || len(ip) != 16 {
		return CqlValue{}, fmt.Errorf("invalid ip address")
	}

	c := CqlValue{
		Type:  &Option{ID: InetID},
		Value: make(Bytes, len(ip)),
	}
	copy(c.Value, ip)
	return c, nil
}

func CqlFromFloat32(v float32) CqlValue {
	c := CqlValue{
		Type: &Option{ID: FloatID},
	}
	bits := math.Float32bits(v)
	c.Value = Bytes{
		byte(bits >> 24),
		byte(bits >> 16),
		byte(bits >> 8),
		byte(bits),
	}
	return c
}

func CqlFromFloat64(v float64) CqlValue {
	c := CqlValue{
		Type: &Option{ID: DoubleID},
	}
	bits := math.Float64bits(v)
	c.Value = Bytes{
		byte(bits >> 56),
		byte(bits >> 48),
		byte(bits >> 40),
		byte(bits >> 32),
		byte(bits >> 24),
		byte(bits >> 16),
		byte(bits >> 8),
		byte(bits),
	}
	return c
}

func CqlFromDuration(d Duration) (CqlValue, error) {
	err := d.validate()
	if err != nil {
		return CqlValue{}, err
	}

	c := CqlValue{
		Type:  &Option{ID: DurationID},
		Value: make(Bytes, 0, 27),
	}

	c.Value = appendVInt(c.Value, int64(d.Months))
	c.Value = appendVInt(c.Value, int64(d.Days))
	c.Value = appendVInt(c.Value, d.Nanoseconds)
	return c, nil
}
