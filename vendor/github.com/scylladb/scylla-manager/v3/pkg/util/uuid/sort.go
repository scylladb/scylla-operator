// Copyright (C) 2017 ScyllaDB

package uuid

import "bytes"

// Compare returns an integer comparing two UUIDs.
// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
func Compare(a, b UUID) int {
	return bytes.Compare(a.Bytes(), b.Bytes())
}
