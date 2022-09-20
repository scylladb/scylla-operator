package response

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

// AuthChallenge spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L802
type AuthChallenge struct {
	Token frame.Bytes
}

func ParseAuthChallenge(b *frame.Buffer) *AuthChallenge {
	return &AuthChallenge{
		Token: b.ReadBytes(),
	}
}
