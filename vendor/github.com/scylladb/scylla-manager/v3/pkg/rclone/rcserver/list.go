// Copyright (C) 2017 ScyllaDB

package rcserver

import (
	rcops "github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
)

const defaultListEncoderMaxItems = 200

type listJSONEncoder struct {
	enc      *jsonEncoder
	buf      []*rcops.ListJSONItem
	maxItems int
	started  bool
}

func newListJSONEncoder(wf writerFlusher, maxItems int) *listJSONEncoder {
	return &listJSONEncoder{
		enc:      newJSONEncoder(wf),
		buf:      make([]*rcops.ListJSONItem, 0, maxItems),
		maxItems: maxItems,
	}
}

func (e *listJSONEncoder) Callback(item *rcops.ListJSONItem) error {
	// Aggregate items
	e.buf = append(e.buf, item)
	if len(e.buf) < e.maxItems {
		return nil
	}

	// Write and flush buffer
	if !e.started {
		e.enc.OpenObject()
		e.enc.OpenList("list")
		e.enc.Encode(e.buf[0])

		e.started = true
		e.buf = e.buf[1:]
	}
	for i := range e.buf {
		e.enc.Delim()
		e.enc.Encode(e.buf[i])
	}
	e.enc.Flush()

	e.reset()

	return e.enc.Error()
}

func (e *listJSONEncoder) reset() {
	e.buf = e.buf[:0]
}

func (e *listJSONEncoder) Close() {
	if !e.started {
		return
	}

	// Write remaining list items
	for i := range e.buf {
		e.enc.Delim()
		e.enc.Encode(e.buf[i])
	}

	// Close and flush json
	e.enc.CloseList()
	e.enc.CloseObject()
	e.enc.Flush()
}

func (e *listJSONEncoder) Result(err error) (rc.Params, error) {
	if err != nil {
		return e.errorResult(err)
	}
	return e.result()
}

func (e *listJSONEncoder) errorResult(err error) (rc.Params, error) {
	if !e.started {
		return nil, err
	}
	return nil, errResponseWritten
}

func (e *listJSONEncoder) result() (rc.Params, error) {
	// If not sent business as usual
	if !e.started {
		return rc.Params{"list": e.buf}, nil
	}

	// Notify response sent
	return nil, errResponseWritten
}
