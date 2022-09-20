package request

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

var _ frame.Request = (*Execute)(nil)

// Execute spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L403
type Execute struct {
	ID          frame.Bytes
	Consistency frame.Consistency
	Options     frame.QueryOptions
}

func (e *Execute) WriteTo(b *frame.Buffer) {
	b.WriteShortBytes(e.ID)
	b.WriteConsistency(e.Consistency)
	e.Options.SetFlags()
	b.WriteQueryOptions(e.Options)
}

func (*Execute) OpCode() frame.OpCode {
	return frame.OpExecute
}
