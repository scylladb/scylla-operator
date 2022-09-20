package frame

type MsgOptionalFields struct {
	TracingID     UUID
	Warnings      StringList
	CustomPayload BytesMap
}

func ParseMsgOptionalFields(b *Buffer, f HeaderFlags) MsgOptionalFields {
	m := MsgOptionalFields{}
	if f&Tracing != 0 {
		m.TracingID = b.ReadUUID()
	}
	if f&Warning != 0 {
		m.Warnings = b.ReadStringList()
	}
	if f&CustomPayload != 0 {
		m.CustomPayload = b.ReadBytesMap()
	}
	return m
}

func (m MsgOptionalFields) WriteTo(b *Buffer) {
	b.WriteUUID(m.TracingID)
	if m.Warnings != nil {
		b.WriteStringList(m.Warnings)
	}
	if m.CustomPayload != nil {
		b.WriteBytesMap(m.CustomPayload)
	}
}
