package response

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

// AuthSuccess spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L814
type AuthSuccess struct {
	Token frame.Bytes
}

func ParseAuthSuccess(b *frame.Buffer) *AuthSuccess {
	return &AuthSuccess{
		Token: b.ReadBytes(),
	}
}
