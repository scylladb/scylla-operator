package frame

import (
	"log"
)

func (b *Buffer) Write(v Bytes) {
	_, _ = b.buf.Write(v)
}

func (b *Buffer) WriteByte(v Byte) {
	_ = b.buf.WriteByte(v)
}

func (b *Buffer) WriteShort(v Short) {
	_, _ = b.buf.Write([]byte{
		byte(v >> 8),
		byte(v),
	})
}

func (b *Buffer) WriteStreamID(v StreamID) {
	_, _ = b.buf.Write([]byte{
		byte(v >> 8),
		byte(v),
	})
}

func (b *Buffer) WriteInt(v Int) {
	_, _ = b.buf.Write([]byte{
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	})
}

func (b *Buffer) WriteLong(v Long) {
	_, _ = b.buf.Write([]byte{
		byte(v >> 56),
		byte(v >> 48),
		byte(v >> 40),
		byte(v >> 32),
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	})
}

func (b *Buffer) WriteBatchTypeFlag(v BatchTypeFlag) {
	b.WriteByte(v)
}

func (b *Buffer) WriteHeaderFlags(v HeaderFlags) {
	b.WriteByte(v)
}

func (b *Buffer) WriteQueryFlags(v QueryFlags) {
	b.WriteByte(v)
}

func (b *Buffer) WriteResultFlags(v ResultFlags) {
	b.WriteInt(v)
}

func (b *Buffer) WritePreparedFlags(v PreparedFlags) {
	b.WriteInt(v)
}

func (b *Buffer) WriteOpCode(v OpCode) {
	if Debug {
		if _, ok := allOpCodes[v]; !ok {
			log.Printf("unknown operation code: %v", v)
		}
	}
	b.WriteByte(v)
}

func (b *Buffer) WriteUUID(v UUID) {
	b.Write(v[:])
}

func (b *Buffer) WriteConsistency(v Consistency) {
	if Debug {
		if v > LOCALONE {
			log.Printf("unknown consistency: %v", v)
		}
	}
	b.WriteShort(v)
}

func (b *Buffer) WriteBytes(v Bytes) {
	if v == nil {
		b.WriteInt(-1)
	} else {
		// Writes length of the bytes.
		b.WriteInt(Int(len(v)))
		b.Write(v)
	}
}

func (b *Buffer) WriteShortBytes(v Bytes) {
	// WriteTo length of the bytes.
	b.WriteShort(Short(len(v)))
	b.Write(v)
}

func (b *Buffer) WriteValue(v Value) {
	b.WriteInt(v.N)
	if Debug {
		if v.N < -2 {
			log.Printf("unsupported value length")
		}
	}
	if v.N > 0 {
		_, _ = b.buf.Write(v.Bytes)
	}
}

func (b *Buffer) WriteInet(v Inet) {
	if Debug {
		if l := len(v.IP); l != 4 && l != 16 {
			log.Printf("unknown IP length")
		}
	}
	b.WriteByte(Byte(len(v.IP)))
	b.Write(v.IP)
	b.WriteInt(v.Port)
}

func (b *Buffer) WriteString(s string) {
	// Writes length of the string.
	b.WriteShort(Short(len(s)))
	_, _ = b.buf.WriteString(s)
}

func (b *Buffer) WriteLongString(s string) {
	// Writes length of the long string.
	b.WriteInt(Int(len(s)))
	_, _ = b.buf.WriteString(s)
}

func (b *Buffer) WriteStringList(l StringList) {
	// Writes length of the string list.
	b.WriteShort(Short(len(l)))
	for _, s := range l {
		b.WriteString(s)
	}
}

func (b *Buffer) WriteStringMap(m StringMap) {
	// Writes the number of elements in the map.
	b.WriteShort(Short(len(m)))
	for k, v := range m {
		b.WriteString(k)
		b.WriteString(v)
	}
}

func (b *Buffer) WriteStringMultiMap(m StringMultiMap) {
	// Writes the number of elements in the map.
	b.WriteShort(Short(len(m)))
	for k, v := range m {
		// Writes key.
		b.WriteString(k)
		// Writes value.
		b.WriteStringList(v)
	}
}

func (b *Buffer) WriteBytesMap(m BytesMap) {
	// Writes the number of elements in the map.
	b.WriteShort(Short(len(m)))
	for k, v := range m {
		// Writes key.
		b.WriteString(k)
		// Writes value.
		b.WriteBytes(v)
	}
}

func (b *Buffer) WriteEventTypes(e []EventType) {
	if Debug {
		for _, k := range e {
			if _, ok := allEventTypes[k]; !ok {
				log.Printf("unknown EventType %s", k)
			}
		}
	}
	b.WriteStringList(e)
}

func (b *Buffer) WriteQueryOptions(q QueryOptions) { // nolint:gocritic
	b.WriteQueryFlags(q.Flags)
	// Checks the flags and writes Values correspondent to the ones that are set.
	if Values&q.Flags != 0 {
		// Writes amount of Values.
		b.WriteShort(Short(len(q.Values)))
		for i := range q.Values {
			if WithNamesForValues&q.Flags != 0 {
				b.WriteString(q.Names[i])
			}
			b.WriteValue(q.Values[i])
		}
	}
	if PageSize&q.Flags != 0 {
		b.WriteInt(q.PageSize)
	}
	if WithPagingState&q.Flags != 0 {
		b.WriteBytes(q.PagingState)
	}
	if WithSerialConsistency&q.Flags != 0 {
		b.WriteConsistency(q.SerialConsistency)
	}
	if WithDefaultTimestamp&q.Flags != 0 {
		b.WriteLong(q.Timestamp)
	}
}
