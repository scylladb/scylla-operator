package response

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

// Authenticate spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L517
type Authenticate struct {
	Name string
}

func ParseAuthenticate(b *frame.Buffer) *Authenticate {
	return &Authenticate{
		Name: b.ReadString(),
	}
}
