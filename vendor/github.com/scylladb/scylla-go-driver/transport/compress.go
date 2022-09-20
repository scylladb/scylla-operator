package transport

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/s2"
	"github.com/pierrec/lz4/v4"
	"github.com/scylladb/scylla-go-driver/frame"
)

var (
	ErrComprCorrupt     = errors.New("compression: corrupt input")
	ErrComprUnknown     = errors.New("compression: unknown algorithm")
	errComprUnspecified = errors.New("compression: unspecified algorithm")
)

type compr struct {
	buf    bytes.Buffer // Holds the data to compress/decompress from source
	tmp    bytes.Buffer // Data should be (de)compressed to tmp before writing it to its destination.
	header []byte

	// codec compresses/decompresses read bytes from buf to tmp.
	codec func(read int) (written int, err error)
	// Used in lz4 encoding.
	c lz4.Compressor
}

func (c *compr) compress(sessionCtx, requestCtx context.Context, dst io.Writer, src *bytes.Buffer) (written int64, err error) {
	c.buf = *src
	buf := c.buf.Bytes()
	read := len(c.buf.Bytes())

	if read <= 9 {
		if err := sessionCtx.Err(); err != nil {
			return 0, fmt.Errorf("request aborted: %w", err)
		}
		if err := requestCtx.Err(); err != nil {
			return 0, &skippedError{err}
		}
		nr, err := dst.Write(buf[:read])
		return int64(nr), err
	}
	_ = copy(c.header, buf[:9])
	c.header[1] |= frame.Compress

	var n int
	if n, err = c.codec(read); err != nil {
		return 0, err
	}

	if err := sessionCtx.Err(); err != nil {
		return 0, fmt.Errorf("request aborted: %w", err)
	}
	if err := requestCtx.Err(); err != nil {
		return 0, &skippedError{err}
	}

	nrh, erh := dst.Write(c.header)
	if erh != nil {
		return int64(nrh), erh
	}

	nr, err := dst.Write(c.tmp.Bytes()[:n])
	return int64(nr + nrh), err
}

func (c *compr) decompress(dst io.Writer, src *io.LimitedReader) (written int64, err error) {
	c.buf.Reset()
	read := int(src.N)

	_, err = io.Copy(&c.buf, src)
	if err != nil {
		return 0, err
	}

	var n int
	if n, err = c.codec(read); err != nil {
		return 0, err
	}

	nr, err := dst.Write(c.tmp.Bytes()[:n])
	return int64(nr), err
}

// newCompr returns a new compressor with respect to whether it should compress or decompress data.
func newCompr(compress bool, algo frame.Compression, bufSize int) (*compr, error) {
	c := new(compr)
	c.buf.Grow(bufSize)
	switch algo {
	case frame.Snappy:
		if compress {
			c.codec = c.encodeSnappy
			c.header = make([]byte, 9)
			c.tmp.Grow(bufSize * bufSize / s2.MaxEncodedLen(bufSize))
		} else {
			c.codec = c.decodeSnappy
			c.tmp.Grow(s2.MaxEncodedLen(bufSize))
		}
	case frame.Lz4:
		if compress {
			c.codec = c.encodeLz4
			c.header = make([]byte, 9)
			c.tmp.Grow(bufSize * bufSize / lz4.CompressBlockBound(bufSize))
		} else {
			c.codec = c.decodeLz4
			c.tmp.Grow(lz4.CompressBlockBound(bufSize))
		}
	default:
		return nil, ErrComprUnknown
	}
	return c, nil
}

func (c *compr) decodeSnappy(read int) (int, error) {
	buf := c.buf.Bytes()
	n, err := s2.DecodedLen(buf)
	if err != nil {
		return 0, err
	}
	c.tmp.Grow(n)
	if _, err := s2.Decode(c.tmp.Bytes(), buf[:read]); err != nil {
		return 0, err
	}
	return n, nil
}

func (c *compr) encodeSnappy(read int) (int, error) {
	c.tmp.Grow(s2.MaxEncodedLen(read - 9))
	enc := s2.EncodeSnappy(c.tmp.Bytes(), c.buf.Bytes()[9:read])
	binary.BigEndian.PutUint32(c.header[5:9], uint32(len(enc)))
	return len(enc), nil
}

func (c *compr) decodeLz4(read int) (int, error) {
	if read < 4 {
		return 0, ErrComprCorrupt
	}

	buf := c.buf.Bytes()
	n := int(binary.BigEndian.Uint32(buf[0:4]))
	c.tmp.Grow(n)
	if _, err := lz4.UncompressBlock(buf[4:read], c.tmp.Bytes()[:n]); err != nil {
		return 0, err
	}
	return n, nil
}

func (c *compr) encodeLz4(read int) (int, error) {
	n := lz4.CompressBlockBound(read - 9)
	c.tmp.Grow(n + 4)
	n, err := c.c.CompressBlock(c.buf.Bytes()[9:read], c.tmp.Bytes()[4:(n+4)])
	if n == 0 || err != nil {
		return 0, ErrComprCorrupt
	}
	binary.BigEndian.PutUint32(c.tmp.Bytes()[:4], uint32(read)-9)
	binary.BigEndian.PutUint32(c.header[5:9], uint32(n)+4)
	return n + 4, nil
}
