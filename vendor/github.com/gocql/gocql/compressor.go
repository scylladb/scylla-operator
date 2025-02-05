package gocql

import (
	"github.com/klauspost/compress/s2"
)

type Compressor interface {
	Name() string
	Encode(data []byte) ([]byte, error)
	Decode(data []byte) ([]byte, error)
}

// SnappyCompressor implements the Compressor interface and can be used to
// compress incoming and outgoing frames. It uses S2 compression algorithm
// that is compatible with snappy and aims for high throughput, which is why
// it features concurrent compression for bigger payloads.
type SnappyCompressor struct{}

func (s SnappyCompressor) Name() string {
	return "snappy"
}

func (s SnappyCompressor) Encode(data []byte) ([]byte, error) {
	return s2.EncodeSnappy(nil, data), nil
}

func (s SnappyCompressor) Decode(data []byte) ([]byte, error) {
	return s2.Decode(nil, data)
}
