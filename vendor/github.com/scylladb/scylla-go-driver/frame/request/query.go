package request

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

var _ frame.Request = (*Query)(nil)

// Query spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L337
type Query struct {
	Query       string
	Consistency frame.Consistency
	Options     frame.QueryOptions
}

func (q *Query) WriteTo(b *frame.Buffer) {
	b.WriteLongString(q.Query)
	b.WriteConsistency(q.Consistency)
	q.Options.SetFlags()
	b.WriteQueryOptions(q.Options)
}

func (*Query) OpCode() frame.OpCode {
	return frame.OpQuery
}
