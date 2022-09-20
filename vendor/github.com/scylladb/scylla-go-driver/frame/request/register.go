package request

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

var _ frame.Request = (*Register)(nil)

// Register spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L477
type Register struct {
	EventTypes []frame.EventType
}

func (r *Register) WriteTo(b *frame.Buffer) {
	b.WriteEventTypes(r.EventTypes)
}

func (*Register) OpCode() frame.OpCode {
	return frame.OpRegister
}
