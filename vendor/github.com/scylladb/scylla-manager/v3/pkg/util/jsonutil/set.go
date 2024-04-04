// Copyright (C) 2017 ScyllaDB

package jsonutil

import (
	"encoding/json"
)

// Set returns a copy of the message where key=value.
func Set(msg json.RawMessage, key string, value interface{}) json.RawMessage {
	m := map[string]interface{}{}
	if err := json.Unmarshal(msg, &m); err != nil {
		panic(err)
	}
	m[key] = value
	v, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return v
}
