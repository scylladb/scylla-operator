package request

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

const (
	// Flag for BatchQuery. Values will have its names.
	WithNamesForValues = 0x40
)

var _ frame.Request = (*Batch)(nil)

// Batch spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L414
type Batch struct {
	Type              frame.BatchTypeFlag
	Flags             frame.QueryFlags
	Queries           []BatchQuery
	Consistency       frame.Consistency
	SerialConsistency frame.Consistency
	Timestamp         frame.Long
}

// WriteTo writes Batch body into bytes.Buffer.
func (q *Batch) WriteTo(b *frame.Buffer) {
	b.WriteBatchTypeFlag(q.Type)

	// WriteTo number of queries.
	b.WriteShort(frame.Short(len(q.Queries)))
	for _, k := range q.Queries {
		k.WriteTo(b, q.Flags&WithNamesForValues != 0)
	}
	b.WriteShort(q.Consistency)
	b.WriteQueryFlags(q.Flags)
	if q.Flags&frame.WithSerialConsistency != 0 {
		b.WriteShort(q.SerialConsistency)
	}
	if q.Flags&frame.WithDefaultTimestamp != 0 {
		b.WriteLong(q.Timestamp)
	}
}

func (*Batch) OpCode() frame.OpCode {
	return frame.OpBatch
}

// BatchQuery spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L452
type BatchQuery struct {
	Kind     frame.BatchQueryKind
	Query    string
	Prepared frame.Bytes
	Names    frame.StringList
	Values   []frame.Value
}

func (q *BatchQuery) WriteTo(b *frame.Buffer, name bool) {
	b.WriteByte(q.Kind)
	if q.Kind == 0 {
		b.WriteLongString(q.Query)
	} else {
		b.WriteShortBytes(q.Prepared)
	}

	// WriteTo number of Values.
	b.WriteShort(frame.Short(len(q.Values)))
	for i, v := range q.Values {
		if name {
			b.WriteString(q.Names[i])
		}
		b.WriteValue(v)
	}
}
