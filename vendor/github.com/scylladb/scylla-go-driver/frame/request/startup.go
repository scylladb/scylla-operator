package request

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

var _ frame.Request = (*Startup)(nil)

// Startup spec: https://github.com/apache/cassandra/blob/adcff3f630c0d07d1ba33bf23fcb11a6db1b9af1/doc/native_protocol_v4.spec#L285
type Startup struct {
	Options frame.StartupOptions
}

func (s *Startup) WriteTo(b *frame.Buffer) {
	b.WriteStartupOptions(s.Options)
}

func (*Startup) OpCode() frame.OpCode {
	return frame.OpStartup
}
