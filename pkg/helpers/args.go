// Copyright (c) 2026 ScyllaDB.

package helpers

import (
	"regexp"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/pointer"
)

var scyllaArgumentsRegexp = regexp.MustCompile(`--([^= ]+)(="[^"]+"|=\S+|\s+"[^"]+"|\s+[^\s-]+|\s+-?\d*\.?\d+[^\s-]+|)`)

func ParseScyllaArguments(scyllaArguments string) map[string]*string {
	output := make(map[string]*string)
	for _, value := range scyllaArgumentsRegexp.FindAllStringSubmatch(scyllaArguments, -1) {
		if value[2] == "" {
			output[value[1]] = nil
		} else {
			output[value[1]] = pointer.Ptr(strings.TrimSpace(strings.TrimPrefix(value[2], "=")))
		}
	}
	return output
}
