package request

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

var _ frame.Request = (*Prepare)(nil)

// Prepare spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L394
type Prepare struct {
	Query string
}

func (p *Prepare) WriteTo(b *frame.Buffer) {
	b.WriteLongString(p.Query)
}

func (*Prepare) OpCode() frame.OpCode {
	return frame.OpPrepare
}
