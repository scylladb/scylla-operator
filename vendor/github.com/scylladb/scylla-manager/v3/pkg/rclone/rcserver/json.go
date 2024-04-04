// Copyright (C) 2017 ScyllaDB

package rcserver

import (
	"encoding/json"
	"io"
	"net/http"
)

type writerFlusher interface {
	io.Writer
	http.Flusher
}

type jsonEncoder struct {
	wf  writerFlusher
	enc *json.Encoder
	err error
}

func newJSONEncoder(wf writerFlusher) *jsonEncoder {
	return &jsonEncoder{
		wf:  wf,
		enc: json.NewEncoder(wf),
	}
}

func (e *jsonEncoder) OpenObject() {
	e.writeString(`{`)
}

func (e *jsonEncoder) CloseObject() {
	e.writeString(`}`)
}

func (e *jsonEncoder) OpenList(name string) {
	e.writeString(`"` + name + `":[`)
}

func (e *jsonEncoder) CloseList() {
	e.writeString("]")
}

func (e *jsonEncoder) Field(key string, value interface{}) {
	e.writeString(`"` + key + `":`)
	e.Encode(value)
}

func (e *jsonEncoder) Encode(v interface{}) {
	if e.err != nil {
		return
	}
	e.err = e.enc.Encode(v)
}

func (e *jsonEncoder) Delim() {
	e.writeString(`,`)
}

func (e *jsonEncoder) writeString(s string) {
	if e.err != nil {
		return
	}
	_, e.err = e.wf.Write([]byte(s))
}

func (e *jsonEncoder) Flush() {
	if e.err != nil {
		return
	}
	e.wf.Flush()
}

func (e *jsonEncoder) Error() error {
	return e.err
}
