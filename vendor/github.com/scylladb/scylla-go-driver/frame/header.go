package frame

// StreamID is a type alias for SIGNED Short.
type StreamID = int16

// HeaderSize specifies number of header bytes.
const HeaderSize = 9

// Header spec https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L101.
type Header struct {
	Version  Byte
	Flags    HeaderFlags
	StreamID StreamID
	OpCode   OpCode
	Length   Int
}

func ParseHeader(b *Buffer) Header {
	return Header{
		Version:  b.ReadByte(),
		Flags:    b.ReadHeaderFlags(),
		StreamID: b.ReadStreamID(),
		OpCode:   b.ReadOpCode(),
		Length:   b.ReadInt(),
	}
}

func (h Header) WriteTo(b *Buffer) {
	b.WriteByte(h.Version)
	b.WriteHeaderFlags(h.Flags)
	b.WriteStreamID(h.StreamID)
	b.WriteOpCode(h.OpCode)
	b.WriteInt(h.Length)
}
