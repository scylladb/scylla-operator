package response

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

// Ready spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L507
type Ready struct{}

func ParseReady(_ *frame.Buffer) *Ready {
	return &Ready{}
}
